package cli_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/cli"
	"github.com/rvenutolo/repo-toolbelt/internal/manifest"
)

// runLints are the manifest entries the run tests select from.
var runLints = []manifest.Lint{
	{Name: "a", Language: "bash", Speed: "quick", Description: "first"},
	{Name: "b", Language: "bash", Speed: "slow", Description: "second"},
	{Name: "c", Language: "go", Speed: "quick", Description: "third"},
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
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

// makeRepo returns a throwaway git repository with one commit holding
// tracked.txt and src/x.go.
func makeRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitIn(t, repo, "init", "--quiet", "--initial-branch=main")
	writeFile(t, filepath.Join(repo, "tracked.txt"), "one\n")
	writeFile(t, filepath.Join(repo, "src", "x.go"), "package x\n")
	gitIn(t, repo, "add", "--all")
	gitIn(t, repo, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "--quiet", "--message=init")
	return repo
}

// writeLint installs a fixture lint: a bash script that prints markers and
// exits with a chosen code.
func writeLint(t *testing.T, lintsDir, name, body string) {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatalf("bash not on PATH: %v", err)
	}
	path := filepath.Join(lintsDir, name, name)
	writeFile(t, path, "#!"+bash+"\n"+`[[ -z "${TOOLBELT_FIXTURE_PROBE-}" ]] || exit 0`+"\n"+body+"\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	waitExecutable(t, path)
}

// waitExecutable runs the fixture in probe mode (its first line exits at
// once) until the exec stops failing with ETXTBSY. A fork running in parallel
// can hold the write descriptor until it execs, and until then an exec of the
// file fails; the first exec that succeeds proves no holder is left.
func waitExecutable(t *testing.T, path string) {
	t.Helper()
	for range 500 {
		cmd := exec.CommandContext(t.Context(), path)
		cmd.Env = append(os.Environ(), "TOOLBELT_FIXTURE_PROBE=1")
		err := cmd.Run()
		if !errors.Is(err, syscall.ETXTBSY) {
			if err != nil {
				t.Fatalf("probe %s: %v", path, err)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s stayed busy", path)
}

// withRemote gives repo a bare "origin" holding its branch, and returns the
// flags and standard input a pre-push of a new branch at HEAD carries.
func withRemote(t *testing.T, repo string) (flags []string, stdin string) {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "origin.git")
	gitIn(t, repo, "init", "--quiet", "--bare", "--initial-branch=main", remote)
	gitIn(t, repo, "remote", "add", "origin", remote)
	gitIn(t, repo, "push", "--quiet", "origin", "main")
	gitIn(t, repo, "remote", "set-head", "origin", "main")
	sha := strings.TrimSpace(gitOut(t, repo, "rev-parse", "HEAD"))
	return []string{"--remote", "origin"}, "refs/heads/main " + sha + " refs/heads/new " + strings.Repeat("0", 40) + "\n"
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}

// runHook runs toolbelt with the flags and input a real run of hook carries.
func runHook(ctx context.Context, t *testing.T, repo, lintsDir, hook string, lints ...string) result {
	t.Helper()
	args := []string{"run", "--hook", hook}
	stdin := ""
	if hook == "pre-push" {
		var flags []string
		flags, stdin = withRemote(t, repo)
		args = append(args, flags...)
	}
	return runStdin(ctx, t, repo, lintsDir, stdin, append(args, lints...)...)
}

// runIn runs toolbelt from dir with lints from lintsDir, mirroring main.
func runIn(ctx context.Context, t *testing.T, dir, lintsDir string, args ...string) result {
	t.Helper()
	return runStdin(ctx, t, dir, lintsDir, "", args...)
}

// runStdin is runIn with the given standard input.
func runStdin(ctx context.Context, t *testing.T, dir, lintsDir, stdin string, args ...string) result {
	t.Helper()
	runner := cli.Runner{Lints: runLints, Version: "1.2.3", Dir: dir}
	return runWith(ctx, t, runner, map[string]string{"TOOLBELT_LINTS_DIR": lintsDir}, stdin, args...)
}

// runWith runs toolbelt as runner with the given environment and standard
// input, mirroring main.
func runWith(ctx context.Context, t *testing.T, runner cli.Runner, env map[string]string, stdin string, args ...string) result {
	t.Helper()
	var stdout, stderr bytes.Buffer
	err := runner.Run(ctx, args, func(k string) string { return env[k] }, strings.NewReader(stdin), &stdout, &stderr)
	if err != nil {
		stderr.WriteString("error: " + err.Error() + "\n")
	}
	return result{code: cli.ExitCode(err), stdout: stdout.String(), stderr: stderr.String()}
}

func TestRun_runPrintsBlocksInNamedOrder(t *testing.T) {
	t.Parallel()
	repo, lintsDir, markers := makeRepo(t), t.TempDir(), t.TempDir()
	cDone := filepath.Join(markers, "c-done")
	// a finishes last, but its block still comes first.
	writeLint(t, lintsDir, "a", `for _ in $(seq 50); do [[ -e '`+cDone+`' ]] && break; sleep 0.1; done; echo a-out`)
	writeLint(t, lintsDir, "b", `printf 'b-partial'; exit 1`)
	writeLint(t, lintsDir, "c", `printf 'c-diag\n' >&2; touch '`+cDone+`'; exit 2`)

	got := runHook(t.Context(), t, repo, lintsDir, "pre-push", "a", "b", "c")

	wantStdout := "== a: pass ==\na-out\n" +
		"== b: fail ==\nb-partial\n" +
		"== c: error ==\n" +
		"toolbelt: 1 passed, 1 failed, 1 errored\n"
	if diff := cmp.Diff(wantStdout, got.stdout); diff != "" {
		t.Errorf("stdout (-want +got):\n%s", diff)
	}
	if !strings.HasPrefix(got.stderr, "c-diag\n") {
		t.Errorf("stderr = %q, want it to start with c's stderr", got.stderr)
	}
	if got.code != 2 {
		t.Errorf("exit %d, want 2", got.code)
	}
}

func TestRun_runExitStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		a, c        string
		wantCode    int
		wantSummary string
	}{
		{name: "all pass", a: "exit 0", c: "exit 0", wantCode: 0, wantSummary: "toolbelt: 2 passed, 0 failed, 0 errored\n"},
		{name: "a finding", a: "exit 0", c: "exit 1", wantCode: 1, wantSummary: "toolbelt: 1 passed, 1 failed, 0 errored\n"},
		{name: "an error beats a finding", a: "exit 2", c: "exit 1", wantCode: 2, wantSummary: "toolbelt: 0 passed, 1 failed, 1 errored\n"},
		{name: "exit 3 is an error", a: "exit 3", c: "exit 1", wantCode: 2, wantSummary: "toolbelt: 0 passed, 1 failed, 1 errored\n"},
		// The fixture signals only its own process.
		{name: "killed by a signal", a: "kill -s KILL $$", c: "exit 0", wantCode: 2, wantSummary: "toolbelt: 1 passed, 0 failed, 1 errored\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, lintsDir := makeRepo(t), t.TempDir()
			writeLint(t, lintsDir, "a", tc.a)
			writeLint(t, lintsDir, "c", tc.c)
			got := runIn(t.Context(), t, repo, lintsDir, "run", "--hook", "pre-commit", "a", "c")
			if got.code != tc.wantCode {
				t.Errorf("exit %d, want %d (stderr %q)", got.code, tc.wantCode, got.stderr)
			}
			if !strings.HasSuffix(got.stdout, tc.wantSummary) {
				t.Errorf("stdout = %q, want it to end with %q", got.stdout, tc.wantSummary)
			}
		})
	}
}

