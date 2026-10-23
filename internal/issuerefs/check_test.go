package issuerefs_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/issuerefs"
	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
)

// gitEnv isolates fixture git commands from the machine's git config.
var gitEnv = []string{"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), gitEnv...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
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

// makeRepo returns a git repository holding files, all added to the index.
func makeRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	repo := t.TempDir()
	gitIn(t, repo, "init", "--quiet", "--initial-branch=main")
	for name, content := range files {
		writeFile(t, filepath.Join(repo, name), content)
	}
	gitIn(t, repo, "add", "--all")
	return repo
}

func ref(path string, line int, text string) lintkit.Finding {
	return lintkit.Finding{Path: path, Line: line, Message: "issue or PR reference " + text + "; state the reason instead"}
}

func TestCheck(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t, map[string]string{
		"a.md":         "intro\nfixed in #540\n",
		"bin.dat":      "\x00\x01binary #99\n",
		"CHANGELOG.md": "## 1.0 (#5)\n",
		"t.md":         "| P-1 | #3 |\nsee #4\n",
		"src/x.sh":     "echo \"${#items[@]}\" $#\n",
		"gone.txt":     "#6\n",
	})
	writeFile(t, filepath.Join(repo, "untracked.md"), "#77\n")
	if err := os.Remove(filepath.Join(repo, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	section := `{"exempt":[
		{"path":"CHANGELOG.md","reason":"release tooling writes the links"},
		{"path":"t.md","match":"^\\| P-","reason":"probe ids"},
		{"path":"old.md","reason":"no longer exists"}
	]}`

	got, err := issuerefs.Check(t.Context(), lintkit.Context{Root: repo, Section: []byte(section)})
	if err != nil {
		t.Fatalf("Check error = %v", err)
	}
	want := []lintkit.Finding{
		ref("a.md", 2, "#540"),
		ref("t.md", 2, "#4"),
		{Path: ".repo-toolbelt.yml", Message: "stale exemption: old.md suppresses nothing"},
	}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(lintkit.Finding{})); diff != "" {
		t.Errorf("Check mismatch (-want +got):\n%s", diff)
	}
}

func TestCheck_noSectionStillScans(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t, map[string]string{"CHANGELOG.md": "## 1.0 (#5)\n"})
	got, err := issuerefs.Check(t.Context(), lintkit.Context{Root: repo, Section: []byte("null")})
	if err != nil {
		t.Fatalf("Check error = %v", err)
	}
	want := []lintkit.Finding{ref("CHANGELOG.md", 1, "#5")}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(lintkit.Finding{})); diff != "" {
		t.Errorf("Check mismatch (-want +got):\n%s", diff)
	}
}

func TestCheck_sectionProblemsStopTheScan(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t, map[string]string{"a.md": "#1\n"})
	got, err := issuerefs.Check(t.Context(), lintkit.Context{Root: repo, Section: []byte(`{"exempt":[{"path":"a.md"}]}`)})
	if err != nil {
		t.Fatalf("Check error = %v", err)
	}
	want := []lintkit.Finding{lintkit.SectionError("exempt[0]", "reason is missing or empty")}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(lintkit.Finding{})); diff != "" {
		t.Errorf("Check mismatch (-want +got):\n%s", diff)
	}
}

func TestCheck_emptyRepoPasses(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t, nil)
	got, err := issuerefs.Check(t.Context(), lintkit.Context{Root: repo, Section: []byte("null")})
	if err != nil {
		t.Fatalf("Check error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Check = %v, want no findings", got)
	}
}

func TestCheck_symlinksAreNotFollowed(t *testing.T) {
	t.Parallel()
	outside := filepath.Join(t.TempDir(), "outside.md")
	writeFile(t, outside, "#2\n")
	repo := t.TempDir()
	gitIn(t, repo, "init", "--quiet", "--initial-branch=main")
	writeFile(t, filepath.Join(repo, "a.md"), "#1\n")
	for name, target := range map[string]string{"inside.md": "a.md", "outside.md": outside} {
		if err := os.Symlink(target, filepath.Join(repo, name)); err != nil {
			t.Fatal(err)
		}
	}
	gitIn(t, repo, "add", "--all")
	got, err := issuerefs.Check(t.Context(), lintkit.Context{Root: repo, Section: []byte("null")})
	if err != nil {
		t.Fatalf("Check error = %v", err)
	}
	want := []lintkit.Finding{ref("a.md", 1, "#1")}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(lintkit.Finding{})); diff != "" {
		t.Errorf("Check mismatch (-want +got):\n%s", diff)
	}
}

func TestCheck_setupFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		root    string
		section string
	}{
		{name: "not a git repository", root: t.TempDir(), section: "null"},
		{name: "malformed section", root: makeRepo(t, nil), section: "{"},
		{name: "root does not exist", root: filepath.Join(t.TempDir(), "absent"), section: "null"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := issuerefs.Check(t.Context(), lintkit.Context{Root: tc.root, Section: []byte(tc.section)})
			if err == nil {
				t.Errorf("Check error = nil with findings %v, want an error", got)
			}
		})
	}
}

