package issuerefs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
	"github.com/rvenutolo/repo-toolbelt/internal/vendored"
)

// Check is the check-issue-numbers lint: it validates the section, then
// reports every reference in the tracked text files that no exemption covers,
// and every exemption that covered nothing. Vendored files are not scanned:
// their references belong to the project they were copied from. A tracked
// text file in the working tree that git grep did not read is a setup error.
func Check(ctx context.Context, lc lintkit.Context) ([]lintkit.Finding, error) {
	exemptions, problems, err := ParseSection(lc.Section)
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		return problems, nil
	}
	root, err := os.OpenRoot(lc.Root)
	if err != nil {
		return nil, fmt.Errorf("open repository root: %w", err)
	}
	defer root.Close()
	all, err := trackedFiles(ctx, lc.Root)
	if err != nil {
		return nil, err
	}
	tracked, err := vendored.Filter(ctx, lc.Root, all)
	if err != nil { // coverage-exempt: check-attr fails only if git fails between two calls on the same checkout
		return nil, err
	}
	files, err := textFiles(ctx, lc.Root, lc.Stderr)
	if err != nil {
		return nil, err
	}
	if err := checkSkipped(ctx, lc.Root, root, tracked, files); err != nil {
		return nil, err
	}
	var findings []lintkit.Finding
	for _, file := range files {
		content, ok, err := lintkit.ReadRegular(root, file)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue // coverage-exempt: git grep lists no missing path and none that ends in a symlink or other non-regular file, so this skips only if the tree changes while the lint runs
		}
		findings = append(findings, scanFile(file, content, exemptions)...)
	}
	return append(findings, exemptions.Stale()...), nil
}

// binaryPrefix is how much of a file git's binary test reads: a file with a
// NUL byte in it is binary.
const binaryPrefix = 8000

// trackedFiles lists every file git tracks, in git's order.
func trackedFiles(ctx context.Context, dir string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-files", "-z", "--")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("list tracked files: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return splitNUL(stdout.String()), nil
}

// checkSkipped proves the text listing whole: every tracked file it left
// out must be missing from the working tree, not a regular file, empty, or
// binary to git. git grep passes over some files without a word, such as
// one marked skip-worktree, and a text file it skipped would go unchecked.
// The content test is git's own, a NUL byte in the first 8000 bytes; a file
// that passes it but that git still took for binary, by a gitattributes
// entry such as `-diff`, is one a forced-text grep reads.
func checkSkipped(ctx context.Context, dir string, root *os.Root, tracked, listed []string) error {
	read := make(map[string]bool, len(listed))
	for _, file := range listed {
		read[file] = true
	}
	var suspects []string
	for _, file := range tracked {
		if read[file] {
			continue
		}
		head, ok, err := lintkit.ReadRegularPrefix(root, file, binaryPrefix)
		if err != nil {
			return err
		}
		if !ok || len(head) == 0 || bytes.IndexByte(head, 0) >= 0 {
			continue
		}
		suspects = append(suspects, file)
	}
	if len(suspects) == 0 {
		return nil
	}
	// Warnings went to diagnostics on the first pass; this one repeats them.
	// A failed batch lists nothing, so each of its suspects counts as unread
	// too. git grep takes no pathspecs on stdin, so the suspects go on the
	// command line in batches.
	read = make(map[string]bool, len(suspects))
	var grepErr error
	for _, batch := range pathBatches(suspects) {
		forced, err := grepFiles(ctx, dir, io.Discard, "-a", true, batch)
		grepErr = errors.Join(grepErr, err)
		for _, file := range forced {
			read[file] = true
		}
	}
	var skipped []string
	for _, file := range suspects {
		if !read[file] {
			skipped = append(skipped, strconv.Quote(file))
		}
	}
	if len(skipped) == 0 {
		return nil
	}
	return errors.Join(grepErr, fmt.Errorf("git grep did not read tracked text files: %s", strings.Join(skipped, ", ")))
}