func TestRun_runMissingExecutable(t *testing.T) {
	t.Parallel()
	repo, lintsDir := makeRepo(t), t.TempDir()
	writeLint(t, lintsDir, "a", "exit 0")
	got := runIn(t.Context(), t, repo, lintsDir, "run", "--hook", "pre-commit", "a", "c")
	if got.code != 2 {
		t.Errorf("exit %d, want 2", got.code)
	}
	if want := filepath.Join(lintsDir, "c", "c"); !strings.Contains(got.stderr, want) {
		t.Errorf("stderr = %q, want it to name %q", got.stderr, want)
	}
}

func TestRun_runCallingConvention(t *testing.T) {
	t.Parallel()
	repo, lintsDir := makeRepo(t), t.TempDir()
	writeFile(t, filepath.Join(repo, ".repo-toolbelt.yml"), "a:\n  x: 1\n")
	writeLint(t, lintsDir, "a", `printf 'cwd=%s hook=%s base=%s range=[%s] section=%s\n' "$(pwd -P)" "${TOOLBELT_HOOK}" "${TOOLBELT_BASE}" "${TOOLBELT_RANGE}" "$(cat "${TOOLBELT_SECTION}")"`)
	writeLint(t, lintsDir, "c", `printf 'section=%s\n' "$(cat "${TOOLBELT_SECTION}")"`)

	// Run from a subdirectory: lints still run at the repo root.
	got := runIn(t.Context(), t, filepath.Join(repo, "src"), lintsDir, "run", "--hook", "pre-commit", "a", "c")

	realRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	want := "== a: pass ==\ncwd=" + realRepo + ` hook=pre-commit base=HEAD range=[] section={"x":1}` + "\n" +
		"== c: pass ==\nsection=null\n" +
		"toolbelt: 2 passed, 0 failed, 0 errored\n"
	if got.code != 0 {
		t.Errorf("exit %d, want 0 (stderr %q)", got.code, got.stderr)
	}
	if diff := cmp.Diff(want, got.stdout); diff != "" {
		t.Errorf("stdout (-want +got):\n%s", diff)
	}
}

func TestRun_runLintsFromEnvironment(t *testing.T) {
	t.Parallel()
	repo, lintsDir := makeRepo(t), t.TempDir()
	writeLint(t, lintsDir, "c", "echo c-ran")
	var stdout, stderr bytes.Buffer
	env := map[string]string{"TOOLBELT_LINTS_DIR": lintsDir, "TOOLBELT_LINTS": "# quick\nc\n"}
	runner := cli.Runner{Lints: runLints, Dir: repo}
	err := runner.Run(t.Context(), []string{"run", "--hook", "ci"}, func(k string) string { return env[k] }, strings.NewReader(""), &stdout, &stderr)
	if err != nil || !strings.Contains(stdout.String(), "== c: pass ==\n") || !strings.Contains(stdout.String(), "\nc-ran\n") {
		t.Errorf("err = %v, stdout = %q, want c to run and pass", err, stdout.String())
	}
}

func TestRun_runDefaultLintsDir(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	var stdout, stderr bytes.Buffer
	runner := cli.Runner{Lints: runLints, Dir: repo}
	err := runner.Run(t.Context(), []string{"run", "--hook", "pre-commit", "a"}, func(string) string { return "" }, strings.NewReader(""), &stdout, &stderr)
	if cli.ExitCode(err) != 2 || !strings.Contains(stderr.String(), "/opt/toolbelt/lints/a/a") {
		t.Errorf("err = %v, stderr = %q, want exit 2 naming the default lint path", err, stderr.String())
	}
}

func TestRun_runSlowLintWarning(t *testing.T) {
	t.Parallel()
	const warning = "toolbelt: b is a slow lint; consider running it from pre-push\n"
	tests := []struct {
		hook string
		want bool
	}{
		{hook: "pre-commit", want: true},
		{hook: "pre-push", want: false},
		{hook: "ci", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.hook, func(t *testing.T) {
			t.Parallel()
			repo, lintsDir := makeRepo(t), t.TempDir()
			writeLint(t, lintsDir, "a", "exit 0")
			writeLint(t, lintsDir, "b", "echo b-ran")
			got := runHook(t.Context(), t, repo, lintsDir, tc.hook, "a", "b")
			if got.code != 0 || !strings.Contains(got.stdout, "== b: pass ==\n") || !strings.Contains(got.stdout, "\nb-ran\n") {
				t.Errorf("got %+v, want b to run and pass", got)
			}
			if gotWarning := strings.Contains(got.stderr, warning); gotWarning != tc.want {
				t.Errorf("stderr = %q, want slow warning %t", got.stderr, tc.want)
			}
			if strings.Contains(got.stderr, "toolbelt: a is a slow lint") {
				t.Errorf("stderr = %q, warned about a quick lint", got.stderr)
			}
		})
	}
}

