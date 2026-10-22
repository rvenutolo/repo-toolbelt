package lintkit_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
)

// harness runs lintkit.Run with buffers and a map-backed environment that
// holds a section file, unless env overrides TOOLBELT_SECTION.
type harness struct {
	stdout, stderr bytes.Buffer
	dir            string
	env            map[string]string
}

func newHarness(t *testing.T, section string) *harness {
	t.Helper()
	dir := t.TempDir()
	sectionFile := filepath.Join(t.TempDir(), "section.json")
	if err := os.WriteFile(sectionFile, []byte(section), 0o600); err != nil {
		t.Fatal(err)
	}
	return &harness{dir: dir, env: map[string]string{"TOOLBELT_SECTION": sectionFile}}
}

func (h *harness) run(t *testing.T, check lintkit.Check, args ...string) int {
	t.Helper()
	return lintkit.Run(t.Context(), "demo", "widget", check, lintkit.Env{
		Args:   args,
		Getenv: func(name string) string { return h.env[name] },
		Dir:    h.dir,
		Stdout: &h.stdout,
		Stderr: &h.stderr,
	})
}

func returning(findings ...lintkit.Finding) lintkit.Check {
	return func(context.Context, lintkit.Context) ([]lintkit.Finding, error) { return findings, nil }
}

func TestRun_noFindingsPasses(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "null")
	if got := h.run(t, returning()); got != 0 {
		t.Errorf("exit = %d, want 0", got)
	}
	if h.stdout.Len() != 0 || h.stderr.Len() != 0 {
		t.Errorf("stdout %q, stderr %q, want both empty", h.stdout.String(), h.stderr.String())
	}
}

func TestRun_findingsSortedThenCounted(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "null")
	got := h.run(t, returning(
		lintkit.Finding{Path: "b.txt", Line: 2, Message: "b two"},
		lintkit.Finding{Path: "a.txt", Line: 10, Message: "a ten"},
		lintkit.Finding{Path: "b.txt", Line: 0, Message: "b whole file"},
		lintkit.Finding{Path: "a.txt", Line: 9, Message: "a nine"},
		lintkit.Finding{Message: "no location"},
	))
	if got != 1 {
		t.Errorf("exit = %d, want 1", got)
	}
	want := "no location\n" +
		"a.txt:9: a nine\n" +
		"a.txt:10: a ten\n" +
		"b.txt: b whole file\n" +
		"b.txt:2: b two\n" +
		"5 widget(s)\n"
	if diff := cmp.Diff(want, h.stdout.String()); diff != "" {
		t.Errorf("stdout mismatch (-want +got):\n%s", diff)
	}
	if h.stderr.Len() != 0 {
		t.Errorf("stderr = %q, want empty", h.stderr.String())
	}
}

func TestRun_equalLocationsKeepCheckOrder(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "null")
	h.run(t, returning(
		lintkit.Finding{Path: "a", Line: 1, Message: "second"},
		lintkit.Finding{Path: "a", Line: 1, Message: "first"},
	))
	want := "a:1: second\na:1: first\n2 widget(s)\n"
	if diff := cmp.Diff(want, h.stdout.String()); diff != "" {
		t.Errorf("stdout mismatch (-want +got):\n%s", diff)
	}
}

func TestRun_lineRange(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "null")
	h.run(t, returning(
		lintkit.Finding{Path: "a", Line: 5, EndLine: 7, Message: "a range"},
		lintkit.Finding{Path: "a", Line: 2, EndLine: 2, Message: "a one-line range"},
		lintkit.Finding{Path: "a", Line: 3, EndLine: 1, Message: "an end before the start"},
	))
	want := "a:2: a one-line range\na:3: an end before the start\na:5-7: a range\n3 widget(s)\n"
	if diff := cmp.Diff(want, h.stdout.String()); diff != "" {
		t.Errorf("stdout mismatch (-want +got):\n%s", diff)
	}
}

func TestRun_escapesUserValuesOntoOneLine(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "null")
	h.run(t, returning(lintkit.Finding{Path: "a\nb\tc", Line: 3, Message: "x\ry\x1bz\x7f\x00 \xffé"}))
	want := "a\\nb\\tc:3: x\\ry?z?? ?é\n1 widget(s)\n"
	if diff := cmp.Diff(want, h.stdout.String()); diff != "" {
		t.Errorf("stdout mismatch (-want +got):\n%s", diff)
	}
}

func TestRun_sectionError(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "null")
	h.run(t, returning(
		lintkit.SectionError("exempt[0]", "reason is missing or empty"),
		lintkit.SectionError("", "unknown key 'x'"),
	))
	want := ".repo-toolbelt.yml: demo: exempt[0]: reason is missing or empty\n" +
		".repo-toolbelt.yml: demo: unknown key 'x'\n" +
		"2 widget(s)\n"
	if diff := cmp.Diff(want, h.stdout.String()); diff != "" {
		t.Errorf("stdout mismatch (-want +got):\n%s", diff)
	}
}

func TestRun_checkErrorIsSetupFailure(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "null")
	got := h.run(t, func(context.Context, lintkit.Context) ([]lintkit.Finding, error) {
		return []lintkit.Finding{{Message: "dropped"}}, errors.New("git ls-files failed")
	})
	if got != 2 {
		t.Errorf("exit = %d, want 2", got)
	}
	if h.stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", h.stdout.String())
	}
	if want := "demo: git ls-files failed\n"; h.stderr.String() != want {
		t.Errorf("stderr = %q, want %q", h.stderr.String(), want)
	}
}

