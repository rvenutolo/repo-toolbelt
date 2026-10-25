package filesel

import (
	"context"
	"slices"
)

// Staged returns the sorted repo-relative paths of the files the index holds
// a change to against HEAD, every indexed file before the first commit, with
// deletions left out; and, among them, those whose working-tree copy also
// differs from the index, a deleted copy included. The index is the one git
// names in GIT_INDEX_FILE when it is set, as git sets it for a pre-commit hook
// under `git commit --all` or `git commit <path>`. A git failure is an error.
func Staged(ctx context.Context, root string) (staged, partly []string, err error) {
	// --no-renames reports a rename as an addition and a deletion, so the new
	// path is kept; --no-ext-diff and --name-only keep any diff driver from
	// running; -O/dev/null overrides a diff.orderFile in the repo's config,
	// which may name a file the container lacks, and git then refuses to diff.
	cached, err := git(ctx, root, "diff", "--cached", "--name-only", "-z", "--no-renames", "--no-ext-diff",
		"-O/dev/null", "--diff-filter=d", "--ignore-submodules")
	if err != nil {
		return nil, nil, err
	}
	staged = slices.Compact(slices.Sorted(slices.Values(splitNUL(cached))))
	if len(staged) == 0 {
		return nil, nil, nil
	}
	unstaged, err := git(ctx, root, "diff", "--name-only", "-z", "--no-renames", "--no-ext-diff", "-O/dev/null",
		"--ignore-submodules")
	// The working-tree diff runs a file's clean filter, which the cached diff
	// never does, so it can fail alone.
	if err != nil {
		return nil, nil, err
	}
	changed := make(map[string]bool)
	for _, file := range splitNUL(unstaged) {
		changed[file] = true
	}
	for _, file := range staged {
		if changed[file] {
			partly = append(partly, file)
		}
	}
	return staged, partly, nil
}