func TestRun_runCIAnnotations(t *testing.T) {
	t.Parallel()
	lintOutput := `src/x.go:12: bad thing
src/x.go: whole file bad
gone.go:3: line form needs no file
gone.go: path form needs the file
src/x.go:1: 100% wrong, a:b
src/x.go:4-6: a block
just a message
2 problems`
	body := "cat <<'EOF'\n" + lintOutput + "\nEOF\nexit 1"

	t.Run("ci", func(t *testing.T) {
		t.Parallel()
		repo, lintsDir := makeRepo(t), t.TempDir()
		writeLint(t, lintsDir, "a", body)
		got := runIn(t.Context(), t, repo, lintsDir, "run", "--hook", "ci", "a")
		token := stopToken(t, got.stdout)
		want := "== a: fail ==\n::stop-commands::" + token + "\n" + lintOutput + "\n::" + token + "::\n" +
			"::error file=src/x.go,line=12::bad thing\n" +
			"::error file=src/x.go::whole file bad\n" +
			"::error file=gone.go,line=3::line form needs no file\n" +
			"::error file=src/x.go,line=1::100%25 wrong, a:b\n" +
			"::error file=src/x.go,line=4,endLine=6::a block\n" +
			"toolbelt: 0 passed, 1 failed, 0 errored\n"
		if diff := cmp.Diff(want, got.stdout); diff != "" {
			t.Errorf("stdout (-want +got):\n%s", diff)
		}
		if got.code != 1 {
			t.Errorf("exit %d, want 1", got.code)
		}
	})
	t.Run("pre-push adds none", func(t *testing.T) {
		t.Parallel()
		repo, lintsDir := makeRepo(t), t.TempDir()
		writeLint(t, lintsDir, "a", body)
		got := runHook(t.Context(), t, repo, lintsDir, "pre-push", "a")
		if strings.Contains(got.stdout, "::error") {
			t.Errorf("stdout = %q, want no annotations outside ci", got.stdout)
		}
	})
	t.Run("a path escaping the repo gets none", func(t *testing.T) {
		t.Parallel()
		repo, lintsDir := makeRepo(t), t.TempDir()
		writeLint(t, lintsDir, "a", "echo '../outside: msg'; echo '/etc/hostname: msg'; exit 1")
		got := runIn(t.Context(), t, repo, lintsDir, "run", "--hook", "ci", "a")
		if strings.Contains(got.stdout, "::error") {
			t.Errorf("stdout = %q, want no annotations for paths outside the repo", got.stdout)
		}
	})
}

// stopToken returns the token of the first ::stop-commands:: line in out.
func stopToken(t *testing.T, out string) string {
	t.Helper()
	m := regexp.MustCompile(`(?m)^::stop-commands::(.+)$`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no ::stop-commands:: line in %q", out)
	}
	return m[1]
}

func TestRun_runCIStopsCommandsInLintOutput(t *testing.T) {
	t.Parallel()
	// A lint echoes file names from the tree it checks, so its output may hold
	// workflow commands: a forged annotation, and an attempt to stop commands
	// so the toolbelt's own annotations are ignored.
	forged := "::error file=src/x.go,line=9::forged\n::stop-commands::hijack\nsrc/x.go:2: real finding"
	repo, lintsDir := makeRepo(t), t.TempDir()
	writeLint(t, lintsDir, "a", "printf '%s\\n' '::warning::from stderr' >&2\ncat <<'EOF'\n"+forged+"\nEOF\nexit 1")
	writeLint(t, lintsDir, "c", "echo '::error::from c'")

	got := runIn(t.Context(), t, repo, lintsDir, "run", "--hook", "ci", "a", "c")

	token := stopToken(t, got.stdout)
	if !regexp.MustCompile(`^[A-Za-z0-9]{20,}$`).MatchString(token) {
		t.Errorf("token %q, want a long random alphanumeric token", token)
	}
	stop, resume := "::stop-commands::"+token+"\n", "::"+token+"::\n"
	want := "== a: fail ==\n" + stop + forged + "\n::warning::from stderr\n" + resume +
		"::error file=src/x.go,line=2::real finding\n" +
		"== c: pass ==\n" + stop + "::error::from c\n" + resume +
		"toolbelt: 1 passed, 1 failed, 0 errored\n"
	if diff := cmp.Diff(want, got.stdout); diff != "" {
		t.Errorf("stdout (-want +got):\n%s", diff)
	}
	if strings.Contains(got.stderr, "from stderr") {
		t.Errorf("stderr = %q, want the lint's stderr inside the stopped region on stdout", got.stderr)
	}
	if again := stopToken(t, runIn(t.Context(), t, repo, lintsDir, "run", "--hook", "ci", "a").stdout); again == token {
		t.Errorf("two runs used the same token %q", token)
	}
}

func TestRun_runTreeWarnings(t *testing.T) {
	t.Parallel()
	const (
		unstaged = "toolbelt: 2 tracked file(s) have unstaged changes; lints saw the working tree\n"
	)
	tests := []struct {
		name   string
		hook   string
		change func(t *testing.T, repo string)
		want   string
	}{
		{name: "pre-commit clean", hook: "pre-commit", change: func(*testing.T, string) {}},
		{
			name: "pre-commit unstaged edits",
			hook: "pre-commit",
			change: func(t *testing.T, repo string) {
				t.Helper()
				writeFile(t, filepath.Join(repo, "tracked.txt"), "two\n")
				writeFile(t, filepath.Join(repo, "src", "x.go"), "package y\n")
			},
			want: unstaged,
		},
		{
			// The runner's container never has the host path a repo's config
			// may name, and git refuses to diff without it.
			name: "pre-commit unstaged edits with a diff.orderFile the checkout lacks",
			hook: "pre-commit",
			change: func(t *testing.T, repo string) {
				t.Helper()
				gitIn(t, repo, "config", "diff.orderFile", filepath.Join(t.TempDir(), "missing"))
				writeFile(t, filepath.Join(repo, "tracked.txt"), "two\n")
				writeFile(t, filepath.Join(repo, "src", "x.go"), "package y\n")
			},
			want: unstaged,
		},
		{
			name: "pre-commit staged edit and untracked file",
			hook: "pre-commit",
			change: func(t *testing.T, repo string) {
				t.Helper()
				writeFile(t, filepath.Join(repo, "tracked.txt"), "two\n")
				gitIn(t, repo, "add", "tracked.txt")
				writeFile(t, filepath.Join(repo, "new.txt"), "new\n")
			},
		},
		{name: "pre-push clean", hook: "pre-push", change: func(*testing.T, string) {}},
		{
			name: "pre-push untracked file",
			hook: "pre-push",
			change: func(t *testing.T, repo string) {
				t.Helper()
				writeFile(t, filepath.Join(repo, "new.txt"), "new\n")
			},
			want: "toolbelt: 1 file(s) have uncommitted changes; lints saw them\n",
		},
		{
			// git status diffs internally, so a missing diff.orderFile fails
			// it the same way.
			name: "pre-push edit with a diff.orderFile the checkout lacks",
			hook: "pre-push",
			change: func(t *testing.T, repo string) {
				t.Helper()
				gitIn(t, repo, "config", "diff.orderFile", filepath.Join(t.TempDir(), "missing"))
				writeFile(t, filepath.Join(repo, "tracked.txt"), "two\n")
			},
			want: "toolbelt: 1 file(s) have uncommitted changes; lints saw them\n",
		},
		{
			// A rename is one entry, though porcelain -z gives it two paths.
			name: "pre-push rename, edit and untracked file",
			hook: "pre-push",
			change: func(t *testing.T, repo string) {
				t.Helper()
				gitIn(t, repo, "mv", "tracked.txt", "moved.txt")
				writeFile(t, filepath.Join(repo, "src", "x.go"), "package y\n")
				writeFile(t, filepath.Join(repo, "new.txt"), "new\n")
			},
			want: "toolbelt: 3 file(s) have uncommitted changes; lints saw them\n",
		},
		{
			name: "ci dirty tree",
			hook: "ci",
			change: func(t *testing.T, repo string) {
				t.Helper()
				writeFile(t, filepath.Join(repo, "tracked.txt"), "two\n")
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, lintsDir := makeRepo(t), t.TempDir()
			tc.change(t, repo)
			writeLint(t, lintsDir, "a", "exit 0")
			got := runHook(t.Context(), t, repo, lintsDir, tc.hook, "a")
			if got.code != 0 {
				t.Errorf("exit %d, want 0: a warning never changes the status", got.code)
			}
			if got.stderr != tc.want {
				t.Errorf("stderr = %q, want %q", got.stderr, tc.want)
			}
		})
	}
}

