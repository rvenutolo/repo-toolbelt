package issuerefs_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/issuerefs"
	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
)

func TestParseSection_valid(t *testing.T) {
	t.Parallel()
	for _, section := range []string{
		"null",
		"{}",
		`{"exempt":[]}`,
		`{"exempt":[{"path":"CHANGELOG.md","reason":"release tooling writes the links"}]}`,
		`{"exempt":[{"path":"t.md","match":"^\\| P-","reason":"probe ids"}]}`,
	} {
		t.Run(section, func(t *testing.T) {
			t.Parallel()
			exemptions, problems, err := issuerefs.ParseSection([]byte(section))
			if err != nil {
				t.Fatalf("ParseSection(%s) error = %v", section, err)
			}
			if len(problems) != 0 {
				t.Errorf("ParseSection(%s) problems = %v, want none", section, problems)
			}
			if exemptions == nil {
				t.Errorf("ParseSection(%s) exemptions = nil, want a usable value", section)
			}
		})
	}
}

func TestParseSection_problems(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		section string
		want    []lintkit.Finding
	}{
		{name: "section is a list", section: "[]", want: []lintkit.Finding{lintkit.SectionError("", "must be a map")}},
		{name: "section is a string", section: `"x"`, want: []lintkit.Finding{lintkit.SectionError("", "must be a map")}},
		{name: "unknown top key", section: `{"other":1,"exempt":[]}`, want: []lintkit.Finding{lintkit.SectionError("", "unknown key 'other'")}},
		{name: "exempt is a map", section: `{"exempt":{}}`, want: []lintkit.Finding{lintkit.SectionError("", "exempt must be a list")}},
		{name: "exempt is null", section: `{"exempt":null}`, want: []lintkit.Finding{lintkit.SectionError("", "exempt must be a list")}},
		{name: "entry is a string", section: `{"exempt":["a.md"]}`, want: []lintkit.Finding{lintkit.SectionError("", "exempt[0] must be a map")}},
		{
			name:    "unknown entry key",
			section: `{"exempt":[{"path":"a.md","reason":"r","step":"x"}]}`,
			want:    []lintkit.Finding{lintkit.SectionError("exempt[0]", "unknown key 'step'")},
		},
		{
			name:    "path missing",
			section: `{"exempt":[{"reason":"r"}]}`,
			want:    []lintkit.Finding{lintkit.SectionError("exempt[0]", "path is missing or empty")},
		},
		{
			name:    "path empty",
			section: `{"exempt":[{"path":"","reason":"r"}]}`,
			want:    []lintkit.Finding{lintkit.SectionError("exempt[0]", "path is missing or empty")},
		},
		{
			name:    "path null",
			section: `{"exempt":[{"path":null,"reason":"r"}]}`,
			want:    []lintkit.Finding{lintkit.SectionError("exempt[0]", "path must be a string")},
		},
		{
			name:    "path a number",
			section: `{"exempt":[{"path":12,"reason":"r"}]}`,
			want:    []lintkit.Finding{lintkit.SectionError("exempt[0]", "path must be a string")},
		},
		{
			name:    "bad glob",
			section: `{"exempt":[{"path":"docs/[a","reason":"r"}]}`,
			want:    []lintkit.Finding{lintkit.SectionError("exempt[0]", "path 'docs/[a' is not a valid glob")},
		},
		{
			name:    "reason missing",
			section: `{"exempt":[{"path":"a.md"}]}`,
			want:    []lintkit.Finding{lintkit.SectionError("exempt[0]", "reason is missing or empty")},
		},
		{
			name:    "reason blank",
			section: `{"exempt":[{"path":"a.md","reason":" \t"}]}`,
			want:    []lintkit.Finding{lintkit.SectionError("exempt[0]", "reason is missing or empty")},
		},
		{
			name:    "reason a list",
			section: `{"exempt":[{"path":"a.md","reason":["r"]}]}`,
			want:    []lintkit.Finding{lintkit.SectionError("exempt[0]", "reason must be a string")},
		},
		{
			name:    "match a number",
			section: `{"exempt":[{"path":"a.md","match":1,"reason":"r"}]}`,
			want:    []lintkit.Finding{lintkit.SectionError("exempt[0]", "match must be a string")},
		},
		{
			name:    "match empty",
			section: `{"exempt":[{"path":"a.md","match":"","reason":"r"}]}`,
			want:    []lintkit.Finding{lintkit.SectionError("exempt[0]", "match must not be empty; drop it to exempt the whole file")},
		},
		{
			name:    "bad regex",
			section: `{"exempt":[{"path":"a.md","match":"(","reason":"r"}]}`,
			want: []lintkit.Finding{lintkit.SectionError("exempt[0]",
				"match '(' is not a valid regex: error parsing regexp: missing closing ): `(`")},
		},
		{
			name:    "every problem in every entry",
			section: `{"x":1,"exempt":[{"path":"a.md","reason":"ok"},{"match":"(","y":2},3]}`,
			want: []lintkit.Finding{
				lintkit.SectionError("", "unknown key 'x'"),
				lintkit.SectionError("exempt[1]", "unknown key 'y'"),
				lintkit.SectionError("exempt[1]", "path is missing or empty"),
				lintkit.SectionError("exempt[1]", "match '(' is not a valid regex: error parsing regexp: missing closing ): `(`"),
				lintkit.SectionError("exempt[1]", "reason is missing or empty"),
				lintkit.SectionError("", "exempt[2] must be a map"),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, problems, err := issuerefs.ParseSection([]byte(tc.section))
			if err != nil {
				t.Fatalf("ParseSection(%s) error = %v", tc.section, err)
			}
			if diff := cmp.Diff(tc.want, problems, cmp.AllowUnexported(lintkit.Finding{})); diff != "" {
				t.Errorf("ParseSection(%s) problems mismatch (-want +got):\n%s", tc.section, diff)
			}
		})
	}
}

