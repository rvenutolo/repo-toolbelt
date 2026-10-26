// Package filesel selects the files each formatter owns from a git checkout,
// and applies a formatter's exclude globs to its list.
package filesel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"
	"syscall"

	"github.com/rvenutolo/repo-toolbelt/internal/manifest"
	"github.com/rvenutolo/repo-toolbelt/internal/pathglob"
	"github.com/rvenutolo/repo-toolbelt/internal/vendored"
)

// shebangPrefix is how much of an extensionless file is read to find its
// shebang. A first line that does not end within it is not recognised.
const shebangPrefix = 256

// Select returns, for each formatter by name, the sorted repo-relative paths
// of the files it owns: the tracked regular files in the working tree under
// root that its include globs or shebangs claim, minus vendored files. Every
// formatter is a key, with an empty list when it owns nothing. A symlink, and
// a file under a directory that is now a symlink, is skipped rather than
// followed, so nothing outside root is read and no file is reached by two
// paths. A git failure, a candidate file that cannot be stat-ed or read, and
// a tracked file outside a vendor directory whose base name the include globs
// of more than one formatter match, are errors.
func Select(ctx context.Context, root string, formatters []manifest.Formatter) (map[string][]string, error) {
	selected := make(map[string][]string, len(formatters))
	for _, f := range formatters {
		selected[f.Name] = []string{}
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", root, err)
	}
	defer dir.Close()

	listing, err := git(ctx, root, "ls-files", "-z")
	if err != nil {
		return nil, err
	}
	owned := make(map[string][]string) // file -> names of the formatters that own it
	var candidates []string
	// git lists a path once per index stage, so an unmerged path repeats; the
	// listing is sorted, which puts the repeats side by side.
	for _, file := range slices.Compact(splitNUL(listing)) {
		if vendored.InVendorDir(file) {
			continue
		}
		owners, err := ownersOf(dir, file, formatters)
		if err != nil {
			return nil, err
		}
		if len(owners) > 0 {
			owned[file] = owners
			candidates = append(candidates, file)
		}
	}

	marked, err := vendored.ByAttribute(ctx, root, candidates)
	if err != nil { // coverage-exempt: check-attr fails only if git fails between two calls on the same checkout
		return nil, err
	}
	for _, file := range candidates {
		if marked[file] {
			continue
		}
		for _, name := range owned[file] {
			selected[name] = append(selected[name], file)
		}
	}
	for name := range selected {
		slices.Sort(selected[name])
	}
	return selected, nil
}

// Exclude returns the files that no glob matches, in their order. The globs
// are pathglob patterns and must be valid.
func Exclude(files, globs []string) []string {
	kept := make([]string, 0, len(files))
	for _, file := range files {
		if !slices.ContainsFunc(globs, func(glob string) bool { return pathglob.Match(glob, file) }) {
			kept = append(kept, file)
		}
	}
	return kept
}

// ownersOf names the formatter that owns the tracked file: none when it is
// not a regular file on disk. An include match claims the file; only a file
// that is extensionless and that no formatter's include claims is looked at
// for a shebang. A file has at most one owner: a base name that the include
// globs of more than one formatter match is an error naming them, since
// distinct globs the manifest accepts can still overlap, and the manifest
// lets no two formatters list one shebang. That error comes before the file
// is stat-ed, and Select checks linguist-vendored only after, so it holds
// whatever the file's state on disk and whether or not it is vendored by
// attribute.
func ownersOf(dir *os.Root, file string, formatters []manifest.Formatter) ([]string, error) {
	var owners []string
	for _, f := range formatters {
		if ClaimedByName(f, file) {
			owners = append(owners, f.Name)
		}
	}
	if len(owners) > 1 {
		slices.Sort(owners)
		return nil, fmt.Errorf("%s: matched by the include globs of more than one formatter: %s", file, strings.Join(owners, ", "))
	}
	extensionless := isExtensionless(path.Base(file))
	if len(owners) == 0 && (!extensionless || !hasShebangFormatter(formatters)) {
		return nil, nil
	}

	regular, err := IsRegular(dir, file)
	if err != nil || !regular {
		return nil, err
	}
	if len(owners) > 0 {
		return owners, nil
	}
	interpreter, err := shebangInterpreter(dir, file)
	if err != nil || interpreter == "" {
		return nil, err
	}
	for _, f := range formatters {
		if slices.Contains(f.Shebangs, interpreter) {
			owners = append(owners, f.Name)
		}
	}
	return owners, nil
}

