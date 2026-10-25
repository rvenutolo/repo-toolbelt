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
	// running.
	cached, err := git(ctx, root, "diff", "--cached", "--name-only", "-z", "--no-renames", "--no-ext-diff",
		"--diff-filter=d", "--ignore-submodules")
	if err != nil {
		return nil, nil, err
	}
	staged = splitNUL(cached)
	if len(staged) == 0 {
		return nil, nil, nil
	}
	unstaged, err := git(ctx, root, "diff", "--name-only", "-z", "--no-renames", "--no-ext-diff", "--ignore-submodules")
	if err != nil { // coverage-exempt: the cached diff just ran in the same checkout
		return nil, nil, err
	}
	changed := splitNUL(unstaged)
	for _, file := range staged {
		if slices.Contains(changed, file) {
			partly = append(partly, file)
		}
	}
	return staged, partly, nil
}