func TestParseSection_malformedJSONIsAnError(t *testing.T) {
	t.Parallel()
	if _, _, err := issuerefs.ParseSection([]byte("{")); err == nil {
		t.Error("ParseSection({) error = nil, want an error")
	}
}

func parse(t *testing.T, section string) *issuerefs.Exemptions {
	t.Helper()
	exemptions, problems, err := issuerefs.ParseSection([]byte(section))
	if err != nil || len(problems) != 0 {
		t.Fatalf("ParseSection(%s) = %v, %v", section, problems, err)
	}
	return exemptions
}

func TestExemptions_Exempts_glob(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{pattern: "CHANGELOG.md", path: "CHANGELOG.md", want: true},
		{pattern: "CHANGELOG.md", path: "docs/CHANGELOG.md", want: false},
		{pattern: "*.md", path: "a.md", want: true},
		{pattern: "*.md", path: "d/a.md", want: false},
		{pattern: "**/*.md", path: "a.md", want: true},
		{pattern: "**/*.md", path: "d/e/a.md", want: true},
		{pattern: "docs/**", path: "docs/a/b.md", want: true},
		{pattern: "docs/**", path: "other/a.md", want: false},
		{pattern: "a/**/b", path: "a/b", want: true},
		{pattern: "a/**/b", path: "a/x/y/b", want: true},
		{pattern: "a/**/b", path: "a/x/c", want: false},
		{pattern: "internal", path: "internal/x/y.go", want: true},
		{pattern: "intern", path: "internal/x/y.go", want: false},
		{pattern: "internal/issuerefs/*_test.go", path: "internal/issuerefs/scan_test.go", want: true},
		{pattern: "internal/issuerefs/*_test.go", path: "internal/issuerefs/scan.go", want: false},
		{pattern: "?.md", path: "a.md", want: true},
		{pattern: "[ab].md", path: "c.md", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.pattern+" "+tc.path, func(t *testing.T) {
			t.Parallel()
			exemptions := parse(t, `{"exempt":[{"path":"`+tc.pattern+`","reason":"r"}]}`)
			if got := exemptions.Exempts(tc.path, "see #1"); got != tc.want {
				t.Errorf("Exempts(%q) under %q = %v, want %v", tc.path, tc.pattern, got, tc.want)
			}
		})
	}
}

func TestExemptions_Exempts_match(t *testing.T) {
	t.Parallel()
	exemptions := parse(t, `{"exempt":[{"path":"t.md","match":"^\\| P-","reason":"probe ids"}]}`)
	if !exemptions.Exempts("t.md", "| P-1 | #3 |") {
		t.Error("a matching line in t.md is not exempt")
	}
	if exemptions.Exempts("t.md", "see #3") {
		t.Error("a line in t.md that does not match is exempt")
	}
	if exemptions.Exempts("u.md", "| P-1 | #3 |") {
		t.Error("a matching line in another file is exempt")
	}
}

func TestExemptions_Stale(t *testing.T) {
	t.Parallel()
	exemptions := parse(t, `{"exempt":[
		{"path":"CHANGELOG.md","reason":"r"},
		{"path":"t.md","match":"^\\| P-","reason":"r"},
		{"path":"used.md","reason":"r"},
		{"path":"used.md","match":"x","reason":"r"}
	]}`)
	exemptions.Exempts("used.md", "x #1")
	exemptions.Exempts("t.md", "see #1")
	want := []lintkit.Finding{
		{Path: ".repo-toolbelt.yml", Message: "stale exemption: CHANGELOG.md suppresses nothing"},
		{Path: ".repo-toolbelt.yml", Message: `stale exemption: t.md match ^\| P- suppresses nothing`},
	}
	if diff := cmp.Diff(want, exemptions.Stale(), cmp.AllowUnexported(lintkit.Finding{})); diff != "" {
		t.Errorf("Stale mismatch (-want +got):\n%s", diff)
	}
}

func FuzzParseSection(f *testing.F) {
	for _, seed := range []string{"null", "{}", `{"exempt":[{"path":"a/**/b","match":"x","reason":"r"}]}`, `{"exempt":[3]}`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, section string) {
		exemptions, problems, err := issuerefs.ParseSection([]byte(section))
		if err != nil && (exemptions != nil || problems != nil) {
			t.Errorf("an error came with exemptions %v or problems %v", exemptions, problems)
		}
		if err == nil && (exemptions == nil) == (len(problems) == 0) {
			t.Errorf("exemptions %v with problems %v, want exactly one of them", exemptions, problems)
		}
	})
}
