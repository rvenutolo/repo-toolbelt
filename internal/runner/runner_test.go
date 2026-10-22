package runner_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/manifest"
	"github.com/rvenutolo/repo-toolbelt/internal/runner"
)

// writeLint installs a fixture lint at <dir>/<name>/<name>: a bash script with
// the given body. Fixtures only print markers, touch files under t.TempDir()
// and exit with chosen codes.
func writeLint(t *testing.T, dir, name, body string) {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatalf("bash not on PATH: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!" + bash + "\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, name, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func lintsNamed(names ...string) []manifest.Lint {
	lints := make([]manifest.Lint, len(names))
	for i, name := range names {
		lints[i] = manifest.Lint{Name: name, Language: "bash", Speed: "quick", Description: name}
	}
	return lints
}

func TestRun_callingConvention(t *testing.T) {
	t.Parallel()
	lintsDir, repo := t.TempDir(), t.TempDir()
	writeLint(t, lintsDir, "probe", `
printf 'cwd=%s\n' "$(pwd -P)"
printf 'args=%s\n' "$#"
printf 'hook=%s\n' "${TOOLBELT_HOOK}"
printf 'base=%s\n' "${TOOLBELT_BASE}"
printf 'range=%s\n' "${TOOLBELT_RANGE}"
printf 'extra=%s\n' "${EXTRA}"
printf 'section=%s\n' "$(cat "${TOOLBELT_SECTION}")"`)
	writeLint(t, lintsDir, "nosection", `printf '%s' "$(cat "${TOOLBELT_SECTION}")"`)

	results := runner.Run(t.Context(), lintsNamed("probe", "nosection"), runner.Options{
		LintsDir: lintsDir,
		RepoRoot: repo,
		Hook:     "pre-push",
		Base:     "abc123",
		Range:    "abc123..def456",
		Sections: map[string][]byte{"probe": []byte(`{"exempt":[{"path":"x y"}]}`)},
		Env:      []string{"EXTRA=from options"},
	})

	realRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	want := []runner.Result{
		{
			Lint: "probe",
			Exit: 0,
			Output: []byte("cwd=" + realRepo + "\nargs=0\nhook=pre-push\nbase=abc123\nrange=abc123..def456\n" +
				"extra=from options\n" + `section={"exempt":[{"path":"x y"}]}` + "\n"),
		},
		{Lint: "nosection", Exit: 0, Output: []byte("null")},
	}
	if diff := cmp.Diff(want, results, cmp.Comparer(bytes.Equal)); diff != "" {
		t.Errorf("results (-want +got):\n%s", diff)
	}
}

func TestRun_lintsRunConcurrently(t *testing.T) {
	t.Parallel()
	lintsDir, markers := t.TempDir(), t.TempDir()
	// Each waits up to 5 s for the other's marker, so both pass only when they
	// run at the same time.
	waitFor := func(mine, theirs string) string {
		return `touch '` + filepath.Join(markers, mine) + `'
for _ in $(seq 50); do
  if [[ -e '` + filepath.Join(markers, theirs) + `' ]]; then echo saw-` + theirs + `; exit 0; fi
  sleep 0.1
done
exit 1`
	}
	writeLint(t, lintsDir, "one", waitFor("one", "two"))
	writeLint(t, lintsDir, "two", waitFor("two", "one"))

	results := runner.Run(t.Context(), lintsNamed("one", "two"), runner.Options{LintsDir: lintsDir, RepoRoot: t.TempDir()})

	want := []runner.Result{
		{Lint: "one", Exit: 0, Output: []byte("saw-two\n")},
		{Lint: "two", Exit: 0, Output: []byte("saw-one\n")},
	}
	if diff := cmp.Diff(want, results, cmp.Comparer(bytes.Equal)); diff != "" {
		t.Errorf("results (-want +got):\n%s", diff)
	}
}

func TestRun_resultsFollowInputOrder(t *testing.T) {
	t.Parallel()
	lintsDir, markers := t.TempDir(), t.TempDir()
	done := filepath.Join(markers, "fast-done")
	// "slow" finishes only after "fast" has finished.
	writeLint(t, lintsDir, "slow", `for _ in $(seq 50); do [[ -e '`+done+`' ]] && break; sleep 0.1; done; echo slow`)
	writeLint(t, lintsDir, "fast", `echo fast; touch '`+done+`'`)

	results := runner.Run(t.Context(), lintsNamed("slow", "fast"), runner.Options{LintsDir: lintsDir, RepoRoot: t.TempDir()})

	want := []runner.Result{
		{Lint: "slow", Exit: 0, Output: []byte("slow\n")},
		{Lint: "fast", Exit: 0, Output: []byte("fast\n")},
	}
	if diff := cmp.Diff(want, results, cmp.Comparer(bytes.Equal)); diff != "" {
		t.Errorf("results (-want +got):\n%s", diff)
	}
}

func TestRun_exitCodes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		body       string
		wantExit   int
		wantStderr string
	}{
		{name: "pass", body: "exit 0", wantExit: 0},
		{name: "findings", body: "exit 1", wantExit: 1},
		{name: "setup failure", body: "exit 2", wantExit: 2},
		{name: "exit 3 is an error", body: "exit 3", wantExit: 2, wantStderr: "toolbelt: lint exited 3"},
		{name: "reason on its own line", body: "printf 'diag' >&2; exit 3", wantExit: 2, wantStderr: "diag\ntoolbelt: lint exited 3\n"},
		{name: "exit 255 is an error", body: "exit 255", wantExit: 2, wantStderr: "toolbelt: lint exited 255"},
		// The fixture signals only its own process.
		{name: "killed by a signal", body: "kill -s KILL $$", wantExit: 2, wantStderr: "toolbelt: lint killed by signal: killed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lintsDir := t.TempDir()
			writeLint(t, lintsDir, "fixture", tc.body)
			results := runner.Run(t.Context(), lintsNamed("fixture"), runner.Options{LintsDir: lintsDir, RepoRoot: t.TempDir()})
			if len(results) != 1 {
				t.Fatalf("got %d results, want 1", len(results))
			}
			if results[0].Exit != tc.wantExit {
				t.Errorf("Exit = %d, want %d", results[0].Exit, tc.wantExit)
			}
			if got := string(results[0].Stderr); !strings.Contains(got, tc.wantStderr) {
				t.Errorf("Stderr = %q, want it to contain %q", got, tc.wantStderr)
			}
		})
	}
}

