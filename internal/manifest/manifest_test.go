package manifest_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/rvenutolo/repo-toolbelt/internal/manifest"
)

const validTOML = `
[[lint]]
name = "check-alpha"
language = "bash"
speed = "quick"
description = "First lint."

[[lint]]
name = "check-beta"
language = "go"
speed = "slow"
description = "Second lint."
`

func TestParseValid(t *testing.T) {
	t.Parallel()

	got, err := manifest.Parse([]byte(validTOML))
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
	want := []manifest.Lint{
		{Name: "check-alpha", Language: "bash", Speed: "quick", Description: "First lint."},
		{Name: "check-beta", Language: "go", Speed: "slow", Description: "Second lint."},
	}
	if len(got) != len(want) {
		t.Fatalf("Parse() returned %d lints, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("lint %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func assertRejects(t *testing.T, input, wantErr string) {
	t.Helper()

	got, err := manifest.Parse([]byte(input))
	if err == nil {
		t.Fatalf("Parse() = %+v, nil error; want an error", got)
	}
	if !strings.Contains(err.Error(), wantErr) {
		t.Errorf("Parse() error = %q, want it to mention %q", err, wantErr)
	}
}

func TestParseRejectsUnknownSpeed(t *testing.T) {
	t.Parallel()

	assertRejects(t,
		"[[lint]]\nname = \"a\"\nlanguage = \"bash\"\nspeed = \"medium\"\ndescription = \"d\"\n",
		"speed")
}

func TestParseRejectsUnknownLanguage(t *testing.T) {
	t.Parallel()

	assertRejects(t,
		"[[lint]]\nname = \"a\"\nlanguage = \"python\"\nspeed = \"quick\"\ndescription = \"d\"\n",
		"language")
}

func TestParseRejectsDuplicateName(t *testing.T) {
	t.Parallel()

	assertRejects(t,
		"[[lint]]\nname = \"a\"\nlanguage = \"bash\"\nspeed = \"quick\"\ndescription = \"d\"\n"+
			"[[lint]]\nname = \"a\"\nlanguage = \"go\"\nspeed = \"slow\"\ndescription = \"e\"\n",
		"duplicate")
}

func TestParseRejectsUnknownKey(t *testing.T) {
	t.Parallel()

	assertRejects(t,
		"[[lint]]\nname = \"a\"\nlanguage = \"bash\"\nspeed = \"quick\"\ndescription = \"d\"\nextra = 1\n",
		"extra")
}

func TestParseRejectsMissingName(t *testing.T) {
	t.Parallel()

	assertRejects(t,
		"[[lint]]\nlanguage = \"bash\"\nspeed = \"quick\"\ndescription = \"d\"\n",
		"name")
}

func TestParseRejectsMissingLanguage(t *testing.T) {
	t.Parallel()

	assertRejects(t,
		"[[lint]]\nname = \"a\"\nspeed = \"quick\"\ndescription = \"d\"\n",
		"language")
}

func TestParseRejectsMissingSpeed(t *testing.T) {
	t.Parallel()

	assertRejects(t,
		"[[lint]]\nname = \"a\"\nlanguage = \"bash\"\ndescription = \"d\"\n",
		"speed")
}

func TestParseRejectsMissingDescription(t *testing.T) {
	t.Parallel()

	assertRejects(t,
		"[[lint]]\nname = \"a\"\nlanguage = \"bash\"\nspeed = \"quick\"\n",
		"description")
}

func TestParseRejectsMalformedTOML(t *testing.T) {
	t.Parallel()

	assertRejects(t,
		"[[lint\n",
		"")
}

func TestParseEmpty(t *testing.T) {
	t.Parallel()

	got, err := manifest.Parse([]byte("# no lints yet\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("Parse() = %+v, want an empty slice", got)
	}
}

func TestLoadEmbedded(t *testing.T) {
	t.Parallel()

	// Load panics on a malformed embed, so reaching the end is the assertion.
	_ = manifest.Load()
}

func TestParseRejectsFormatterKey(t *testing.T) {
	t.Parallel()

	assertRejects(t,
		"[[lint]]\nname = \"a\"\nlanguage = \"bash\"\nspeed = \"quick\"\ndescription = \"d\"\nformatter = \"x\"\n",
		"formatter")
}

const validFormatterTOML = `
[[formatter]]
name = "shell"
language = "bash"
speed = "quick"
description = "Shell files are formatted."
include = ["*.sh", ".envrc"]
shebangs = ["bash", "sh"]
settings = ["indent"]

[[formatter]]
name = "go"
language = "go"
speed = "slow"
description = "Go files are formatted."
include = ["*.go"]
`

func TestParseFormattersValid(t *testing.T) {
	t.Parallel()

	got, err := manifest.ParseFormatters([]byte(validFormatterTOML))
	if err != nil {
		t.Fatalf("ParseFormatters() error = %v, want nil", err)
	}
	want := []manifest.Formatter{
		{
			Name: "shell", Language: "bash", Speed: "quick", Description: "Shell files are formatted.",
			Include: []string{"*.sh", ".envrc"}, Shebangs: []string{"bash", "sh"}, Settings: []string{"indent"},
		},
		{
			Name: "go", Language: "go", Speed: "slow", Description: "Go files are formatted.",
			Include: []string{"*.go"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseFormatters() = %+v, want %+v", got, want)
	}
}

func TestParseFormattersEmpty(t *testing.T) {
	t.Parallel()

	got, err := manifest.ParseFormatters([]byte("# no formatters yet\n"))
	if err != nil {
		t.Fatalf("ParseFormatters() error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("ParseFormatters() = %+v, want an empty slice", got)
	}
}

// formatterTOML builds one valid [[formatter]] table for name. A key in
// overrides replaces that key's line; an empty override drops the line. The
// optional keys shebangs and extra are written only when overridden.
func formatterTOML(name string, overrides map[string]string) string {
	keys := []string{"name", "language", "speed", "description", "include", "shebangs", "extra"}
	lines := map[string]string{
		"name":        "name = \"" + name + "\"",
		"language":    "language = \"bash\"",
		"speed":       "speed = \"quick\"",
		"description": "description = \"d\"",
		"include":     "include = [\"*." + name + "\"]",
	}
	var out strings.Builder
	out.WriteString("[[formatter]]\n")
	for _, key := range keys {
		line := lines[key]
		if override, ok := overrides[key]; ok {
			line = override
		}
		if line != "" {
			out.WriteString(line + "\n")
		}
	}
	return out.String()
}

func TestParseFormattersRejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"malformed TOML", "[[formatter\n", ""},
		{"unknown key", formatterTOML("a", map[string]string{"extra": "extra = 1"}), "extra"},
		{"lint table", "[[lint]]\nname = \"a\"\n", "lint"},
		{"missing name", formatterTOML("a", map[string]string{"name": ""}), "name"},
		{"missing language", formatterTOML("a", map[string]string{"language": ""}), "language"},
		{"missing speed", formatterTOML("a", map[string]string{"speed": ""}), "speed"},
		{"missing description", formatterTOML("a", map[string]string{"description": ""}), "description"},
		{"empty include", formatterTOML("a", map[string]string{"include": "include = []"}), "include"},
		{"missing include", formatterTOML("a", map[string]string{"include": ""}), "include"},
		{"uppercase name", formatterTOML("a", map[string]string{"name": "name = \"Shell\""}), "name"},
		{"name starting with a digit", formatterTOML("a", map[string]string{"name": "name = \"1a\""}), "name"},
		{"name with a hyphen", formatterTOML("a", map[string]string{"name": "name = \"a-b\""}), "name"},
		{"unknown language", formatterTOML("a", map[string]string{"language": "language = \"python\""}), "language"},
		{"unknown speed", formatterTOML("a", map[string]string{"speed": "speed = \"medium\""}), "speed"},
		{
			"duplicate name",
			formatterTOML("a", nil) + formatterTOML("a", map[string]string{"include": "include = [\"*.other\"]"}),
			"duplicate",
		},
		{
			"shared include glob",
			formatterTOML("a", map[string]string{"include": "include = [\"*.x\"]"}) +
				formatterTOML("b", map[string]string{"include": "include = [\"*.y\", \"*.x\"]"}),
			"*.x",
		},
		{
			"shared shebang",
			formatterTOML("a", map[string]string{"shebangs": "shebangs = [\"bash\"]"}) +
				formatterTOML("b", map[string]string{"shebangs": "shebangs = [\"bash\"]"}),
			"bash",
		},
		{"invalid include pattern", formatterTOML("a", map[string]string{"include": "include = [\"[\"]"}), "include"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := manifest.ParseFormatters([]byte(tt.input))
			if err == nil {
				t.Fatalf("ParseFormatters() = %+v, nil error; want an error", got)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("ParseFormatters() error = %q, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}

func TestLintName(t *testing.T) {
	t.Parallel()

	if got, want := (manifest.Formatter{Name: "shell"}).LintName(), "check-shell-formatted"; got != want {
		t.Errorf("LintName() = %q, want %q", got, want)
	}
}

func TestMerge(t *testing.T) {
	t.Parallel()

	lints := []manifest.Lint{
		{Name: "check-alpha", Language: "bash", Speed: "quick", Description: "First lint."},
	}
	formatters := []manifest.Formatter{
		{Name: "shell", Language: "bash", Speed: "slow", Description: "Shell files are formatted.", Include: []string{"*.sh"}},
		{Name: "go", Language: "go", Speed: "quick", Description: "Go files are formatted.", Include: []string{"*.go"}},
	}

	got, err := manifest.Merge(lints, formatters)
	if err != nil {
		t.Fatalf("Merge() error = %v, want nil", err)
	}
	want := []manifest.Lint{
		{Name: "check-alpha", Language: "bash", Speed: "quick", Description: "First lint."},
		{Name: "check-shell-formatted", Language: "bash", Speed: "slow", Description: "Shell files are formatted.", Formatter: "shell"},
		{Name: "check-go-formatted", Language: "go", Speed: "quick", Description: "Go files are formatted.", Formatter: "go"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Merge() = %+v, want %+v", got, want)
	}
}

func TestMergeWithoutFormattersReturnsTheLints(t *testing.T) {
	t.Parallel()

	lints := []manifest.Lint{{Name: "check-alpha", Language: "bash", Speed: "quick", Description: "First lint."}}

	got, err := manifest.Merge(lints, nil)
	if err != nil {
		t.Fatalf("Merge() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, lints) {
		t.Errorf("Merge() = %+v, want %+v", got, lints)
	}
}

func TestMergeRejectsDerivedNameCollision(t *testing.T) {
	t.Parallel()

	lints := []manifest.Lint{{Name: "check-shell-formatted", Language: "bash", Speed: "quick", Description: "d"}}
	formatters := []manifest.Formatter{
		{Name: "shell", Language: "bash", Speed: "quick", Description: "d", Include: []string{"*.sh"}},
	}

	got, err := manifest.Merge(lints, formatters)
	if err == nil {
		t.Fatalf("Merge() = %+v, nil error; want an error", got)
	}
	if !strings.Contains(err.Error(), "check-shell-formatted") {
		t.Errorf("Merge() error = %q, want it to mention the colliding name", err)
	}
}

func TestLoadAll(t *testing.T) {
	t.Parallel()

	lints, formatters := manifest.LoadAll()
	if len(formatters) != 0 {
		t.Errorf("LoadAll() formatters = %+v, want none while formatters.toml has no entries", formatters)
	}
	if want := manifest.Load(); !reflect.DeepEqual(lints, want) {
		t.Errorf("LoadAll() lints = %+v, want Load()'s %+v", lints, want)
	}
}
