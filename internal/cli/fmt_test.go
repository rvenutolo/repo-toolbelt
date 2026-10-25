package cli_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/cli"
)

// Fixture formatter bodies. rewriteFiles changes every file it is given,
// rewriteSame writes each file's own bytes back, so it writes without
// changing anything.
const (
	rewriteFiles = `while IFS= read -r -d '' f; do printf 'formatted\n' > "${f}"; done < "${TOOLBELT_FILES}"`
	rewriteSame  = `while IFS= read -r -d '' f; do data="$(cat "${f}")"; printf '%s\n' "${data}" > "${f}"; done < "${TOOLBELT_FILES}"`
)

// fmtRepo is a repository holding a.x, b.x and c.y, each "content\n", plus
// config as its config file when config is not empty.
func fmtRepo(t *testing.T, config string) string {
	t.Helper()
	repo := makeRepo(t)
	for _, name := range []string{"a.x", "b.x", "c.y"} {
		writeFile(t, filepath.Join(repo, name), "content\n")
	}
	if config != "" {
		writeFile(t, filepath.Join(repo, ".repo-toolbelt.yml"), config)
	}
	commitAll(t, repo)
	return repo
}

// runFmt runs toolbelt fmt in repo with fixture formatters from formattersDir.
func runFmt(t *testing.T, repo, formattersDir string, args ...string) result {
	t.Helper()
	return runWith(t.Context(), t, formatterRunner(t, repo), map[string]string{"TOOLBELT_FORMATTERS_DIR": formattersDir}, "",
		append([]string{"fmt"}, args...)...)
}

func TestRun_fmtVerdictsAndExitStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		// bodyY, when set, is a second formatter y, run after x.
		bodyY        string
		failOnChange bool
		wantStdout   string
		wantCode     int
		wantA        string
	}{
		{
			name: "a rewrite exits 0",
			body: rewriteFiles,
			wantStdout: "== x: formatted ==\na.x: formatted\nb.x: formatted\n" +
				"toolbelt: 2 file(s) formatted, 0 failed, 0 errored; review and stage them\n",
			wantCode: 0,
			wantA:    "formatted\n",
		},
		{
			name:         "a rewrite with --fail-on-change exits 1",
			body:         rewriteFiles,
			failOnChange: true,
			wantStdout: "== x: formatted ==\na.x: formatted\nb.x: formatted\n" +
				"toolbelt: 2 file(s) formatted, 0 failed, 0 errored; review and stage them\n",
			wantCode: 1,
			wantA:    "formatted\n",
		},
		{
			name:         "identical bytes written with --fail-on-change exit 0",
			body:         rewriteSame,
			failOnChange: true,
			wantStdout:   "== x: pass ==\ntoolbelt: 0 file(s) formatted, 0 failed, 0 errored\n",
			wantCode:     0,
			wantA:        "content\n",
		},
		{
			name:       "a finding exits 1",
			body:       "echo 'a.x: cannot be parsed: boom'; exit 1",
			wantStdout: "== x: fail ==\na.x: cannot be parsed: boom\ntoolbelt: 0 file(s) formatted, 1 failed, 0 errored\n",
			wantCode:   1,
			wantA:      "content\n",
		},
		{
			name:       "an error exits 2",
			body:       "exit 2",
			wantStdout: "== x: error ==\ntoolbelt: 0 file(s) formatted, 0 failed, 1 errored\n",
			wantCode:   2,
			wantA:      "content\n",
		},
		{
			name:       "an error outranks a finding",
			body:       "exit 2",
			bodyY:      "exit 1",
			wantStdout: "== x: error ==\n== y: fail ==\ntoolbelt: 0 file(s) formatted, 1 failed, 1 errored\n",
			wantCode:   2,
			wantA:      "content\n",
		},
		{
			name: "a rewrite is listed even when the formatter fails",
			body: `printf 'formatted\n' > b.x; echo 'a.x: cannot be parsed: boom'; exit 1`,
			wantStdout: "== x: fail ==\nb.x: formatted\na.x: cannot be parsed: boom\n" +
				"toolbelt: 1 file(s) formatted, 1 failed, 0 errored; review and stage them\n",
			wantCode: 1,
			wantA:    "content\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, formattersDir := fmtRepo(t, ""), t.TempDir()
			writeLint(t, formattersDir, "x", tt.body)
			args := []string{"x"}
			if tt.bodyY != "" {
				writeLint(t, formattersDir, "y", tt.bodyY)
				args = append(args, "y")
			}
			if tt.failOnChange {
				args = append([]string{"--fail-on-change"}, args...)
			}

			got := runFmt(t, repo, formattersDir, args...)

			if diff := cmp.Diff(tt.wantStdout, got.stdout); diff != "" {
				t.Errorf("stdout (-want +got):\n%s", diff)
			}
			if got.code != tt.wantCode {
				t.Errorf("exit %d, want %d (stderr %q)", got.code, tt.wantCode, got.stderr)
			}
			if a := readFile(t, filepath.Join(repo, "a.x")); a != tt.wantA {
				t.Errorf("a.x = %q, want %q", a, tt.wantA)
			}
		})
	}
}