func TestCheck_cancelledContextStopsGit(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t, map[string]string{"a.md": "#1\n"})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := issuerefs.Check(ctx, lintkit.Context{Root: repo, Section: []byte("null")})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Check error = %v with findings %v, want context.Canceled", err, got)
	}
}

func TestCheck_doesNotRecurseIntoSubmodules(t *testing.T) {
	t.Parallel()
	sub := makeRepo(t, map[string]string{"inner.md": "#1\n"})
	gitIn(t, sub, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "--quiet", "--message=init")
	repo := makeRepo(t, map[string]string{"a.md": "clean\n"})
	gitIn(t, repo, "-c", "protocol.file.allow=always", "submodule", "--quiet", "add", sub, "sub")
	// A consumer's repo config could ask every git command to recurse.
	gitIn(t, repo, "config", "submodule.recurse", "true")
	got, err := issuerefs.Check(t.Context(), lintkit.Context{Root: repo, Section: []byte("null")})
	if err != nil {
		t.Fatalf("Check error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Check = %v, want no findings from the submodule", got)
	}
}

func TestCheck_unreadableTrackedFileIsASetupError(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads files whatever their mode")
	}
	tests := []struct {
		name       string
		unreadable string // made mode 0o000, relative to the repo
		want       string // part of the error
	}{
		// b.md still matches, so git grep exits 0 having skipped a.md.
		{name: "a file with no permissions beside a readable one", unreadable: "a.md", want: "a.md"},
		{name: "a directory with no permissions", unreadable: "sub", want: "sub/c.md"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := makeRepo(t, map[string]string{"a.md": "#1\n", "b.md": "#2\n", "sub/c.md": "#3\n"})
			target := filepath.Join(repo, tc.unreadable)
			if err := os.Chmod(target, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(target, 0o755) }) //nolint:errcheck // lets TempDir clean up; a failure only leaves the dir
			// git also warns that it cannot read sub/.gitattributes.
			lc := lintkit.Context{Root: repo, Section: []byte("null"), Stderr: io.Discard}
			got, err := issuerefs.Check(t.Context(), lc)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Check = %v, %v; want an error naming %q", got, err, tc.want)
			}
		})
	}
}

func TestCheck_gitWarningPassesThroughAndFindingsStand(t *testing.T) {
	t.Parallel()
	// git ignores a negative attribute pattern, warns on stderr, and still
	// exits 0 with the full listing.
	repo := makeRepo(t, map[string]string{".gitattributes": "!*.md text\n", "a.md": "#1\n"})
	var stderr bytes.Buffer
	got, err := issuerefs.Check(t.Context(), lintkit.Context{Root: repo, Section: []byte("null"), Stderr: &stderr})
	if err != nil {
		t.Fatalf("Check error = %v", err)
	}
	if diff := cmp.Diff([]lintkit.Finding{ref("a.md", 1, "#1")}, got, cmp.AllowUnexported(lintkit.Finding{})); diff != "" {
		t.Errorf("Check mismatch (-want +got):\n%s", diff)
	}
	if !strings.HasPrefix(stderr.String(), "warning: Negative patterns are ignored in git attributes\n") {
		t.Errorf("stderr = %q, want git's warning passed through", stderr.String())
	}
}

func TestCheck_gitWarningWithNoStderrIsDiscarded(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t, map[string]string{".gitattributes": "!*.md text\n", "a.md": "#1\n"})
	got, err := issuerefs.Check(t.Context(), lintkit.Context{Root: repo, Section: []byte("null")})
	if err != nil {
		t.Fatalf("Check error = %v", err)
	}
	if diff := cmp.Diff([]lintkit.Finding{ref("a.md", 1, "#1")}, got, cmp.AllowUnexported(lintkit.Finding{})); diff != "" {
		t.Errorf("Check mismatch (-want +got):\n%s", diff)
	}
}

func TestCheck_trackedFileUnderSymlinkOutsideRepoIsASetupError(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "c.md"), "#9\n")
	repo := makeRepo(t, map[string]string{"a.md": "clean\n", "sub/c.md": "clean\n"})
	if err := os.RemoveAll(filepath.Join(repo, "sub")); err != nil {
		t.Fatal(err)
	}
	// git grep follows the symlinked directory and lists sub/c.md, which
	// the lint must not read outside the repository.
	if err := os.Symlink(outside, filepath.Join(repo, "sub")); err != nil {
		t.Fatal(err)
	}
	got, err := issuerefs.Check(t.Context(), lintkit.Context{Root: repo, Section: []byte("null")})
	if err == nil || !strings.Contains(err.Error(), "sub/c.md") {
		t.Errorf("Check = %v, %v; want a setup error naming sub/c.md", got, err)
	}
}
