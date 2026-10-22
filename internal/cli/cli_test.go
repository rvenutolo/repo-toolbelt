package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/cli"
	"github.com/rvenutolo/repo-toolbelt/internal/manifest"
)

var lints = []manifest.Lint{
	{Name: "a", Language: "bash", Speed: "quick", Description: "first lint"},
	{Name: "b", Language: "go", Speed: "slow", Description: "second lint"},
	{Name: "c", Language: "go", Speed: "quick", Description: "third lint"},
}

type result struct {
	code           int
	stdout, stderr string
}

func run(t *testing.T, env map[string]string, args ...string) result {
	t.Helper()
	var stdout, stderr bytes.Buffer
	runner := cli.Runner{Lints: lints}
	code := runner.Run(args, func(k string) string { return env[k] }, strings.NewReader(""), &stdout, &stderr)
	return result{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func TestRun_usage(t *testing.T) {
	t.Parallel()
	t.Run("no arguments", func(t *testing.T) {
		t.Parallel()
		got := run(t, nil)
		if got.code != 2 || got.stdout != "" || !strings.Contains(got.stderr, "Usage:") {
			t.Errorf("got %+v, want exit 2, usage on stderr only", got)
		}
	})
	for _, flag := range []string{"--help", "-h", "help"} {
		t.Run(flag, func(t *testing.T) {
			t.Parallel()
			got := run(t, nil, flag)
			if got.code != 0 || got.stderr != "" || !strings.Contains(got.stdout, "Usage:") {
				t.Errorf("got %+v, want exit 0, usage on stdout only", got)
			}
			for _, sub := range []string{"plan", "list"} {
				if !strings.Contains(got.stdout, sub) {
					t.Errorf("usage does not mention %q:\n%s", sub, got.stdout)
				}
			}
		})
	}
	t.Run("unknown subcommand", func(t *testing.T) {
		t.Parallel()
		got := run(t, nil, "frobnicate")
		if got.code != 2 || !strings.Contains(got.stderr, `unknown subcommand "frobnicate"`) {
			t.Errorf("got %+v, want exit 2 naming the subcommand", got)
		}
	})
}

func TestRun_list(t *testing.T) {
	t.Parallel()
	got := run(t, nil, "list")
	want := "a\tquick\tfirst lint\nb\tslow\tsecond lint\nc\tquick\tthird lint\n"
	if got.code != 0 || got.stdout != want {
		t.Errorf("got %+v, want exit 0 and %q", got, want)
	}
	if got := run(t, nil, "list", "extra"); got.code != 2 {
		t.Errorf("list with an argument: exit %d, want 2", got.code)
	}
	if got := run(t, nil, "list", "--help"); got.code != 0 || !strings.Contains(got.stdout, "Usage:") {
		t.Errorf("list --help: got %+v", got)
	}
}

func TestRun_plan(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		env  map[string]string
		want string
	}{
		{name: "split default", args: []string{"plan", "a", "b", "c"}, want: `{"groups":[["a","c"],["b"]]}`},
		{name: "split explicit", args: []string{"plan", "--mode", "split", "a", "b", "c"}, want: `{"groups":[["a","c"],["b"]]}`},
		{name: "split only slow", args: []string{"plan", "b"}, want: `{"groups":[["b"]]}`},
		{name: "split only quick", args: []string{"plan", "c", "a"}, want: `{"groups":[["c","a"]]}`},
		{name: "parallel", args: []string{"plan", "--mode=parallel", "c", "b", "a"}, want: `{"groups":[["c","b","a"]]}`},
		{name: "matrix", args: []string{"plan", "--mode", "matrix", "c", "b", "a"}, want: `{"groups":[["c"],["b"],["a"]]}`},
		{
			name: "lints from the environment",
			args: []string{"plan", "--mode", "matrix"},
			env:  map[string]string{"TOOLBELT_LINTS": "a\n# skip\nb\n"},
			want: `{"groups":[["a"],["b"]]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := run(t, tt.env, tt.args...)
			if got.code != 0 || got.stderr != "" {
				t.Fatalf("got %+v, want exit 0", got)
			}
			if diff := cmp.Diff(tt.want+"\n", got.stdout); diff != "" {
				t.Errorf("stdout (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRun_planUsageErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		env  map[string]string
		want string
	}{
		{name: "unknown mode", args: []string{"plan", "--mode", "fast", "a"}, want: `unknown mode "fast"`},
		{name: "no lint", args: []string{"plan"}, want: "no lint named"},
		{name: "unknown lint", args: []string{"plan", "x"}, want: `unknown lint "x"`},
		{name: "repeated lint", args: []string{"plan", "a", "a"}, want: `lint "a" named twice`},
		{
			name: "both sources",
			args: []string{"plan", "a"},
			env:  map[string]string{"TOOLBELT_LINTS": "b"},
			want: "lints given both as arguments and in TOOLBELT_LINTS",
		},
		{name: "unknown flag", args: []string{"plan", "--bogus", "a"}, want: "bogus"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := run(t, tt.env, tt.args...)
			if got.code != 2 || got.stdout != "" || !strings.Contains(got.stderr, tt.want) {
				t.Errorf("got %+v, want exit 2 with %q on stderr", got, tt.want)
			}
		})
	}
}

func TestRun_planHelp(t *testing.T) {
	t.Parallel()
	got := run(t, nil, "plan", "--help")
	if got.code != 0 || got.stderr != "" || !strings.Contains(got.stdout, "--mode") {
		t.Errorf("got %+v, want exit 0 with flags on stdout", got)
	}
}
