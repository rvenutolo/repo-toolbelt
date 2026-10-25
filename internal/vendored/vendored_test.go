package vendored_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/vendored"
)

// gitEnv isolates fixture git commands from the machine's git config,
// including the global excludes file, which may list vendor/.
var gitEnv = []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "XDG_CONFIG_HOME=/nonexistent"}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), gitEnv...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// makeRepo returns a git repository holding files, all added to the index.
func makeRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	repo := t.TempDir()
	gitIn(t, repo, "init", "--quiet", "--initial-branch=main")
	for name, content := range files {
		path := filepath.Join(repo, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitIn(t, repo, "add", "--all")
	return repo
}

func TestInVendorDir(t *testing.T) {
	t.Parallel()
	tests := []struct {
		file string
		want bool
	}{
		{file: "vendor/a.go", want: true},
		{file: "a/vendor/b.go", want: true},
		{file: "a/vendor/deep/c.go", want: true},
		{file: "vendor", want: false},
		{file: "x/vendor", want: false},
		{file: "vendored.go", want: false},
		{file: "myvendor/x.go", want: false},
		{file: "vendors/x.go", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()
			if got := vendored.InVendorDir(tc.file); got != tc.want {
				t.Errorf("InVendorDir(%q) = %v, want %v", tc.file, got, tc.want)
			}
		})
	}
}

func TestByAttribute(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t, map[string]string{
		".gitattributes":   "third_party/** linguist-vendored\nkept/** linguist-vendored=false\nexplicit.go linguist-vendored=true\n",
		"third_party/x.go": "package x\n",
		"kept/y.go":        "package y\n",
		"explicit.go":      "package e\n",
		"plain.go":         "package p\n",
	})
	got, err := vendored.ByAttribute(t.Context(), repo, []string{"third_party/x.go", "kept/y.go", "explicit.go", "plain.go"})
	if err != nil {
		t.Fatalf("ByAttribute error = %v", err)
	}
	want := map[string]bool{"third_party/x.go": true, "explicit.go": true}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ByAttribute mismatch (-want +got):\n%s", diff)
	}
}

func TestByAttribute_noFilesRunsNoGit(t *testing.T) {
	t.Parallel()
	// Not a repository: a git call would fail.
	got, err := vendored.ByAttribute(t.Context(), t.TempDir(), nil)
	if err != nil {
		t.Fatalf("ByAttribute error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ByAttribute = %v, want empty", got)
	}
}

func TestByAttribute_gitFailureIsAnError(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := vendored.ByAttribute(ctx, t.TempDir(), []string{"a.go"})
	if err == nil {
		t.Fatal("ByAttribute with a cancelled context: error = nil, want one")
	}
	if msg := err.Error(); strings.HasSuffix(msg, ": ") {
		t.Errorf("error %q ends with an empty stderr suffix", msg)
	}
}

func TestFilter(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t, map[string]string{
		".gitattributes":   "third_party/** linguist-vendored\n",
		"vendor/a.go":      "package a\n",
		"third_party/x.go": "package x\n",
		"src/b.go":         "package b\n",
		"README.md":        "readme\n",
	})
	files := []string{"README.md", "src/b.go", "third_party/x.go", "vendor/a.go"}
	got, err := vendored.Filter(t.Context(), repo, files)
	if err != nil {
		t.Fatalf("Filter error = %v", err)
	}
	if diff := cmp.Diff([]string{"README.md", "src/b.go"}, got); diff != "" {
		t.Errorf("Filter mismatch (-want +got):\n%s", diff)
	}
}

func TestExcludePathspecs_matchFilter(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t, map[string]string{
		".gitattributes":        "third_party/** linguist-vendored\nkept/** linguist-vendored=false\nexplicit.go linguist-vendored=true\n",
		"vendor/a.go":           "package a\n",
		"deep/vendor/b.go":      "package b\n",
		"x/vendor":              "a file named vendor\n",
		"vendored.go":           "package v\n",
		"myvendor/c.go":         "package c\n",
		"third_party/x.go":      "package x\n",
		"kept/y.go":             "package y\n",
		"explicit.go":           "package e\n",
		"src/[ab]*?.go":         "package s\n",
		"src/:(exclude)name.go": "package s\n",
		"README.md":             "readme\n",
	})
	listing := exec.CommandContext(t.Context(), "git", append([]string{"ls-files", "-z", "--", "."}, vendored.ExcludePathspecs()...)...)
	listing.Dir = repo
	out, err := listing.Output()
	if err != nil {
		t.Fatalf("git ls-files with the pathspecs: %v", err)
	}
	got := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")

	all := exec.CommandContext(t.Context(), "git", "ls-files", "-z")
	all.Dir = repo
	everything, err := all.Output()
	if err != nil {
		t.Fatal(err)
	}
	want, err := vendored.Filter(t.Context(), repo, strings.Split(strings.TrimSuffix(string(everything), "\x00"), "\x00"))
	if err != nil {
		t.Fatalf("Filter error = %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("pathspecs and Filter disagree (-Filter +pathspecs):\n%s", diff)
	}
}

func TestFilter_gitFailureIsAnError(t *testing.T) {
	t.Parallel()
	if _, err := vendored.Filter(t.Context(), t.TempDir(), []string{"a.go"}); err == nil {
		t.Fatal("Filter outside a repository: error = nil, want one")
	}
}