func TestRun_runConfigProblems(t *testing.T) {
	t.Parallel()
	repo, lintsDir, markers := makeRepo(t), t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(repo, ".repo-toolbelt.yml"), "nope: 1\n")
	ran := filepath.Join(markers, "ran")
	writeLint(t, lintsDir, "a", `touch '`+ran+`'`)

	got := runIn(t.Context(), t, repo, lintsDir, "run", "--hook", "ci", "a")

	if got.code != 1 {
		t.Errorf("exit %d, want 1", got.code)
	}
	if want := ".repo-toolbelt.yml: unknown lint 'nope'; known: a, b, c\n"; !strings.HasPrefix(got.stdout, want) {
		t.Errorf("stdout = %q, want it to start with %q", got.stdout, want)
	}
	if _, err := os.Stat(ran); err == nil {
		t.Error("a lint ran despite config problems")
	}
}

func TestRun_runUnreadableConfig(t *testing.T) {
	t.Parallel()
	repo, lintsDir := makeRepo(t), t.TempDir()
	// A directory where the file belongs cannot be read as a file.
	if err := os.Mkdir(filepath.Join(repo, ".repo-toolbelt.yml"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeLint(t, lintsDir, "a", "exit 0")
	got := runIn(t.Context(), t, repo, lintsDir, "run", "--hook", "ci", "a")
	if got.code != 2 || !strings.Contains(got.stderr, ".repo-toolbelt.yml") {
		t.Errorf("got %+v, want exit 2 naming the config file", got)
	}
}

func TestRun_runNotAGitRepository(t *testing.T) {
	t.Parallel()
	lintsDir := t.TempDir()
	writeLint(t, lintsDir, "a", "exit 0")
	got := runIn(t.Context(), t, t.TempDir(), lintsDir, "run", "--hook", "ci", "a")
	if got.code != 2 || !strings.Contains(got.stderr, "not a git repository") {
		t.Errorf("got %+v, want exit 2 saying it is not a git repository", got)
	}
}

func TestRun_runCheckoutOwnedByAnotherUser(t *testing.T) {
	// Not parallel: git reads its test hook from the process environment. The
	// hook makes git treat the repository as owned by another user, which a
	// non-root test cannot arrange with chown.
	repo, lintsDir := makeRepo(t), t.TempDir()
	t.Setenv("GIT_TEST_ASSUME_DIFFERENT_OWNER", "1")
	writeLint(t, lintsDir, "a", "exit 0")
	got := runIn(t.Context(), t, repo, lintsDir, "run", "--hook", "ci", "a")
	if got.code != 2 || !strings.Contains(got.stderr, "the checkout must be owned by the user running the container") {
		t.Errorf("got %+v, want exit 2 with the ownership message", got)
	}
}

func TestRun_runUsageErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "no hook", args: []string{"run", "a"}, want: "--hook"},
		{name: "unknown hook", args: []string{"run", "--hook", "post-merge", "a"}, want: `unknown hook "post-merge"`},
		{name: "no lint", args: []string{"run", "--hook", "ci"}, want: "no lint named"},
		{name: "unknown lint", args: []string{"run", "--hook", "ci", "zz"}, want: `unknown lint "zz"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := runIn(t.Context(), t, makeRepo(t), t.TempDir(), tc.args...)
			if got.code != 2 || got.stdout != "" || !strings.Contains(got.stderr, tc.want) {
				t.Errorf("got %+v, want exit 2 with %q on stderr", got, tc.want)
			}
		})
	}
}

func TestRun_runCancelled(t *testing.T) {
	t.Parallel()
	repo, lintsDir := makeRepo(t), t.TempDir()
	writeLint(t, lintsDir, "a", "exit 0")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got := runIn(ctx, t, repo, lintsDir, "run", "--hook", "ci", "a")
	if got.code != 130 {
		t.Errorf("got %+v, want exit 130", got)
	}
}

func TestRun_runWriteError(t *testing.T) {
	t.Parallel()
	repo, lintsDir := makeRepo(t), t.TempDir()
	writeLint(t, lintsDir, "a", "exit 0")
	runner := cli.Runner{Lints: runLints, Dir: repo}
	env := map[string]string{"TOOLBELT_LINTS_DIR": lintsDir}
	err := runner.Run(t.Context(), []string{"run", "--hook", "ci", "a"}, func(k string) string { return env[k] }, strings.NewReader(""), failingWriter{}, &bytes.Buffer{})
	if !errors.Is(err, errWrite) || cli.ExitCode(err) != 1 {
		t.Errorf("err = %v (exit %d), want the write error and exit 1", err, cli.ExitCode(err))
	}
}

func TestRun_runWarningGitFailure(t *testing.T) {
	t.Parallel()
	for _, hook := range []string{"pre-commit", "pre-push"} {
		t.Run(hook, func(t *testing.T) {
			t.Parallel()
			repo, lintsDir := makeRepo(t), t.TempDir()
			// rev-parse never reads the index; diff and status do.
			writeFile(t, filepath.Join(repo, ".git", "index"), "not an index")
			writeLint(t, lintsDir, "a", "exit 0")
			got := runHook(t.Context(), t, repo, lintsDir, hook, "a")
			if got.code != 2 || !strings.Contains(got.stderr, "index file") {
				t.Errorf("got %+v, want exit 2 carrying git's own message", got)
			}
		})
	}
}

func TestRun_runPrePushRunsOncePerRefAndLabelsBlocks(t *testing.T) {
	t.Parallel()
	repo, lintsDir := makeRepo(t), t.TempDir()
	flags, _ := withRemote(t, repo)
	head := strings.TrimSpace(gitOut(t, repo, "rev-parse", "HEAD"))
	zeros := strings.Repeat("0", 40)
	stdin := "refs/heads/one " + head + " refs/heads/one " + zeros + "\n" +
		"refs/heads/two " + head + " refs/heads/two " + zeros + "\n"
	writeLint(t, lintsDir, "a", `printf 'base=%s range=%s\n' "${TOOLBELT_BASE}" "${TOOLBELT_RANGE}"`)
	writeLint(t, lintsDir, "c", "exit 1")

	got := runStdin(t.Context(), t, repo, lintsDir, stdin, append([]string{"run", "--hook", "pre-push"}, append(flags, "a", "c")...)...)

	rng := head + ".." + head
	want := "== a (refs/heads/one): pass ==\nbase=" + head + " range=" + rng + "\n" +
		"== c (refs/heads/one): fail ==\n" +
		"== a (refs/heads/two): pass ==\nbase=" + head + " range=" + rng + "\n" +
		"== c (refs/heads/two): fail ==\n" +
		"toolbelt: 2 passed, 2 failed, 0 errored (2 lints x 2 refs)\n"
	if diff := cmp.Diff(want, got.stdout); diff != "" {
		t.Errorf("stdout (-want +got):\n%s", diff)
	}
	if got.code != 1 {
		t.Errorf("exit %d, want 1", got.code)
	}
}

func TestRun_runPrePushExitIsTheWorstAcrossRefs(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		first, second string
	}{
		"error then pass":   {first: "exit 2", second: "exit 0"},
		"pass then error":   {first: "exit 0", second: "exit 2"},
		"finding then pass": {first: "exit 1", second: "exit 0"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo, lintsDir := makeRepo(t), t.TempDir()
			flags, _ := withRemote(t, repo)
			first := strings.TrimSpace(gitOut(t, repo, "rev-parse", "HEAD"))
			writeFile(t, filepath.Join(repo, "more.txt"), "more\n")
			gitIn(t, repo, "add", "--all")
			gitIn(t, repo, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "--quiet", "--message=more")
			second := strings.TrimSpace(gitOut(t, repo, "rev-parse", "HEAD"))
			zeros := strings.Repeat("0", 40)
			stdin := "refs/heads/one " + first + " refs/heads/one " + zeros + "\n" +
				"refs/heads/two " + second + " refs/heads/two " + zeros + "\n"
			// The second ref's range ends at the second commit.
			writeLint(t, lintsDir, "a", `case "${TOOLBELT_RANGE}" in *..`+second+`) `+tc.second+`;; *) `+tc.first+`;; esac`)
			got := runStdin(t.Context(), t, repo, lintsDir, stdin, append([]string{"run", "--hook", "pre-push"}, append(flags, "a")...)...)
			want := 0
			for _, c := range []string{tc.first, tc.second} {
				want = max(want, map[string]int{"exit 0": 0, "exit 1": 1, "exit 2": 2}[c])
			}
			if got.code != want {
				t.Errorf("exit %d, want %d", got.code, want)
			}
		})
	}
}

func TestRun_runPrePushWithOnlyDeletionsRunsNothing(t *testing.T) {
	t.Parallel()
	repo, lintsDir := makeRepo(t), t.TempDir()
	flags, _ := withRemote(t, repo)
	head := strings.TrimSpace(gitOut(t, repo, "rev-parse", "HEAD"))
	marker := filepath.Join(t.TempDir(), "ran")
	writeLint(t, lintsDir, "a", `touch '`+marker+`'`)
	stdin := "(delete) " + strings.Repeat("0", 40) + " refs/heads/gone " + head + "\n"

	got := runStdin(t.Context(), t, repo, lintsDir, stdin, append([]string{"run", "--hook", "pre-push"}, append(flags, "a")...)...)

	if _, err := os.Stat(marker); err == nil {
		t.Error("a lint ran for a push that only deletes")
	}
	if got.code != 0 || got.stdout != "toolbelt: 0 passed, 0 failed, 0 errored\n" {
		t.Errorf("got %+v, want exit 0 and an empty summary", got)
	}
}

func TestRun_runPrePushWithoutRemoteIsUsageError(t *testing.T) {
	t.Parallel()
	repo, lintsDir := makeRepo(t), t.TempDir()
	writeLint(t, lintsDir, "a", "exit 0")
	got := runIn(t.Context(), t, repo, lintsDir, "run", "--hook", "pre-push", "a")
	if got.code != 2 || !strings.Contains(got.stderr, "--remote") {
		t.Errorf("got %+v, want exit 2 naming --remote", got)
	}
}

func TestRun_runPrePushNewBranchWithoutRemoteHeadNamesTheFix(t *testing.T) {
	t.Parallel()
	repo, lintsDir := makeRepo(t), t.TempDir()
	flags, stdin := withRemote(t, repo)
	gitIn(t, repo, "remote", "set-head", "origin", "--delete")
	writeLint(t, lintsDir, "a", "exit 0")
	got := runStdin(t.Context(), t, repo, lintsDir, stdin, append([]string{"run", "--hook", "pre-push"}, append(flags, "a")...)...)
	if got.code != 2 || !strings.Contains(got.stderr, "git remote set-head origin --auto") {
		t.Errorf("got %+v, want exit 2 naming the set-head fix", got)
	}
}

func TestRun_runCIResolvesBaseAndRange(t *testing.T) {
	t.Parallel()
	repo, lintsDir := makeRepo(t), t.TempDir()
	head := strings.TrimSpace(gitOut(t, repo, "rev-parse", "HEAD"))
	writeLint(t, lintsDir, "a", `printf 'base=%s range=%s\n' "${TOOLBELT_BASE}" "${TOOLBELT_RANGE}"`)
	got := runIn(t.Context(), t, repo, lintsDir, "run", "--hook", "ci", "--base", head, "--head", head, "a")
	if !strings.Contains(got.stdout, "base="+head+" range="+head+".."+head+"\n") {
		t.Errorf("got %+v, want the resolved base and range", got)
	}
}

func TestRun_runCIBaseWithoutHeadIsUsageError(t *testing.T) {
	t.Parallel()
	repo, lintsDir := makeRepo(t), t.TempDir()
	writeLint(t, lintsDir, "a", "exit 0")
	got := runIn(t.Context(), t, repo, lintsDir, "run", "--hook", "ci", "--base", "abc", "a")
	if got.code != 2 || !strings.Contains(got.stderr, "--head") {
		t.Errorf("got %+v, want exit 2 naming --head", got)
	}
}

var errBoom = errors.New("boom")

// writerFailingAfter accepts its first ok Write calls and fails every later one
// with errBoom.
type writerFailingAfter struct{ ok int }

func (w *writerFailingAfter) Write(p []byte) (int, error) {
	if w.ok == 0 {
		return 0, errBoom
	}
	w.ok--
	return len(p), nil
}

// readerFailingAfter returns data, then errBoom.
type readerFailingAfter struct{ data io.Reader }

func (r readerFailingAfter) Read(p []byte) (int, error) {
	n, err := r.data.Read(p)
	if errors.Is(err, io.EOF) {
		return n, errBoom
	}
	return n, err
}

// runStreams runs toolbelt in repo with the given streams, mirroring main's
// error handling, and returns the error Run returned.
func runStreams(t *testing.T, repo, lintsDir string, stdin io.Reader, stdout, stderr io.Writer, args ...string) error {
	t.Helper()
	env := map[string]string{"TOOLBELT_LINTS_DIR": lintsDir}
	runner := cli.Runner{Lints: runLints, Version: "1.2.3", Dir: repo}
	return runner.Run(t.Context(), args, func(k string) string { return env[k] }, stdin, stdout, stderr)
}

func TestRun_runStreamFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		args     []string
		dirty    bool
		stdout   int // Write calls stdout accepts before failing
		stderr   int // Write calls stderr accepts before failing
		wantCode int
	}{
		// ci writes the whole block in one stdout Write, then the summary.
		{name: "summary line", args: []string{"run", "--hook", "ci", "a"}, stdout: 1, stderr: 100, wantCode: 1},
		// A modified tracked file under pre-commit makes run warn about unstaged changes on stderr.
		{name: "warning line", args: []string{"run", "--hook", "pre-commit", "a"}, dirty: true, stdout: 100, stderr: 0, wantCode: 1},
		{name: "result block on stdout", args: []string{"run", "--hook", "pre-commit", "a"}, stdout: 0, stderr: 100, wantCode: 1},
		{name: "result block on stderr", args: []string{"run", "--hook", "pre-commit", "a"}, stdout: 100, stderr: 0, wantCode: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, lintsDir := makeRepo(t), t.TempDir()
			writeLint(t, lintsDir, "a", "exit 0")
			if tt.dirty {
				writeFile(t, filepath.Join(repo, "tracked.txt"), "two\n")
			}
			err := runStreams(t, repo, lintsDir, strings.NewReader(""), &writerFailingAfter{ok: tt.stdout}, &writerFailingAfter{ok: tt.stderr}, tt.args...)
			if !errors.Is(err, errBoom) || cli.ExitCode(err) != tt.wantCode {
				t.Errorf("err = %v (exit %d), want errBoom and exit %d", err, cli.ExitCode(err), tt.wantCode)
			}
		})
	}
}

func TestRun_runPrePushInputReadFailure(t *testing.T) {
	t.Parallel()
	repo, lintsDir := makeRepo(t), t.TempDir()
	writeLint(t, lintsDir, "a", "exit 0")
	flags, line := withRemote(t, repo)
	args := append([]string{"run", "--hook", "pre-push"}, flags...)
	args = append(args, "a")
	var stdout, stderr bytes.Buffer
	err := runStreams(t, repo, lintsDir, readerFailingAfter{data: strings.NewReader(line)}, &stdout, &stderr, args...)
	if !errors.Is(err, errBoom) || cli.ExitCode(err) != 2 || !strings.Contains(err.Error(), "read pre-push input") {
		t.Errorf("err = %v (exit %d), want errBoom, exit 2 and \"read pre-push input\"", err, cli.ExitCode(err))
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want no lint run after the failed read", stdout.String())
	}
}

// runFormatters are the formatters the derived-lint tests select from: x
// takes a setting and claims extensionless xsh scripts, y is slow and claims
// Buildfile by name, whatever its shebang.
var runFormatters = []manifest.Formatter{
	{Name: "x", Language: "bash", Speed: "quick", Description: "formats x", Include: []string{"*.x"}, Shebangs: []string{"xsh"}, Settings: []string{"width"}},
	{Name: "y", Language: "go", Speed: "slow", Description: "formats y", Include: []string{"*.y", "Buildfile"}},
}

// formatterRunner is a Runner over runLints plus runFormatters and their
// derived lints, run from dir.
func formatterRunner(t *testing.T, dir string) cli.Runner {
	t.Helper()
	merged, err := manifest.Merge(runLints, runFormatters)
	if err != nil {
		t.Fatal(err)
	}
	return cli.Runner{Lints: merged, Formatters: runFormatters, Version: "1.2.3", Dir: dir}
}

// listFiles is a fixture formatter's body: it prints its mode and each file
// it was given, then runs exit.
func listFiles(exit string) string {
	return `printf 'mode=%s\n' "${TOOLBELT_MODE}"
