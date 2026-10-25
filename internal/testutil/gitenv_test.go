package testutil_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rvenutolo/repo-toolbelt/internal/testutil"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// git runs git in dir with the process environment, as the code under test
// does, and returns its trimmed output and exit code.
func git(t *testing.T, dir string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	code := cmd.ProcessState.ExitCode()
	if err != nil && code < 0 {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out)), code
}

func TestIsolateGit(t *testing.T) {
	// Not parallel: it changes the process environment. A host setup is
	// planted first, as a developer's machine would have one, and t.Setenv
	// restores each variable when the test ends.
	host := t.TempDir()
	writeFile(t, filepath.Join(host, "git", "ignore"), "ignored.txt\n")
	writeFile(t, filepath.Join(host, "git", "attributes"), "* linguist-vendored\n")
	writeFile(t, filepath.Join(host, "gitconfig"), "[toolbelt]\n\tprobe = host\n")
	t.Setenv("XDG_CONFIG_HOME", host)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(host, "gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "")
	t.Setenv("GIT_ATTR_NOSYSTEM", "")

	if err := testutil.IsolateGit(); err != nil {
		t.Fatalf("IsolateGit() = %v, want nil", err)
	}

	repo := t.TempDir()
	if _, code := git(t, repo, "init", "--quiet"); code != 0 {
		t.Fatalf("git init exited %d", code)
	}
	writeFile(t, filepath.Join(repo, "ignored.txt"), "x\n")
	if out, code := git(t, repo, "check-ignore", "ignored.txt"); code != 1 {
		t.Errorf("git check-ignore = %q, exit %d; want exit 1, the host excludes file hidden", out, code)
	}
	out, _ := git(t, repo, "check-attr", "linguist-vendored", "--", "ignored.txt")
	if want := "ignored.txt: linguist-vendored: unspecified"; out != want {
		t.Errorf("git check-attr = %q, want %q, the host attributes file hidden", out, want)
	}
	if out, code := git(t, repo, "config", "--get", "toolbelt.probe"); code != 1 {
		t.Errorf("git config --get = %q, exit %d; want exit 1, the host global config hidden", out, code)
	}
	// The system attributes file sits under git's compiled-in prefix, which a
	// test cannot write, so git is asked where it would read it: nowhere.
	if out, code := git(t, repo, "var", "GIT_ATTR_SYSTEM"); out != "" || code == 0 {
		t.Errorf("git var GIT_ATTR_SYSTEM = %q, exit %d; want no path, the system attributes file skipped", out, code)
	}
}