// pathspecBatchBytes bounds the bytes of pathspecs one git command takes, far
// under Linux's limit on a command's arguments, which is a quarter of the
// stack limit and commonly 2 MiB.
const pathspecBatchBytes = 256 << 10

// pathBatches splits paths, in order, into runs of at most pathspecBatchBytes
// bytes each, counting a terminator per path.
func pathBatches(paths []string) [][]string {
	var batches [][]string
	start, size := 0, 0
	for i, file := range paths {
		if size > 0 && size+len(file)+1 > pathspecBatchBytes {
			batches = append(batches, paths[start:i])
			start, size = i, 0
		}
		size += len(file) + 1
	}
	return append(batches, paths[start:])
}

// textFiles lists the tracked files that are not vendored and that git takes
// for text, by git's own test, in git's order. Vendored files are left out by
// pathspec, so git never opens them. A tracked file missing from the working
// tree is left out; one git cannot read is an error. Any other stderr from
// git, such as a warning about the repo's .gitattributes, goes to diagnostics
// unchanged.
func textFiles(ctx context.Context, dir string, diagnostics io.Writer) ([]string, error) {
	return grepFiles(ctx, dir, diagnostics, "-I", false, append([]string{"."}, vendored.ExcludePathspecs()...))
}

// grepFiles lists the files among pathspecs that git grep reads and finds
// content in, with binary handled as the flag says: -I skips binary files, -a
// reads them as text. With literal set, every pathspec is a tracked name,
// never a pattern; without it, pathspec magic applies.
func grepFiles(ctx context.Context, dir string, diagnostics io.Writer, binary string, literal bool, pathspecs []string) ([]string, error) {
	// --no-recurse-submodules: a repo's submodule.recurse would otherwise
	// widen the scan to files the repo does not track.
	// An empty pattern matches every line, so this lists every file with any
	// content; an empty file holds no reference to report.
	var args []string
	if literal {
		args = append(args, "--literal-pathspecs")
	}
	args = append(args, "grep", "--no-recurse-submodules", binary, "-z", "--files-with-matches", "--color=never", "-e", "", "--")
	args = append(args, pathspecs...)
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // fixed program; the pathspecs follow -- and are literal
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		// git grep skips a file it cannot read, says so on stderr, and still
		// exits 0 when another file matched; skipping it would pass the file
		// unchecked. A warning leaves the listing whole.
		if failures := passWarnings(stderr.String(), diagnostics); failures != "" {
			return nil, fmt.Errorf("list tracked text files: %s", failures)
		}
	case errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && stderr.Len() == 0:
		// git grep exits 1 when nothing matched: no file it read has content.
		return nil, nil
	default:
		return nil, fmt.Errorf("list tracked text files: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return splitNUL(stdout.String()), nil
}

// splitNUL splits git's -z output into names.
func splitNUL(out string) []string {
	if out == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(out, "\x00"), "\x00")
}

// passWarnings writes every line of git's stderr that is not an error to w
// and returns the error lines, `error:` or `fatal:`, joined by newlines. A
// nil w discards the warnings.
func passWarnings(stderr string, w io.Writer) string {
	if w == nil {
		w = io.Discard
	}
	var failures []string
	for line := range strings.Lines(stderr) {
		if strings.HasPrefix(line, "error:") || strings.HasPrefix(line, "fatal:") {
			failures = append(failures, strings.TrimSuffix(line, "\n"))
			continue
		}
		fmt.Fprint(w, line)
	}
	return strings.Join(failures, "\n")
}

func scanFile(file string, content []byte, exemptions *Exemptions) []lintkit.Finding {
	refs := Scan(file, content)
	if len(refs) == 0 {
		return nil
	}
	lines := strings.Split(string(content), "\n")
	var findings []lintkit.Finding
	for _, ref := range refs {
		if exemptions.Exempts(file, strings.TrimSuffix(lines[ref.Line-1], "\r")) {
			continue
		}
		findings = append(findings, lintkit.Finding{
			Path:    file,
			Line:    ref.Line,
			Message: "issue or PR reference " + ref.Text + "; state the reason instead",
		})
	}
	return findings
}
