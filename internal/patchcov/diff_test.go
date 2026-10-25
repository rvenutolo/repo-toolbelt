package patchcov_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/patchcov"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newRepo creates a repository and returns it with the commit of its first
// state.
func newRepo(t *testing.T, files map[string]string) (string, string) {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "--quiet", "--initial-branch=main")
	git(t, root, "config", "user.email", "t@example.com")
	git(t, root, "config", "user.name", "t")
	git(t, root, "config", "commit.gpgsign", "false")
	for name, content := range files {
		writeFile(t, filepath.Join(root, name), content)
	}
	git(t, root, "add", "--all")
	git(t, root, "commit", "--quiet", "--allow-empty", "--message", "base")
	return root, git(t, root, "rev-parse", "HEAD")
}

// ten is a file of ten numbered lines.
const ten = "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"

func TestChangedLines(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		files  map[string]string // committed at the base
		change func(t *testing.T, root string)
		wanted []string
		want   map[string][]patchcov.Span
	}{
		{
			name:   "no change",
			files:  map[string]string{"a.txt": ten},
			change: func(*testing.T, string) {},
			wanted: []string{"a.txt"},
			want:   map[string][]patchcov.Span{},
		},
		{
			name:  "a changed line and an added run",
			files: map[string]string{"a.txt": ten},
			change: func(t *testing.T, root string) {
				t.Helper()
				writeFile(t, filepath.Join(root, "a.txt"), "1\n2\nthree\n4\n5\nnew\nnew\n6\n7\n8\n9\n10\n")
			},
			wanted: []string{"a.txt"},
			want:   map[string][]patchcov.Span{"a.txt": {{Start: 3, End: 3}, {Start: 6, End: 7}}},
		},
		{
			name:  "a deleted line changes nothing",
			files: map[string]string{"a.txt": ten},
			change: func(t *testing.T, root string) {
				t.Helper()
				writeFile(t, filepath.Join(root, "a.txt"), "1\n2\n3\n5\n6\n7\n8\n9\n10\n")
			},
			wanted: []string{"a.txt"},
			want:   map[string][]patchcov.Span{},
		},
		{
			name:  "a committed change counts like an uncommitted one",
			files: map[string]string{"a.txt": ten},
			change: func(t *testing.T, root string) {
				t.Helper()
				writeFile(t, filepath.Join(root, "a.txt"), "one\n2\n3\n4\n5\n6\n7\n8\n9\n10\n")
				git(t, root, "commit", "--quiet", "--all", "--message", "next")
				writeFile(t, filepath.Join(root, "a.txt"), "one\n2\n3\n4\n5\n6\n7\n8\n9\nten\n")
			},
			wanted: []string{"a.txt"},
			want:   map[string][]patchcov.Span{"a.txt": {{Start: 1, End: 1}, {Start: 10, End: 10}}},
		},
		{
			name:  "only wanted files are diffed",
			files: map[string]string{"a.txt": ten, "b.txt": ten},
			change: func(t *testing.T, root string) {
				t.Helper()
				writeFile(t, filepath.Join(root, "a.txt"), "x\n")
				writeFile(t, filepath.Join(root, "b.txt"), "x\n")
			},
			wanted: []string{"b.txt"},
			want:   map[string][]patchcov.Span{"b.txt": {{Start: 1, End: 1}}},
		},
		{
			name:  "a staged new file is all changed and an untracked one is unseen",
			files: map[string]string{},
			change: func(t *testing.T, root string) {
				t.Helper()
				writeFile(t, filepath.Join(root, "new.txt"), "a\nb\nc\n")
				writeFile(t, filepath.Join(root, "untracked.txt"), "a\n")
				git(t, root, "add", "new.txt")
			},
			wanted: []string{"new.txt", "untracked.txt"},
			want:   map[string][]patchcov.Span{"new.txt": {{Start: 1, End: 3}}},
		},
		{
			name:  "a deleted file changes nothing",
			files: map[string]string{"a.txt": ten},
			change: func(t *testing.T, root string) {
				t.Helper()
				if err := os.Remove(filepath.Join(root, "a.txt")); err != nil {
					t.Fatal(err)
				}
			},
			wanted: []string{"a.txt"},
			want:   map[string][]patchcov.Span{},
		},
		{
			name:  "an added line that reads like a hunk header",
			files: map[string]string{"a.txt": ten},
			change: func(t *testing.T, root string) {
				t.Helper()
				writeFile(t, filepath.Join(root, "a.txt"), "1\n@@ -1 +100,5 @@\n3\n4\n5\n6\n7\n8\n9\n10\n")
			},
			wanted: []string{"a.txt"},
			want:   map[string][]patchcov.Span{"a.txt": {{Start: 2, End: 2}}},
		},
		{
			name:  "file names git would quote or pad",
			files: map[string]string{"sp ace.txt": ten, "tab\there.txt": ten, "qu\"ote.txt": ten, "café.txt": ten},
			change: func(t *testing.T, root string) {
				t.Helper()
				for _, name := range []string{"sp ace.txt", "tab\there.txt", "qu\"ote.txt", "café.txt"} {
					writeFile(t, filepath.Join(root, name), "1\n2\nx\n4\n5\n6\n7\n8\n9\n10\n")
				}
			},
			wanted: []string{"sp ace.txt", "tab\there.txt", "qu\"ote.txt", "café.txt"},
			want: map[string][]patchcov.Span{
				"sp ace.txt": {{Start: 3, End: 3}}, "tab\there.txt": {{Start: 3, End: 3}},
				"qu\"ote.txt": {{Start: 3, End: 3}}, "café.txt": {{Start: 3, End: 3}},
			},
		},
		{
			name:  "a file marked binary is still diffed as text",
			files: map[string]string{"a.txt": ten, ".gitattributes": "a.txt -diff\n"},
			change: func(t *testing.T, root string) {
				t.Helper()
				writeFile(t, filepath.Join(root, "a.txt"), "1\n2\n3\nx\n5\n6\n7\n8\n9\n10\n")
			},
			wanted: []string{"a.txt"},
			want:   map[string][]patchcov.Span{"a.txt": {{Start: 4, End: 4}}},
		},
		{
			name:  "git config that reshapes diff output is overridden",
			files: map[string]string{"sub/a.txt": ten, ".gitattributes": "*.txt diff=blank\n"},
			change: func(t *testing.T, root string) {
				t.Helper()
				for _, kv := range [][2]string{
					{"diff.noprefix", "true"},
					{"diff.mnemonicPrefix", "true"},
					{"diff.interHunkContext", "10"},
					{"diff.context", "5"},
					{"diff.relative", "true"},
					{"diff.renames", "copies"},
					{"diff.external", "false"},
					{"diff.blank.textconv", "true"},
					{"color.diff", "always"},
					{"core.quotePath", "true"},
				} {
					git(t, root, "config", kv[0], kv[1])
				}
				writeFile(t, filepath.Join(root, "sub/a.txt"), "x\n2\n3\n4\n5\n6\n7\n8\n9\ny\n")
			},
			wanted: []string{"sub/a.txt"},
			want:   map[string][]patchcov.Span{"sub/a.txt": {{Start: 1, End: 1}, {Start: 10, End: 10}}},
		},
		{
			name:  "a file without a final newline",
			files: map[string]string{"a.txt": "1\n2"},
			change: func(t *testing.T, root string) {
				t.Helper()
				writeFile(t, filepath.Join(root, "a.txt"), "1\n2\n3")
			},
			wanted: []string{"a.txt"},
			want:   map[string][]patchcov.Span{"a.txt": {{Start: 2, End: 3}}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, base := newRepo(t, tc.files)
			tc.change(t, root)
			wanted := map[string]bool{}
			for _, name := range tc.wanted {
				wanted[name] = true
			}
			got, err := patchcov.ChangedLines(t.Context(), root, base, wanted)
			if err != nil {
				t.Fatalf("ChangedLines error = %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ChangedLines mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestChangedLines_gitFails(t *testing.T) {
	t.Parallel()
	root, _ := newRepo(t, map[string]string{"a.txt": ten})
	if _, err := patchcov.ChangedLines(t.Context(), root, "0000000000000000000000000000000000000000", map[string]bool{"a.txt": true}); err == nil {
		t.Error("ChangedLines error = nil, want an error for a base git cannot diff against")
	}
}

// TestChangedLines_gitOutput has no t.Parallel: it puts a fake git first on
// PATH, which lists a changed file and then misbehaves diffing it.
func TestChangedLines_gitOutput(t *testing.T) {
	tests := []struct {
		name string
		diff string // the fake's diff of the file, as shell
		want string
	}{
		{name: "an unreadable hunk header", diff: "printf '@@ -1 +x @@\\n'", want: "unexpected hunk header"},
		{name: "a failing diff of one file", diff: "echo boom >&2; exit 3", want: "boom"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root, base := newRepo(t, map[string]string{"a.txt": ten})
			bin := t.TempDir()
			writeFile(t, filepath.Join(bin, "git"), "#!/bin/sh\ncase \" $* \" in\n*' --name-only '*) printf 'a.txt\\0' ;;\n*) "+tc.diff+" ;;\nesac\n")
			if err := os.Chmod(filepath.Join(bin, "git"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			_, err := patchcov.ChangedLines(t.Context(), root, base, map[string]bool{"a.txt": true})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("ChangedLines error = %v, want one containing %q", err, tc.want)
			}
		})
	}
}