func TestRun_fmtCallingConvention(t *testing.T) {
	t.Parallel()
	repo, formattersDir := fmtRepo(t, "check-x-formatted:\n  exclude:\n    - b.x\n  settings:\n    width: 80\n"), t.TempDir()
	writeLint(t, formattersDir, "x", `printf 'mode=%s hook=[%s] base=[%s] range=[%s] dir=%s\n' "${TOOLBELT_MODE}" "${TOOLBELT_HOOK-unset}" "${TOOLBELT_BASE-unset}" "${TOOLBELT_RANGE-unset}" "${PWD}"
printf 'section=%s\n' "$(cat "${TOOLBELT_SECTION}")"
while IFS= read -r -d '' f; do printf 'file=[%s]\n' "${f}"; done < "${TOOLBELT_FILES}"
printf 'x-diag\n' >&2`)

	got := runFmt(t, repo, formattersDir, "x")

	want := "== x: pass ==\n" +
		"mode=write hook=[] base=[] range=[] dir=" + repo + "\n" +
		`section={"exclude":["b.x"],"settings":{"width":80}}` + "\n" +
		"file=[a.x]\n" +
		"toolbelt: 0 file(s) formatted, 0 failed, 0 errored\n"
	if diff := cmp.Diff(want, got.stdout); diff != "" {
		t.Errorf("stdout (-want +got):\n%s", diff)
	}
	if got.stderr != "x-diag\n" || got.code != 0 {
		t.Errorf("got exit %d and stderr %q, want exit 0 and the formatter's stderr", got.code, got.stderr)
	}
}

func TestRun_fmtRunsOnlyTheNamedFormattersInNamedOrder(t *testing.T) {
	t.Parallel()
	repo, formattersDir := fmtRepo(t, ""), t.TempDir()
	writeLint(t, formattersDir, "x", rewriteFiles)
	writeLint(t, formattersDir, "y", `echo y-out; `+rewriteFiles)

	got := runFmt(t, repo, formattersDir, "y", "x")
	want := "== y: formatted ==\nc.y: formatted\ny-out\n" +
		"== x: formatted ==\na.x: formatted\nb.x: formatted\n" +
		"toolbelt: 3 file(s) formatted, 0 failed, 0 errored; review and stage them\n"
	if diff := cmp.Diff(want, got.stdout); diff != "" {
		t.Errorf("fmt y x: stdout (-want +got):\n%s", diff)
	}

	repo = fmtRepo(t, "")
	got = runFmt(t, repo, formattersDir, "y")
	if want := "== y: formatted ==\nc.y: formatted\ny-out\ntoolbelt: 1 file(s) formatted, 0 failed, 0 errored; review and stage them\n"; got.stdout != want {
		t.Errorf("fmt y: stdout = %q, want %q", got.stdout, want)
	}
	if a := readFile(t, filepath.Join(repo, "a.x")); a != "content\n" {
		t.Errorf("fmt y rewrote a.x, an unnamed formatter's file, to %q", a)
	}
}

