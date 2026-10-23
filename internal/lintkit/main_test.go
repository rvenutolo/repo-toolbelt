package lintkit_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
)

// mainResult is what lintkit.Main returned and wrote to the process streams.
type mainResult struct {
	status         int
	stdout, stderr string
}

// runMain calls lintkit.Main with os.Args and the process streams replaced,
// restoring them afterwards. It cannot run in parallel: it swaps process state.
func runMain(t *testing.T, check lintkit.Check, args ...string) mainResult {
	t.Helper()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldArgs, oldOut, oldErr := os.Args, os.Stdout, os.Stderr
	os.Args, os.Stdout, os.Stderr = append([]string{"demo"}, args...), outW, errW
	status := lintkit.Main("demo", "widget", check)
	os.Args, os.Stdout, os.Stderr = oldArgs, oldOut, oldErr
	_ = outW.Close()
	_ = errW.Close()
	stdout, err := io.ReadAll(outR)
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := io.ReadAll(errR)
	if err != nil {
		t.Fatal(err)
	}
	return mainResult{status: status, stdout: string(stdout), stderr: string(stderr)}
}

func setSection(t *testing.T) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "section.json")
	if err := os.WriteFile(file, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TOOLBELT_SECTION", file)
}

func TestMain_noFindingsExitsZero(t *testing.T) {
	setSection(t)
	t.Chdir(t.TempDir())
	got := runMain(t, func(context.Context, lintkit.Context) ([]lintkit.Finding, error) { return nil, nil })
	if got.status != 0 || got.stdout != "" || got.stderr != "" {
		t.Errorf("got %+v, want exit 0 and no output", got)
	}
}

func TestMain_findingsExitOneAndAreCounted(t *testing.T) {
	setSection(t)
	t.Chdir(t.TempDir())
	got := runMain(t, func(context.Context, lintkit.Context) ([]lintkit.Finding, error) {
		return []lintkit.Finding{{Path: "a.txt", Line: 3, Message: "bad"}}, nil
	})
	if got.status != 1 || got.stdout != "a.txt:3: bad\n1 widget(s)\n" {
		t.Errorf("got %+v, want exit 1 and the finding with its count", got)
	}
}

func TestMain_checkRunsInTheWorkingDirectory(t *testing.T) {
	setSection(t)
	dir := t.TempDir()
	t.Chdir(dir)
	var root string
	runMain(t, func(_ context.Context, lc lintkit.Context) ([]lintkit.Finding, error) {
		root = lc.Root
		return nil, nil
	})
	want, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if root != want {
		t.Errorf("root = %q, want the working directory %q", root, want)
	}
}

func TestMain_helpPrintsUsageAndExitsZero(t *testing.T) {
	t.Chdir(t.TempDir())
	got := runMain(t, nil, "--help")
	if got.status != 0 || !strings.HasPrefix(got.stdout, "Usage: demo") {
		t.Errorf("got %+v, want exit 0 and the usage", got)
	}
}

func TestMain_unreadableWorkingDirectoryExitsTwo(t *testing.T) {
	setSection(t)
	gone := filepath.Join(t.TempDir(), "gone")
	if err := os.Mkdir(gone, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(gone)
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	// Getwd trusts $PWD when it names the current directory, so unset it.
	t.Setenv("PWD", "")
	if err := os.Unsetenv("PWD"); err != nil {
		t.Fatal(err)
	}
	got := runMain(t, nil)
	if got.status != 2 || !strings.HasPrefix(got.stderr, "demo: ") || got.stdout != "" {
		t.Errorf("got %+v, want exit 2 and `demo: ...` on stderr", got)
	}
}
