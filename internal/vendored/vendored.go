// Package vendored tells which tracked files are vendored: copies of another
// project's code that a repo carries but does not own, which no formatter
// formats and no lint judges.
package vendored

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// InVendorDir reports whether a directory of the repo-relative path file is
// named vendor, the convention Go modules, Composer and Bundler share. The
// file's own name does not count.
func InVendorDir(file string) bool {
	segments := strings.Split(file, "/")
	return slices.Contains(segments[:len(segments)-1], "vendor")
}

// ByAttribute returns the files among files, repo-relative to the git
// checkout at root, that git reports the linguist-vendored attribute set or
// true for, in one git call. A git failure is an error.
func ByAttribute(ctx context.Context, root string, files []string) (map[string]bool, error) {
	vendored := make(map[string]bool)
	if len(files) == 0 {
		return vendored, nil
	}
	cmd := exec.CommandContext(ctx, "git", "check-attr", "-z", "--stdin", "linguist-vendored")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANGUAGE=")
	cmd.Stdin = strings.NewReader(strings.Join(files, "\x00") + "\x00")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if first, _, _ := strings.Cut(strings.TrimSpace(stderr.String()), "\n"); first != "" {
			return nil, fmt.Errorf("git check-attr: %w: %s", err, first)
		}
		return nil, fmt.Errorf("git check-attr: %w", err)
	}
	// Each record is path, attribute, value, all NUL-terminated.
	fields := strings.Split(strings.TrimSuffix(stdout.String(), "\x00"), "\x00")
	if len(fields)%3 != 0 { // coverage-exempt: check-attr -z always prints whole records, so this guards against a git change
		return nil, fmt.Errorf("git check-attr: unexpected output of %d fields", len(fields))
	}
	for record := range slices.Chunk(fields, 3) {
		if value := record[2]; value == "set" || value == "true" {
			vendored[record[0]] = true
		}
	}
	return vendored, nil
}

// Filter returns files without the vendored ones, in order: those under a
// vendor directory and those git marks linguist-vendored. A git failure is an
// error.
func Filter(ctx context.Context, root string, files []string) ([]string, error) {
	var kept []string
	for _, file := range files {
		if !InVendorDir(file) {
			kept = append(kept, file)
		}
	}
	marked, err := ByAttribute(ctx, root, kept)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(kept, func(file string) bool { return marked[file] }), nil
}