func TestRun_fmtFormattersFromEnvironment(t *testing.T) {
	t.Parallel()
	repo, formattersDir := fmtRepo(t, ""), t.TempDir()
	writeLint(t, formattersDir, "x", "echo x-out")
	writeLint(t, formattersDir, "y", "echo y-out")
	env := map[string]string{"TOOLBELT_FORMATTERS_DIR": formattersDir, "TOOLBELT_FORMATTERS": "# both\ny\nx # last\n"}

	got := runWith(t.Context(), t, formatterRunner(t, repo), env, "", "fmt")

	want := "== y: pass ==\ny-out\n== x: pass ==\nx-out\ntoolbelt: 0 file(s) formatted, 0 failed, 0 errored\n"
	if diff := cmp.Diff(want, got.stdout); diff != "" {
		t.Errorf("stdout (-want +got):\n%s", diff)
	}
}

func TestRun_fmtBadSectionFailsOnlyItsFormatter(t *testing.T) {
	t.Parallel()
	repo, formattersDir, markers := fmtRepo(t, badXSection), t.TempDir(), t.TempDir()
	ran := filepath.Join(markers, "x-ran")
	writeLint(t, formattersDir, "x", `touch '`+ran+`'`)
	writeLint(t, formattersDir, "y", rewriteFiles)

	got := runFmt(t, repo, formattersDir, "x", "y")

	want := "== x: fail ==\n" + badXBlock +
		"== y: formatted ==\nc.y: formatted\n" +
		"toolbelt: 1 file(s) formatted, 1 failed, 0 errored; review and stage them\n"
	if diff := cmp.Diff(want, got.stdout); diff != "" {
		t.Errorf("stdout (-want +got):\n%s", diff)
	}
	if got.code != 1 {
		t.Errorf("exit %d, want 1 (stderr %q)", got.code, got.stderr)
	}
	if _, err := os.Stat(ran); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("x ran despite its bad section (stat: %v)", err)
	}
}

func TestRun_fmtListsAFileGoneAfterTheRunAsFormatted(t *testing.T) {
	t.Parallel()
	repo, formattersDir := fmtRepo(t, ""), t.TempDir()
	writeLint(t, formattersDir, "x", `mv b.x b.moved`)

	got := runFmt(t, repo, formattersDir, "x")

	if want := "== x: formatted ==\nb.x: formatted\ntoolbelt: 1 file(s) formatted, 0 failed, 0 errored; review and stage them\n"; got.stdout != want {
		t.Errorf("stdout = %q, want %q", got.stdout, want)
	}
}

func TestRun_fmtListsAFileWhoseParentBecameAFileAsFormatted(t *testing.T) {
	t.Parallel()
	repo, formattersDir := makeRepo(t), t.TempDir()
	writeFile(t, filepath.Join(repo, "d", "a.x"), "content\n")
	commitAll(t, repo)
	writeLint(t, formattersDir, "x", `mv d d.moved && printf 'file\n' > d`)

	got := runFmt(t, repo, formattersDir, "x")

	if want := "== x: formatted ==\nd/a.x: formatted\ntoolbelt: 1 file(s) formatted, 0 failed, 0 errored; review and stage them\n"; got.stdout != want || got.code != 0 {
		t.Errorf("got %+v, want exit 0 and stdout %q", got, want)
	}
}

func TestRun_fmtAFileUnreadableAfterTheRunIsASetupFailure(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads a file with no read permission")
	}
	repo, formattersDir := fmtRepo(t, ""), t.TempDir()
	writeLint(t, formattersDir, "x", `chmod 000 b.x`)

	got := runFmt(t, repo, formattersDir, "x")

	if got.code != 2 || got.stdout != "" || !strings.Contains(got.stderr, "hash b.x after formatting") {
		t.Errorf("got %+v, want exit 2 naming b.x and nothing on stdout", got)
	}
}