func TestRun_outputIntact(t *testing.T) {
	t.Parallel()
	lintsDir := t.TempDir()
	writeLint(t, lintsDir, "big", `head -c 1048576 /dev/zero | tr '\0' x`)
	writeLint(t, lintsDir, "partial", `printf 'no newline'`)
	writeLint(t, lintsDir, "stderr", `printf 'out\n'; printf 'diag one\ndiag two' >&2`)

	results := runner.Run(t.Context(), lintsNamed("big", "partial", "stderr"), runner.Options{LintsDir: lintsDir, RepoRoot: t.TempDir()})

	want := []runner.Result{
		{Lint: "big", Exit: 0, Output: bytes.Repeat([]byte("x"), 1<<20)},
		{Lint: "partial", Exit: 0, Output: []byte("no newline")},
		{Lint: "stderr", Exit: 0, Output: []byte("out\n"), Stderr: []byte("diag one\ndiag two")},
	}
	if diff := cmp.Diff(want, results, cmp.Comparer(bytes.Equal)); diff != "" {
		t.Errorf("results (-want +got):\n%s", diff)
	}
}

func TestRun_missingExecutable(t *testing.T) {
	t.Parallel()
	lintsDir := t.TempDir()
	writeLint(t, lintsDir, "present", "echo ran")

	results := runner.Run(t.Context(), lintsNamed("absent", "present"), runner.Options{LintsDir: lintsDir, RepoRoot: t.TempDir()})

	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Lint != "absent" || results[0].Exit != 2 {
		t.Errorf("absent: got %+v, want exit 2", results[0])
	}
	wantPath := filepath.Join(lintsDir, "absent", "absent")
	if got := string(results[0].Stderr); !strings.Contains(got, wantPath) {
		t.Errorf("absent: Stderr = %q, want it to name %q", got, wantPath)
	}
	if results[1].Exit != 0 || string(results[1].Output) != "ran\n" {
		t.Errorf("present: got %+v, want exit 0 and %q", results[1], "ran\n")
	}
}

func TestRun_cancelKillsLints(t *testing.T) {
	t.Parallel()
	lintsDir, dir := t.TempDir(), t.TempDir()
	fifo := filepath.Join(dir, "started")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	// The sleep is a child of the lint's shell, so ending it needs the whole
	// process group signalled, not only the shell.
	writeLint(t, lintsDir, "hang", `echo started > '`+fifo+`'; sleep 60; echo not-cancelled`)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan error, 1)
	go func() {
		// Reading the FIFO blocks until the lint has written its line and
		// closed its end, so the lint is mid-sleep when ctx is cancelled.
		_, err := os.ReadFile(fifo)
		started <- err
		cancel()
	}()
	start := time.Now()
	results := runner.Run(ctx, lintsNamed("hang"), runner.Options{LintsDir: lintsDir, RepoRoot: t.TempDir()})
	elapsed := time.Since(start)

	// If the lint never opened the FIFO, the reader is still blocked in open;
	// opening the write end without blocking releases it. ENXIO means the
	// reader is already gone, which is the normal case.
	if w, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
		w.Close()
	}
	if err := <-started; err != nil {
		t.Fatalf("read start marker: %v", err)
	}
	if len(results) != 1 || results[0].Exit != 2 {
		t.Fatalf("got %+v, want one result with exit 2", results)
	}
	if strings.Contains(string(results[0].Output), "not-cancelled") {
		t.Errorf("lint ran to completion after cancel: %q", results[0].Output)
	}
	// Well under the 60 s sleep and under the runner's grace period for
	// processes that ignore the signal.
	if elapsed > 4*time.Second {
		t.Errorf("Run took %v after cancel, want the lint's process group killed promptly", elapsed)
	}
}

func TestRun_noTempDir(t *testing.T) {
	// Not parallel: the section files' directory comes from TMPDIR.
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	lintsDir := t.TempDir()
	writeLint(t, lintsDir, "fixture", "echo ran")

	results := runner.Run(t.Context(), lintsNamed("fixture"), runner.Options{LintsDir: lintsDir, RepoRoot: t.TempDir()})

	if len(results) != 1 || results[0].Exit != 2 || len(results[0].Output) != 0 {
		t.Fatalf("got %+v, want one errored result that never ran", results)
	}
	if got := string(results[0].Stderr); !strings.Contains(got, "toolbelt: cannot create a directory for config sections") {
		t.Errorf("Stderr = %q, want the reason", got)
	}
}