func hasShebangFormatter(formatters []manifest.Formatter) bool {
	return slices.ContainsFunc(formatters, func(f manifest.Formatter) bool { return len(f.Shebangs) > 0 })
}

// ClaimedByName reports whether one of f's include globs matches the base
// name of the repo-relative path file. A shebang claims only a file that can
// be read, so it never counts here.
func ClaimedByName(f manifest.Formatter, file string) bool {
	return slices.ContainsFunc(f.Include, func(glob string) bool { return matchesBase(glob, file) })
}

// matchesBase reports whether the include glob matches the base name of file.
// An invalid glob matches nothing; the manifest rejects them.
func matchesBase(glob, file string) bool {
	ok, _ := path.Match(glob, path.Base(file)) //nolint:errcheck // an invalid glob matches nothing, and the manifest validates its globs
	return ok
}

// isExtensionless reports whether the base name holds no '.' after its first
// character, so ".envrc" and "run-tests" are extensionless and "a.sh" and
// ".envrc.local" are not.
func isExtensionless(base string) bool {
	return !strings.Contains(base[1:], ".")
}

// IsRegular reports whether the repo-relative path file is a regular file in
// the working tree dir, reached without passing through a symlink. A file that
// is gone, one whose parent is no longer a directory, and one under a
// directory that is now a symlink are not; any other failure to stat it is an
// error.
func IsRegular(dir *os.Root, file string) (bool, error) {
	// os.Root follows a symlink in a parent component that stays inside the
	// root and fails on one that leaves it, so each parent is checked first,
	// outermost first.
	segments := strings.Split(file, "/")
	for i := 1; i < len(segments); i++ {
		info, ok, err := lstat(dir, strings.Join(segments[:i], "/"))
		if err != nil || !ok || !info.IsDir() {
			return false, err
		}
	}
	info, ok, err := lstat(dir, file)
	return ok && info.Mode().IsRegular(), err
}

// lstat is dir.Lstat reporting false, with no error, for a name that is gone
// or whose parent is not a directory.
func lstat(dir *os.Root, name string) (fs.FileInfo, bool, error) {
	info, err := dir.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("stat %s: %w", name, err)
	}
	return info, true, nil
}

// shebangInterpreter returns the base name of the interpreter that the first
// line of file names, or "" when it has no shebang or its first line is not
// whole within the prefix read.
func shebangInterpreter(dir *os.Root, file string) (string, error) {
	f, err := dir.Open(file)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", file, err)
	}
	defer f.Close()
	head, err := io.ReadAll(io.LimitReader(f, shebangPrefix))
	if err != nil { // coverage-exempt: the file is a regular file that opened, so a read fails only on an I/O fault
		return "", fmt.Errorf("read %s: %w", file, err)
	}
	rest, ok := bytes.CutPrefix(head, []byte("#!"))
	if !ok {
		return "", nil
	}
	line, _, terminated := bytes.Cut(rest, []byte("\n"))
	if !terminated && len(head) == shebangPrefix {
		return "", nil
	}
	return interpreterOf(strings.Fields(string(line))), nil
}

// interpreterOf returns the base name of the interpreter a shebang's words
// name: the first word, or for env the first word that is neither an option
// (-S included, its argument string being words like the rest) nor a
// NAME=value assignment.
func interpreterOf(words []string) string {
	if len(words) == 0 {
		return ""
	}
	interpreter := path.Base(words[0])
	if interpreter != "env" {
		return interpreter
	}
	for _, word := range words[1:] {
		if strings.HasPrefix(word, "-") || strings.Index(word, "=") > 0 {
			continue
		}
		return path.Base(word)
	}
	return ""
}

// git runs git in dir and returns its stdout. Git runs in the C locale so its
// messages are stable. The error of a failed git carries the first line of
// its stderr.
func git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // fixed program; the arguments are this package's own
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANGUAGE=")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		first, _, _ := strings.Cut(strings.TrimSpace(stderr.String()), "\n")
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, first)
	}
	return stdout.Bytes(), nil
}

// splitNUL splits NUL-terminated output into its entries.
func splitNUL(out []byte) []string {
	if len(out) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
}