func TestRun_fmtUsageErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		env  string
		want string
	}{
		{name: "none named", want: "no formatter named"},
		{name: "unknown", args: []string{"x", "zz"}, want: `unknown formatter "zz"`},
		{name: "a lint name", args: []string{"check-x-formatted"}, want: `unknown formatter "check-x-formatted"`},
		{name: "duplicate", args: []string{"x", "y", "x"}, want: `formatter "x" named twice`},
		{name: "both sources", args: []string{"x"}, env: "y", want: "formatters given both as arguments and in TOOLBELT_FORMATTERS"},
		{name: "unknown flag", args: []string{"--hook", "pre-commit", "x"}, want: "unknown flag: --hook"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, formattersDir, markers := fmtRepo(t, ""), t.TempDir(), t.TempDir()
			ran := filepath.Join(markers, "ran")
			writeLint(t, formattersDir, "x", `touch '`+ran+`'`)
			env := map[string]string{"TOOLBELT_FORMATTERS_DIR": formattersDir, "TOOLBELT_FORMATTERS": tt.env}

			got := runWith(t.Context(), t, formatterRunner(t, repo), env, "", append([]string{"fmt"}, tt.args...)...)

			if got.code != 2 || got.stdout != "" || !strings.Contains(got.stderr, tt.want) {
				t.Errorf("got %+v, want exit 2 with %q on stderr and nothing on stdout", got, tt.want)
			}
			if _, err := os.Stat(ran); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("a formatter ran despite the usage error (stat: %v)", err)
			}
		})
	}
}

func TestRun_fmtRefusesAReadOnlyRoot(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root may write to a directory with no write permission")
	}
	repo, formattersDir, markers := fmtRepo(t, ""), t.TempDir(), t.TempDir()
	ran := filepath.Join(markers, "ran")
	writeLint(t, formattersDir, "x", `touch '`+ran+`'`)
	if err := os.Chmod(repo, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(repo, 0o755); err != nil {
			t.Error(err)
		}
	})

	got := runFmt(t, repo, formattersDir, "y", "x")

	if got.code != 2 || got.stdout != "" {
		t.Errorf("got %+v, want exit 2 and nothing on stdout", got)
	}
	for _, want := range []string{
		repo,
		`--volume "${root}:${root}" --volume "${common}:${common}:ro" --workdir "${root}"`,
		"ghcr.io/rvenutolo/repo-toolbelt:<tag>@sha256:<digest> fmt y x\n",
	} {
		if !strings.Contains(got.stderr, want) {
			t.Errorf("stderr = %q, want it to hold %q", got.stderr, want)
		}
	}
	if _, err := os.Stat(ran); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a formatter ran in a read-only root (stat: %v)", err)
	}
}

func TestRun_fmtSetupFailures(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads a file with no read permission")
	}
	tests := []struct {
		name       string
		unreadable string
		content    string
	}{
		// Selection reads an extensionless file's shebang to find its owner.
		{name: "selection cannot read a candidate", unreadable: "script", content: "#!/usr/bin/env xsh\n"},
		{name: "a selected file cannot be hashed", unreadable: "a.x", content: "content\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, formattersDir, markers := makeRepo(t), t.TempDir(), t.TempDir()
			path := filepath.Join(repo, tt.unreadable)
			writeFile(t, path, tt.content)
			commitAll(t, repo)
			if err := os.Chmod(path, 0o000); err != nil {
				t.Fatal(err)
			}
			ran := filepath.Join(markers, "ran")
			writeLint(t, formattersDir, "x", `touch '`+ran+`'`)

			got := runFmt(t, repo, formattersDir, "x")

			if got.code != 2 || got.stdout != "" || !strings.Contains(got.stderr, tt.unreadable) {
				t.Errorf("got %+v, want exit 2 naming %s and nothing on stdout", got, tt.unreadable)
			}
			if _, err := os.Stat(ran); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("a formatter ran despite the setup failure (stat: %v)", err)
			}
		})
	}
}