func TestRun_passesContext(t *testing.T) {
	t.Parallel()
	h := newHarness(t, `{"exempt":[]}`)
	h.env["TOOLBELT_HOOK"] = "pre-push"
	h.env["TOOLBELT_BASE"] = "abc"
	h.env["TOOLBELT_RANGE"] = "abc..def"
	var got lintkit.Context
	if code := h.run(t, func(_ context.Context, lc lintkit.Context) ([]lintkit.Finding, error) {
		got = lc
		return nil, nil
	}); code != 0 {
		t.Fatalf("exit = %d, want 0; stderr %q", code, h.stderr.String())
	}
	want := lintkit.Context{
		Root:    h.dir,
		Section: []byte(`{"exempt":[]}`),
		Hook:    "pre-push",
		Base:    "abc",
		Range:   "abc..def",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("context mismatch (-want +got):\n%s", diff)
	}
}

func TestRun_help(t *testing.T) {
	t.Parallel()
	for _, arg := range []string{"--help", "-h"} {
		t.Run(arg, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, "null")
			called := false
			got := h.run(t, func(context.Context, lintkit.Context) ([]lintkit.Finding, error) {
				called = true
				return nil, nil
			}, arg)
			if got != 0 {
				t.Errorf("exit = %d, want 0", got)
			}
			if called {
				t.Error("check ran, want only usage")
			}
			if !strings.HasPrefix(h.stdout.String(), "Usage: demo\n") {
				t.Errorf("stdout = %q, want a usage starting %q", h.stdout.String(), "Usage: demo\n")
			}
			for _, want := range []string{"TOOLBELT_SECTION", "TOOLBELT_HOOK", "TOOLBELT_BASE", "TOOLBELT_RANGE", "Exit codes"} {
				if !strings.Contains(h.stdout.String(), want) {
					t.Errorf("usage does not mention %q:\n%s", want, h.stdout.String())
				}
			}
		})
	}
}

func TestRun_setupFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		args       []string
		section    func(h *harness)
		wantStderr string
	}{
		{name: "unknown flag", args: []string{"--bogus"}, wantStderr: "demo: "},
		{name: "positional argument", args: []string{"extra"}, wantStderr: "demo: unexpected argument \"extra\"; usage: demo [--help]\n"},
		{
			name:       "section unset",
			section:    func(h *harness) { delete(h.env, "TOOLBELT_SECTION") },
			wantStderr: "demo: TOOLBELT_SECTION is not set\n",
		},
		{
			name:       "section file missing",
			section:    func(h *harness) { h.env["TOOLBELT_SECTION"] = filepath.Join(h.dir, "absent.json") },
			wantStderr: "demo: read config section: ",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t, "null")
			if tc.section != nil {
				tc.section(h)
			}
			called := false
			got := h.run(t, func(context.Context, lintkit.Context) ([]lintkit.Finding, error) {
				called = true
				return nil, nil
			}, tc.args...)
			if got != 2 {
				t.Errorf("exit = %d, want 2", got)
			}
			if called {
				t.Error("check ran, want a setup failure first")
			}
			if h.stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", h.stdout.String())
			}
			if !strings.HasPrefix(h.stderr.String(), tc.wantStderr) {
				t.Errorf("stderr = %q, want prefix %q", h.stderr.String(), tc.wantStderr)
			}
		})
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("closed pipe") }

func TestRun_unwritableStdoutIsSetupFailure(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "null")
	var stderr bytes.Buffer
	got := lintkit.Run(t.Context(), "demo", "widget", returning(lintkit.Finding{Message: "x"}), lintkit.Env{
		Getenv: func(name string) string { return h.env[name] },
		Dir:    h.dir,
		Stdout: failingWriter{},
		Stderr: &stderr,
	})
	if got != 2 {
		t.Errorf("exit = %d, want 2", got)
	}
	if want := "demo: write findings: closed pipe\n"; stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestRun_unwritableUsageIsSetupFailure(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	got := lintkit.Run(t.Context(), "demo", "widget", returning(), lintkit.Env{
		Args:   []string{"--help"},
		Getenv: func(string) string { return "" },
		Stdout: failingWriter{},
		Stderr: &stderr,
	})
	if got != 2 {
		t.Errorf("exit = %d, want 2", got)
	}
	if want := "demo: write usage: closed pipe\n"; stderr.String() != want {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestRun_passesTheContextThrough(t *testing.T) {
	t.Parallel()
	h := newHarness(t, "null")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	code := lintkit.Run(ctx, "demo", "widget", func(ctx context.Context, _ lintkit.Context) ([]lintkit.Finding, error) {
		return nil, ctx.Err()
	}, lintkit.Env{
		Getenv: func(name string) string { return h.env[name] },
		Dir:    h.dir,
		Stdout: &h.stdout,
		Stderr: &h.stderr,
	})
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if want := "demo: context canceled\n"; h.stderr.String() != want {
		t.Errorf("stderr = %q, want %q", h.stderr.String(), want)
	}
}