while IFS= read -r -d '' f; do printf 'file=[%s]\n' "${f}"; done < "${TOOLBELT_FILES}"
` + exit
}

func commitAll(t *testing.T, repo string) {
	t.Helper()
	gitIn(t, repo, "add", "--all")
	gitIn(t, repo, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "--quiet", "--message=more")
}

// formatterEnv points the runner at fixture lints and formatters.
func formatterEnv(lintsDir, formattersDir string) map[string]string {
	return map[string]string{"TOOLBELT_LINTS_DIR": lintsDir, "TOOLBELT_FORMATTERS_DIR": formattersDir}
}

func TestRun_runDerivedLintRunsItsFormatterInCheckMode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		exit, verdict, summary string
		wantCode               int
	}{
		{exit: "exit 0", verdict: "pass", summary: "1 passed, 0 failed, 0 errored", wantCode: 0},
		{exit: "exit 1", verdict: "fail", summary: "0 passed, 1 failed, 0 errored", wantCode: 1},
		{exit: "exit 2", verdict: "error", summary: "0 passed, 0 failed, 1 errored", wantCode: 2},
	}
	for _, tt := range tests {
		t.Run(tt.verdict, func(t *testing.T) {
			t.Parallel()
			repo, lintsDir, formattersDir := makeRepo(t), t.TempDir(), t.TempDir()
			for _, name := range []string{"a.x", "dir with space/b c.x", "-lead.x", "new\nline.x", "notes.txt", "c.y"} {
				writeFile(t, filepath.Join(repo, name), "content\n")
			}
			writeFile(t, filepath.Join(repo, "run"), "#!/usr/bin/env xsh\n")
			commitAll(t, repo)
			writeLint(t, formattersDir, "x", listFiles(tt.exit))

			got := runWith(t.Context(), t, formatterRunner(t, repo), formatterEnv(lintsDir, formattersDir), "",
				"run", "--hook", "pre-commit", "check-x-formatted")

			want := "== check-x-formatted: " + tt.verdict + " ==\nmode=check\n" +
				"file=[-lead.x]\nfile=[a.x]\nfile=[dir with space/b c.x]\nfile=[new\nline.x]\nfile=[run]\n" +
				"toolbelt: " + tt.summary + "\n"
			if diff := cmp.Diff(want, got.stdout); diff != "" {
				t.Errorf("stdout (-want +got):\n%s", diff)
			}
			if got.code != tt.wantCode {
				t.Errorf("exit %d, want %d (stderr %q)", got.code, tt.wantCode, got.stderr)
			}
		})
	}
}

func TestRun_runDerivedLintAppliesItsSection(t *testing.T) {
	t.Parallel()
	repo, lintsDir, formattersDir := makeRepo(t), t.TempDir(), t.TempDir()
	for _, name := range []string{"a.x", "skip/z.x", "skip/deep/w.x", "keep/skip.x"} {
		writeFile(t, filepath.Join(repo, name), "content\n")
	}
	writeFile(t, filepath.Join(repo, ".repo-toolbelt.yml"), "check-x-formatted:\n  exclude:\n    - skip\n  settings:\n    width: 80\n")
	commitAll(t, repo)
	writeLint(t, formattersDir, "x", `printf 'section=%s\n' "$(cat "${TOOLBELT_SECTION}")"`+"\n"+listFiles("exit 0"))

	got := runWith(t.Context(), t, formatterRunner(t, repo), formatterEnv(lintsDir, formattersDir), "",
		"run", "--hook", "pre-commit", "check-x-formatted")

	want := "== check-x-formatted: pass ==\n" +
		`section={"exclude":["skip"],"settings":{"width":80}}` + "\nmode=check\n" +
		"file=[a.x]\nfile=[keep/skip.x]\n" +
		"toolbelt: 1 passed, 0 failed, 0 errored\n"
	if diff := cmp.Diff(want, got.stdout); diff != "" {
		t.Errorf("stdout (-want +got):\n%s", diff)
	}
	if got.code != 0 {
		t.Errorf("exit %d, want 0 (stderr %q)", got.code, got.stderr)
	}
}

// badXSection is a check-x-formatted section with two problems.
const badXSection = "check-x-formatted:\n  settings:\n    width: 0\n    bogus: 1\n"

// badXBlock is the body of the failing block a run prints for badXSection.
const badXBlock = ".repo-toolbelt.yml: check-x-formatted: settings: unknown setting 'bogus'; allowed: width\n" +
	".repo-toolbelt.yml: check-x-formatted: settings.width: must be a positive integer\n" +
	"2 problem(s)\n"

func TestRun_runBadSectionFailsOnlyItsLint(t *testing.T) {
	t.Parallel()
	repo, lintsDir, formattersDir, markers := makeRepo(t), t.TempDir(), t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(repo, ".repo-toolbelt.yml"), badXSection)
	writeFile(t, filepath.Join(repo, "a.x"), "content\n")
	commitAll(t, repo)
	ran := filepath.Join(markers, "x-ran")
	writeLint(t, lintsDir, "a", "echo a-out")
	writeLint(t, lintsDir, "c", "echo c-out")
	writeLint(t, formattersDir, "x", `touch '`+ran+`'`)

	got := runWith(t.Context(), t, formatterRunner(t, repo), formatterEnv(lintsDir, formattersDir), "",
		"run", "--hook", "pre-commit", "a", "check-x-formatted", "c")

	want := "== a: pass ==\na-out\n" +
		"== check-x-formatted: fail ==\n" + badXBlock +
		"== c: pass ==\nc-out\n" +
		"toolbelt: 2 passed, 1 failed, 0 errored\n"
	if diff := cmp.Diff(want, got.stdout); diff != "" {
		t.Errorf("stdout (-want +got):\n%s", diff)
	}
	if got.code != 1 {
		t.Errorf("exit %d, want 1 (stderr %q)", got.code, got.stderr)
	}
	if _, err := os.Stat(ran); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the formatter ran despite its bad section (stat: %v)", err)
	}
}

func TestRun_runPrePushBadSectionFailsOncePerRef(t *testing.T) {
	t.Parallel()
	repo, lintsDir, formattersDir := makeRepo(t), t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(repo, ".repo-toolbelt.yml"), badXSection)
	commitAll(t, repo)
	flags, _ := withRemote(t, repo)
	head := strings.TrimSpace(gitOut(t, repo, "rev-parse", "HEAD"))
	zeros := strings.Repeat("0", 40)
	stdin := "refs/heads/one " + head + " refs/heads/one " + zeros + "\n" +
		"refs/heads/two " + head + " refs/heads/two " + zeros + "\n"
	writeLint(t, lintsDir, "a", "exit 0")

	got := runWith(t.Context(), t, formatterRunner(t, repo), formatterEnv(lintsDir, formattersDir), stdin,
		append(append([]string{"run", "--hook", "pre-push"}, flags...), "check-x-formatted", "a")...)

	want := "== check-x-formatted (refs/heads/one): fail ==\n" + badXBlock +
		"== a (refs/heads/one): pass ==\n" +
		"== check-x-formatted (refs/heads/two): fail ==\n" + badXBlock +
		"== a (refs/heads/two): pass ==\n" +
		"toolbelt: 2 passed, 2 failed, 0 errored (2 lints x 2 refs)\n"
	if diff := cmp.Diff(want, got.stdout); diff != "" {
		t.Errorf("stdout (-want +got):\n%s", diff)
	}
	if got.code != 1 {
		t.Errorf("exit %d, want 1 (stderr %q)", got.code, got.stderr)
	}
}

func TestRun_runValidatesOnlyNamedFormatterSections(t *testing.T) {
	t.Parallel()
	repo, lintsDir, formattersDir := makeRepo(t), t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(repo, ".repo-toolbelt.yml"), badXSection+"check-y-formatted:\n  exclude: nope\n")
	commitAll(t, repo)
	writeLint(t, lintsDir, "a", "exit 0")

	got := runWith(t.Context(), t, formatterRunner(t, repo), formatterEnv(lintsDir, formattersDir), "",
		"run", "--hook", "pre-commit", "a")
	if want := "== a: pass ==\ntoolbelt: 1 passed, 0 failed, 0 errored\n"; got.code != 0 || got.stdout != want {
		t.Errorf("got %+v, want exit 0 and stdout %q", got, want)
	}
}

func TestRun_runSelectsFilesForEveryFormatter(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads a file with no read permission")
	}
	repo, lintsDir, formattersDir := makeRepo(t), t.TempDir(), t.TempDir()
	// Only x's shebangs make selection read an extensionless file, so a run
	// that names only y still reads it.
	script := filepath.Join(repo, "script")
	writeFile(t, script, "#!/usr/bin/env xsh\n")
	writeFile(t, filepath.Join(repo, "c.y"), "content\n")
	commitAll(t, repo)
	if err := os.Chmod(script, 0o000); err != nil {
		t.Fatal(err)
	}
	writeLint(t, lintsDir, "a", "exit 0")
	writeLint(t, formattersDir, "y", "exit 0")
	runner, env := formatterRunner(t, repo), formatterEnv(lintsDir, formattersDir)

	got := runWith(t.Context(), t, runner, env, "", "run", "--hook", "pre-commit", "check-y-formatted")
	if got.code != 2 || got.stdout != "" || !strings.Contains(got.stderr, "script") {
		t.Errorf("run check-y-formatted: got %+v, want exit 2 naming the unreadable file and no lint run", got)
	}

	// A run that names no derived lint selects nothing.
	got = runWith(t.Context(), t, runner, env, "", "run", "--hook", "pre-commit", "a")
	if want := "== a: pass ==\ntoolbelt: 1 passed, 0 failed, 0 errored\n"; got.code != 0 || got.stdout != want {
		t.Errorf("run a: got %+v, want exit 0 and stdout %q", got, want)
	}
}

func TestRun_runFileOwnershipIgnoresWhichFormattersRun(t *testing.T) {
	t.Parallel()
	// Buildfile's shebang names x's interpreter, but y's include claims it,
	// and an include wins over a shebang.
	tests := []struct {
		name   string
		config string
		args   []string
		want   string
	}{
		{
			name: "both named",
			args: []string{"check-x-formatted", "check-y-formatted"},
			want: "== check-x-formatted: pass ==\nmode=check\nfile=[a.x]\n" +
				"== check-y-formatted: pass ==\nmode=check\nfile=[Buildfile]\n" +
				"toolbelt: 2 passed, 0 failed, 0 errored\n",
		},
		{
			name: "owner not named",
			args: []string{"check-x-formatted"},
			want: "== check-x-formatted: pass ==\nmode=check\nfile=[a.x]\ntoolbelt: 1 passed, 0 failed, 0 errored\n",
		},
		{
			name:   "owner's section bad",
			config: "check-y-formatted:\n  settings:\n    width: 1\n",
			args:   []string{"check-x-formatted", "check-y-formatted"},
			want: "== check-x-formatted: pass ==\nmode=check\nfile=[a.x]\n" +
				"== check-y-formatted: fail ==\n" +
				".repo-toolbelt.yml: check-y-formatted: settings: check-y-formatted takes no settings\n1 problem(s)\n" +
				"toolbelt: 1 passed, 1 failed, 0 errored\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, lintsDir, formattersDir := makeRepo(t), t.TempDir(), t.TempDir()
			writeFile(t, filepath.Join(repo, "Buildfile"), "#!/usr/bin/env xsh\n")
			writeFile(t, filepath.Join(repo, "a.x"), "content\n")
			if tt.config != "" {
				writeFile(t, filepath.Join(repo, ".repo-toolbelt.yml"), tt.config)
			}
			commitAll(t, repo)
			writeLint(t, formattersDir, "x", listFiles("exit 0"))
			writeLint(t, formattersDir, "y", listFiles("exit 0"))

			got := runWith(t.Context(), t, formatterRunner(t, repo), formatterEnv(lintsDir, formattersDir), "",
				append([]string{"run", "--hook", "pre-commit"}, tt.args...)...)

			if diff := cmp.Diff(tt.want, got.stdout); diff != "" {
				t.Errorf("stdout (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRun_runDefaultFormattersDir(t *testing.T) {
	t.Parallel()
	repo, lintsDir := makeRepo(t), t.TempDir()
	got := runWith(t.Context(), t, formatterRunner(t, repo), map[string]string{"TOOLBELT_LINTS_DIR": lintsDir}, "",
		"run", "--hook", "pre-commit", "check-x-formatted")
	if got.code != 2 || !strings.Contains(got.stderr, "/opt/toolbelt/formatters/x/x") {
		t.Errorf("got %+v, want exit 2 naming the default formatter path", got)
	}
}

func TestRun_runDerivedLintWithoutItsFormatter(t *testing.T) {
	t.Parallel()
	repo, lintsDir, formattersDir := makeRepo(t), t.TempDir(), t.TempDir()
	runner := formatterRunner(t, repo)
	runner.Formatters = runFormatters[1:]
	got := runWith(t.Context(), t, runner, formatterEnv(lintsDir, formattersDir), "",
		"run", "--hook", "pre-commit", "check-x-formatted")
	if got.code != 2 || got.stdout != "" || !strings.Contains(got.stderr, `"x"`) {
		t.Errorf("got %+v, want exit 2 naming the missing formatter and no lint run", got)
	}
}