func TestRun_fmtConfigProblems(t *testing.T) {
	t.Parallel()
	repo, formattersDir, markers := fmtRepo(t, "nope: 1\n"), t.TempDir(), t.TempDir()
	ran := filepath.Join(markers, "ran")
	writeLint(t, formattersDir, "x", `touch '`+ran+`'`)

	got := runFmt(t, repo, formattersDir, "x")

	if got.code != 1 || !strings.HasPrefix(got.stdout, ".repo-toolbelt.yml: unknown lint 'nope'") {
		t.Errorf("got %+v, want exit 1 and the config problem on stdout", got)
	}
	if _, err := os.Stat(ran); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a formatter ran despite config problems (stat: %v)", err)
	}
}

func TestRun_fmtNotAGitRepository(t *testing.T) {
	t.Parallel()
	got := runFmt(t, t.TempDir(), t.TempDir(), "x")
	if got.code != 2 || !strings.Contains(got.stderr, "not a git repository") {
		t.Errorf("got %+v, want exit 2 saying it is not a git repository", got)
	}
}

func TestRun_fmtFormatterWithoutItsLint(t *testing.T) {
	t.Parallel()
	repo := fmtRepo(t, "")
	runner := cli.Runner{Lints: runLints, Formatters: runFormatters, Dir: repo}
	got := runWith(t.Context(), t, runner, map[string]string{"TOOLBELT_FORMATTERS_DIR": t.TempDir()}, "", "fmt", "x")
	if got.code != 2 || got.stdout != "" || !strings.Contains(got.stderr, `"check-x-formatted"`) {
		t.Errorf("got %+v, want exit 2 naming the missing lint", got)
	}
}

func TestRun_fmtDefaultFormattersDir(t *testing.T) {
	t.Parallel()
	repo := fmtRepo(t, "")
	got := runWith(t.Context(), t, formatterRunner(t, repo), nil, "", "fmt", "x")
	if got.code != 2 || !strings.Contains(got.stderr, "/opt/toolbelt/formatters/x/x") {
		t.Errorf("got %+v, want exit 2 naming the default formatter path", got)
	}
}

func TestRun_fmtCancelled(t *testing.T) {
	t.Parallel()
	repo := fmtRepo(t, "")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got := runWith(ctx, t, formatterRunner(t, repo), map[string]string{"TOOLBELT_FORMATTERS_DIR": t.TempDir()}, "", "fmt", "x")
	if got.code != 130 {
		t.Errorf("got %+v, want exit 130", got)
	}
}

func TestRun_fmtStreamFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		stdout int // Write calls stdout accepts before failing
		stderr int // Write calls stderr accepts before failing
	}{
		{name: "block on stdout", stdout: 0, stderr: 100},
		{name: "block on stderr", stdout: 100, stderr: 0},
		{name: "summary line", stdout: 1, stderr: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, formattersDir := fmtRepo(t, ""), t.TempDir()
			writeLint(t, formattersDir, "x", "echo x-diag >&2")
			env := map[string]string{"TOOLBELT_FORMATTERS_DIR": formattersDir}
			err := formatterRunner(t, repo).Run(t.Context(), []string{"fmt", "x"}, func(k string) string { return env[k] },
				strings.NewReader(""), &writerFailingAfter{ok: tt.stdout}, &writerFailingAfter{ok: tt.stderr})
			if !errors.Is(err, errBoom) || cli.ExitCode(err) != 1 {
				t.Errorf("err = %v (exit %d), want errBoom and exit 1", err, cli.ExitCode(err))
			}
		})
	}
}

func TestRun_fmtHelp(t *testing.T) {
	t.Parallel()
	got := runWith(t.Context(), t, formatterRunner(t, t.TempDir()), nil, "", "fmt", "--help")
	if got.code != 0 {
		t.Errorf("exit %d, want 0", got.code)
	}
	for _, want := range []string{
		"--fail-on-change",
		"TOOLBELT_FORMATTERS",
		"Exit codes: 0",
		"1 a formatter reported a finding",
		"2 a formatter errored",
		"a runtime error (a failed write of the output)",
		"130 interrupted",
		"pass (exit 0, nothing changed)",
		"formatted (exit 0, files changed)",
		"fail (exit 1)",
		"error (exit 2)",
	} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("help does not hold %q:\n%s", want, got.stdout)
		}
	}
}
