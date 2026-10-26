package filesel_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/filesel"
)

func assertStaged(t *testing.T, repo string, wantStaged, wantPartly []string) {
	t.Helper()
	staged, partly, err := filesel.Staged(t.Context(), repo)
	if err != nil {
		t.Fatalf("Staged error = %v", err)
	}
	if diff := cmp.Diff(wantStaged, staged); diff != "" {
		t.Errorf("staged mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantPartly, partly); diff != "" {
		t.Errorf("partly staged mismatch (-want +got):\n%s", diff)
	}
}

func TestStaged(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, map[string]string{
		"whole.sh": "a\n", "partly.sh": "a\n", "unstaged.sh": "a\n", "deleted.sh": "a\n", "same.sh": "a\n",
		"dir/old name.sh": "a\n",
	})
	writeFile(t, filepath.Join(repo, "whole.sh"), "b\n")
	writeFile(t, filepath.Join(repo, "partly.sh"), "b\n")
	writeFile(t, filepath.Join(repo, "added.sh"), "b\n")
	gitIn(t, repo, "add", "whole.sh", "partly.sh", "added.sh")
	gitIn(t, repo, "rm", "--quiet", "deleted.sh")
	gitIn(t, repo, "mv", "dir/old name.sh", "dir/new name.sh")
	writeFile(t, filepath.Join(repo, "partly.sh"), "c\n")
	writeFile(t, filepath.Join(repo, "unstaged.sh"), "b\n")
	// A touched but unchanged file is not a change.
	writeFile(t, filepath.Join(repo, "same.sh"), "a\n")

	assertStaged(t, repo, []string{"added.sh", "dir/new name.sh", "partly.sh", "whole.sh"}, []string{"partly.sh"})
}

func TestStaged_edgeCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		prepare    func(t *testing.T, repo string)
		wantStaged []string
		wantPartly []string
	}{
		{
			name: "an intent-to-add file is not staged",
			prepare: func(t *testing.T, repo string) {
				t.Helper()
				writeFile(t, filepath.Join(repo, "new.sh"), "a\n")
				gitIn(t, repo, "add", "--intent-to-add", "new.sh")
			},
		},
		{
			name: "a mode change alone is staged",
			prepare: func(t *testing.T, repo string) {
				t.Helper()
				gitIn(t, repo, "update-index", "--chmod=+x", "a.sh")
				if err := os.Chmod(filepath.Join(repo, "a.sh"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			wantStaged: []string{"a.sh"},
		},
		{
			name: "a staged file reverted in the working tree is partly staged",
			prepare: func(t *testing.T, repo string) {
				t.Helper()
				writeFile(t, filepath.Join(repo, "a.sh"), "b\n")
				gitIn(t, repo, "add", "a.sh")
				writeFile(t, filepath.Join(repo, "a.sh"), "a\n")
			},
			wantStaged: []string{"a.sh"},
			wantPartly: []string{"a.sh"},
		},
		{
			name: "a diff.orderFile in the repo's config neither reorders the result nor fails",
			prepare: func(t *testing.T, repo string) {
				t.Helper()
				// The container never has the host path a repo's config may name.
				gitIn(t, repo, "config", "diff.orderFile", filepath.Join(t.TempDir(), "missing"))
				writeFile(t, filepath.Join(repo, "a.sh"), "b\n")
				writeFile(t, filepath.Join(repo, "z.sh"), "b\n")
				gitIn(t, repo, "add", "a.sh", "z.sh")
				writeFile(t, filepath.Join(repo, "a.sh"), "c\n")
				writeFile(t, filepath.Join(repo, "z.sh"), "c\n")
			},
			wantStaged: []string{"a.sh", "z.sh"},
			wantPartly: []string{"a.sh", "z.sh"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := newRepo(t, map[string]string{"a.sh": "a\n", "z.sh": "a\n"})
			tc.prepare(t, repo)
			assertStaged(t, repo, tc.wantStaged, tc.wantPartly)
		})
	}
}

func TestStaged_aStagedFileDeletedFromTheWorkingTreeIsPartlyStaged(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, map[string]string{"a.sh": "a\n"})
	writeFile(t, filepath.Join(repo, "a.sh"), "b\n")
	gitIn(t, repo, "add", "a.sh")
	if err := os.Remove(filepath.Join(repo, "a.sh")); err != nil {
		t.Fatal(err)
	}
	assertStaged(t, repo, []string{"a.sh"}, []string{"a.sh"})
}

func TestStaged_beforeTheFirstCommitEveryIndexedFileIsStaged(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	gitIn(t, repo, "init", "--quiet", "--initial-branch=main")
	writeFile(t, filepath.Join(repo, "a.sh"), "a\n")
	writeFile(t, filepath.Join(repo, "b.sh"), "a\n")
	gitIn(t, repo, "add", "a.sh", "b.sh")
	writeFile(t, filepath.Join(repo, "b.sh"), "b\n")
	assertStaged(t, repo, []string{"a.sh", "b.sh"}, []string{"b.sh"})
}

func TestStaged_nothingStaged(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, map[string]string{"a.sh": "a\n"})
	writeFile(t, filepath.Join(repo, "a.sh"), "b\n")
	assertStaged(t, repo, nil, nil)
}

func TestStaged_readsTheIndexGitIndexFileNames(t *testing.T) {
	// Not parallel: GIT_INDEX_FILE is process environment, as git sets it for
	// a pre-commit hook under `git commit --all`.
	repo := newRepo(t, map[string]string{"a.sh": "a\n", "b.sh": "a\n"})
	writeFile(t, filepath.Join(repo, "a.sh"), "b\n")
	writeFile(t, filepath.Join(repo, "b.sh"), "b\n")
	index := filepath.Join(t.TempDir(), "index")
	t.Setenv("GIT_INDEX_FILE", index)
	gitIn(t, repo, "read-tree", "HEAD")
	gitIn(t, repo, "add", "a.sh")
	assertStaged(t, repo, []string{"a.sh"}, nil)
}

func TestStaged_aFailingCleanFilterIsAnError(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, map[string]string{".gitattributes": "*.bin filter=f\n", "y.bin": "a\n", "a.sh": "a\n"})
	writeFile(t, filepath.Join(repo, "a.sh"), "b\n")
	gitIn(t, repo, "add", "a.sh")
	// Set only now, so git add never runs it: the shape `git lfs install`
	// leaves where git-lfs is missing. Only the working-tree diff cleans
	// y.bin once its stat data changes.
	gitIn(t, repo, "config", "filter.f.clean", "false")
	gitIn(t, repo, "config", "filter.f.required", "true")
	writeFile(t, filepath.Join(repo, "y.bin"), "a\n")
	if _, _, err := filesel.Staged(t.Context(), repo); err == nil {
		t.Error("Staged with a failing clean filter error = nil, want the git failure")
	}
}

func TestStaged_gitFailureIsAnError(t *testing.T) {
	t.Parallel()
	if _, _, err := filesel.Staged(t.Context(), t.TempDir()); err == nil {
		t.Error("Staged outside a git repository error = nil, want the git failure")
	}
}
