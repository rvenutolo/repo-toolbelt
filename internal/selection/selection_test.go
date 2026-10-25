package selection_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/manifest"
	"github.com/rvenutolo/repo-toolbelt/internal/selection"
)

var known = []manifest.Lint{
	{Name: "a", Language: "bash", Speed: "quick", Description: "first"},
	{Name: "b", Language: "go", Speed: "slow", Description: "second"},
	{Name: "c", Language: "go", Speed: "quick", Description: "third"},
}

func names(lints []manifest.Lint) []string {
	out := make([]string, len(lints))
	for i, l := range lints {
		out[i] = l.Name
	}
	return out
}

func TestParse_selects(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		env  string
		want []string
	}{
		{name: "arguments keep the given order", args: []string{"c", "a"}, want: []string{"c", "a"}},
		{name: "environment lines", env: "b\na\n", want: []string{"b", "a"}},
		{name: "blank lines and surrounding whitespace", env: "\n  a  \n\n\t b\t\n", want: []string{"a", "b"}},
		{name: "whole-line comment", env: "# note\na\n", want: []string{"a"}},
		{name: "trailing comment", env: "a # why\nc#x\n", want: []string{"a", "c"}},
		{name: "carriage returns", env: "a\r\nb\r\n", want: []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := selection.Parse(tt.args, tt.env, known)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if diff := cmp.Diff(tt.want, names(got)); diff != "" {
				t.Errorf("names (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParse_returnsManifestEntries(t *testing.T) {
	t.Parallel()
	got, err := selection.Parse([]string{"b"}, "", known)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if diff := cmp.Diff([]manifest.Lint{known[1]}, got); diff != "" {
		t.Errorf("lints (-want +got):\n%s", diff)
	}
}

func TestParse_errors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		env  string
		want string
	}{
		{name: "nothing named", want: "no lint named"},
		{name: "only comments and blanks", env: "# x\n\n  \n", want: "no lint named"},
		{name: "both sources", args: []string{"a"}, env: "b", want: "lints given both as arguments and in TOOLBELT_LINTS"},
		{name: "unknown argument", args: []string{"x"}, want: `unknown lint "x"`},
		{name: "unknown in environment", env: "a\nx\n", want: `unknown lint "x"`},
		{name: "repeated argument", args: []string{"a", "b", "a"}, want: `lint "a" named twice`},
		{name: "repeated in environment", env: "a\na # again", want: `lint "a" named twice`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := selection.Parse(tt.args, tt.env, known)
			if err == nil {
				t.Fatalf("Parse = %v, want error", got)
			}
			if err.Error() != tt.want {
				t.Errorf("error = %q, want %q", err, tt.want)
			}
		})
	}
}

var knownFormatters = []manifest.Formatter{
	{Name: "go", Language: "bash", Speed: "quick", Description: "formats go", Include: []string{"*.go"}},
	{Name: "shell", Language: "bash", Speed: "quick", Description: "formats shell", Include: []string{"*.sh"}},
}

func TestParseFormatters_selects(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		env  string
		want []manifest.Formatter
	}{
		{name: "arguments keep the given order", args: []string{"shell", "go"}, want: []manifest.Formatter{knownFormatters[1], knownFormatters[0]}},
		{name: "environment lines with comments", env: "# note\n  go # why\n\nshell\r\n", want: knownFormatters},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := selection.ParseFormatters(tt.args, tt.env, knownFormatters)
			if err != nil {
				t.Fatalf("ParseFormatters: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("formatters (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseFormatters_errors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args []string
		env  string
		want string
	}{
		{name: "nothing named", want: "no formatter named"},
		{name: "only comments and blanks", env: "# x\n\n", want: "no formatter named"},
		{name: "both sources", args: []string{"go"}, env: "shell", want: "formatters given both as arguments and in TOOLBELT_FORMATTERS"},
		{name: "unknown argument", args: []string{"x"}, want: `unknown formatter "x"`},
		{name: "a lint name is not a formatter name", args: []string{"check-go-formatted"}, want: `unknown formatter "check-go-formatted"`},
		{name: "repeated in environment", env: "go\ngo", want: `formatter "go" named twice`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := selection.ParseFormatters(tt.args, tt.env, knownFormatters)
			if err == nil {
				t.Fatalf("ParseFormatters = %v, want error", got)
			}
			if err.Error() != tt.want {
				t.Errorf("error = %q, want %q", err, tt.want)
			}
		})
	}
}
