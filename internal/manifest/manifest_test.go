package manifest_test

import (
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
