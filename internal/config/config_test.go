package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/config"
)

var known = []string{"check-issue-numbers", "check-workflow-hermetic"}

// repoWith returns a repo root holding the testdata fixture as its config file.
func repoWith(t *testing.T, fixture string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixture, err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, config.FileName), data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return root
}

func TestLoad_problems(t *testing.T) {
	t.Parallel()
	const tagFix = "; quote the value or drop the tag"
	tests := []struct {
		name    string
		fixture string
		want    []string
	}{
		{name: "two documents", fixture: "two-documents.yml", want: []string{"holds 2 YAML documents; expected one"}},
		{
			name: "merge key", fixture: "merge-key.yml",
			want: []string{
				"merge key '<<' is not supported (in check-workflow-hermetic.exempt[1])",
				"YAML alias '*base' is not supported (in check-workflow-hermetic.exempt[1].<<)",
			},
		},
		{
			name: "alias", fixture: "alias.yml",
			want: []string{"YAML alias '*entry' is not supported (in check-workflow-hermetic.exempt[1])"},
		},
		{
			name: "tag on a key", fixture: "tag-on-key.yml",
			want: []string{"YAML tag '!custom' is not supported (in check-workflow-hermetic)" + tagFix},
		},
		{
			name: "alias as a key", fixture: "alias-as-key.yml",
			want: []string{"YAML alias '*w' is not supported (in check-workflow-hermetic)"},
		},
		{
			name: "alias as an explicit key", fixture: "alias-as-explicit-key.yml",
			want: []string{"YAML alias '*w' is not supported (in check-workflow-hermetic)"},
		},
		{
			name: "tag on an explicit key", fixture: "tag-on-explicit-key.yml",
			want: []string{"YAML tag '!custom' is not supported (in check-workflow-hermetic)" + tagFix},
		},
		{
			name: "tag on a value", fixture: "tag-on-value.yml",
			want: []string{"YAML tag '!custom' is not supported (in check-workflow-hermetic.wrapper)" + tagFix},
		},
		{
			name: "tag on a list entry", fixture: "tag-on-entry.yml",
			want: []string{"YAML tag '!foo' is not supported (in check-workflow-hermetic.exempt[0])" + tagFix},
		},
		{
			name: "tag on the document root", fixture: "tag-on-root.yml",
			want: []string{"YAML tag '!x' is not supported (in top level)" + tagFix},
		},
		{
			name: "core tags", fixture: "core-tags.yml",
			want: []string{
				"YAML tag '!!str' is not supported (in check-workflow-hermetic.wrapper)" + tagFix,
				"YAML tag '!!seq' is not supported (in check-workflow-hermetic.exempt)" + tagFix,
			},
		},
		{
			name: "str tag on an int step", fixture: "str-tag-on-int-step.yml",
			want: []string{"YAML tag '!!str' is not supported (in check-workflow-hermetic.exempt[0].step)" + tagFix},
		},
		{
			name: "bool tag on a non-boolean", fixture: "bool-tag-on-non-boolean.yml",
			want: []string{"YAML tag '!!bool' is not supported (in check-issue-numbers.a)" + tagFix},
		},
		{name: "top level list", fixture: "top-level-list.yml", want: []string{"top level must be a map"}},
		{name: "top level scalar", fixture: "top-level-scalar.yml", want: []string{"top level must be a map"}},
		{
			name: "duplicate top-level key", fixture: "duplicate-top-level.yml",
			want: []string{"duplicate key 'check-workflow-hermetic' in top level"},
		},
		{
			name: "duplicate key in a section", fixture: "duplicate-in-section.yml",
			want: []string{"duplicate key 'wrapper' in check-workflow-hermetic"},
		},
		{
			name: "duplicate key in a list entry", fixture: "duplicate-in-entry.yml",
			want: []string{"duplicate key 'step' in check-workflow-hermetic.exempt[0]"},
		},
		{
			name: "duplicate explicit key", fixture: "duplicate-explicit-key.yml",
			want: []string{"duplicate key 'wrapper' in check-workflow-hermetic"},
		},
		{
			name: "unknown explicit key", fixture: "unknown-explicit-key.yml",
			want: []string{"unknown lint 'check-workflow-hermitic'; known: check-issue-numbers, check-workflow-hermetic"},
		},
		{
			name: "unknown lint", fixture: "unknown-lint.yml",
			want: []string{"unknown lint 'check-workflow-hermitic'; known: check-issue-numbers, check-workflow-hermetic"},
		},
		{
			name: "unknown lint with a newline", fixture: "unknown-lint-newline.yml",
			want: []string{
				`unknown lint 'check-workflow-hermetic\ncheck-workflow-hermetic'; ` +
					"known: check-issue-numbers, check-workflow-hermetic",
			},
		},
		{
			name: "unknown lint with a carriage return", fixture: "unknown-lint-carriage-return.yml",
			want: []string{`unknown lint 'a\rb'; known: check-issue-numbers, check-workflow-hermetic`},
		},
		{
			name: "problems come in file order", fixture: "problems-out-of-order.yml",
			want: []string{
				"unknown lint 'zzz'; known: check-issue-numbers, check-workflow-hermetic",
				"duplicate key 'wrapper' in check-workflow-hermetic",
				"unknown lint 'aaa'; known: check-issue-numbers, check-workflow-hermetic",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, got, err := config.Load(repoWith(t, tc.fixture), known)
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Load() problems mismatch (-want +got):\n%s", diff)
			}
			if section := string(cfg.Section("check-workflow-hermetic")); section != "null" {
				t.Errorf("Section after problems = %s, want null", section)
			}
		})
	}
}

// A tag is rejected wherever it appears, core tags included: decoding would
// otherwise coerce a value its tag does not fit, such as !!int x to 0.
func TestLoad_everyTagIsAProblem(t *testing.T) {
	t.Parallel()
	const tagFix = "; quote the value or drop the tag"
	coreTags := []struct {
		tag   string
		value string
	}{
		{tag: "!!str", value: "x"},
		{tag: "!!int", value: "1"},
		{tag: "!!bool", value: "true"},
		{tag: "!!float", value: "1.5"},
		{tag: "!!null", value: ""},
		{tag: "!!map", value: "{}"},
		{tag: "!!seq", value: "[]"},
	}
	type testCase struct {
		name    string
		content string
		want    []string
	}
	var tests []testCase
	for _, ct := range coreTags {
		tests = append(tests, testCase{
			name:    ct.tag + " on a value",
			content: "check-issue-numbers:\n  a: " + ct.tag + " " + ct.value + "\n",
			want:    []string{"YAML tag '" + ct.tag + "' is not supported (in check-issue-numbers.a)" + tagFix},
		})
		// The parser itself rejects !!seq on a scalar key; TestLoad_invalidYAML
		// covers that case.
		if ct.tag != "!!seq" {
			tests = append(tests, testCase{
				name:    ct.tag + " on a key",
				content: "check-issue-numbers:\n  " + ct.tag + " a: 1\n",
				want:    []string{"YAML tag '" + ct.tag + "' is not supported (in check-issue-numbers)" + tagFix},
			})
		}
	}
	tests = append(tests,
		testCase{
			name:    "int tag on a non-number",
			content: "check-issue-numbers:\n  a: !!int x\n",
			want:    []string{"YAML tag '!!int' is not supported (in check-issue-numbers.a)" + tagFix},
		},
		testCase{
			name:    "int tag on an overflowing number",
			content: "check-issue-numbers:\n  a: !!int 99999999999999999999\n",
			want:    []string{"YAML tag '!!int' is not supported (in check-issue-numbers.a)" + tagFix},
		},
		testCase{
			name:    "float tag on a non-number",
			content: "check-issue-numbers:\n  a: !!float abc\n",
			want:    []string{"YAML tag '!!float' is not supported (in check-issue-numbers.a)" + tagFix},
		},
		testCase{
			name:    "null tag on a value",
			content: "check-issue-numbers:\n  a: !!null something\n",
			want:    []string{"YAML tag '!!null' is not supported (in check-issue-numbers.a)" + tagFix},
		},
		testCase{
			name:    "tag on a section",
			content: "check-issue-numbers: !!map {a: 1}\n",
			want:    []string{"YAML tag '!!map' is not supported (in check-issue-numbers)" + tagFix},
		},
	)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, config.FileName), []byte(tc.content), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			cfg, got, err := config.Load(root, known)
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Load() problems mismatch (-want +got):\n%s", diff)
			}
			if section := string(cfg.Section("check-issue-numbers")); section != "null" {
				t.Errorf("Section after problems = %s, want null", section)
			}
		})
	}
}

func TestLoad_invalidYAML(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		fixture string
	}{
		{name: "unclosed flow sequence", fixture: "invalid.yml"},
		// A !!str-tagged << key in a flow map is rejected outright rather than
		// read as a plain key, so it can never reach a lint as either a merge
		// or a string key.
		{name: "str-tagged merge key in a flow map", fixture: "str-tagged-merge-key-in-flow-map.yml"},
		// A mapping used as a key has no text a lint could match, and YAML
		// that holds one is rejected before any key is read.
		{name: "flow mapping as a key", fixture: "flow-mapping-key.yml"},
		// A seq-tagged scalar key is rejected by the parser, so it never
		// reaches a lint as a key.
		{name: "seq tag on a key", fixture: "seq-tag-on-key.yml"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, problems, err := config.Load(repoWith(t, tc.fixture), known)
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
			if len(problems) != 1 || !strings.HasPrefix(problems[0], "not valid YAML: ") {
				t.Errorf("Load() problems = %q, want one starting with %q", problems, "not valid YAML: ")
			}
		})
	}
}

func TestLoad_danglingSymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Symlink("missing.yml", filepath.Join(root, config.FileName)); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	_, problems, err := config.Load(root, known)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if len(problems) != 1 || !strings.HasPrefix(problems[0], "not valid YAML: ") {
		t.Errorf("Load() problems = %q, want one starting with %q", problems, "not valid YAML: ")
	}
}

func TestLoad_unreadableFileIsASetupError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, config.FileName), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, problems, err := config.Load(root, known)
	if err == nil {
		t.Fatal("Load() error = nil, want an error for a config file that is a directory")
	}
	if len(problems) != 0 {
		t.Errorf("Load() problems = %q, want none", problems)
	}
}

func TestLoad_symlinkedFileIsRead(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	target := filepath.Join(root, "real.yml")
	if err := os.WriteFile(target, []byte("check-issue-numbers: {a: 1}\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Symlink("real.yml", filepath.Join(root, config.FileName)); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	cfg, problems, err := config.Load(root, known)
	if err != nil || len(problems) != 0 {
		t.Fatalf("Load() = (%q, %v), want no problems", problems, err)
	}
	if got, want := string(cfg.Section("check-issue-numbers")), `{"a":1}`; got != want {
		t.Errorf("Section = %s, want %s", got, want)
	}
}

func TestLoad_noProblems(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		fixture  string // empty means no config file
		sections map[string]string
	}{
		{name: "no file", sections: map[string]string{"check-workflow-hermetic": "null", "check-issue-numbers": "null"}},
		{name: "empty file", fixture: "empty.yml", sections: map[string]string{"check-workflow-hermetic": "null"}},
		{name: "comment only", fixture: "comments-only.yml", sections: map[string]string{"check-workflow-hermetic": "null"}},
		// A document end marker with nothing before it is a stream of no
		// documents, which is no config, as an empty file is.
		{name: "document end marker only", fixture: "document-end-only.yml", sections: map[string]string{"check-workflow-hermetic": "null"}},
		{
			name: "sections present", fixture: "valid.yml",
			sections: map[string]string{
				"check-workflow-hermetic": `{"exempt":[{"reason":"It only compares results.",` +
					`"step":".github/workflows/ci.yml::verdict"}],"wrapper":".ci/in-devshell"}`,
				"check-issue-numbers": "null",
			},
		},
		{
			name: "explicit keys", fixture: "explicit-keys.yml",
			sections: map[string]string{"check-workflow-hermetic": `{"wrapper":".ci/x"}`},
		},
		{name: "section holding null", fixture: "null-section.yml", sections: map[string]string{"check-issue-numbers": "null"}},
		{
			name: "section for a lint absent from the file", fixture: "valid.yml",
			sections: map[string]string{"not-a-lint": "null"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if tc.fixture != "" {
				root = repoWith(t, tc.fixture)
			}
			cfg, problems, err := config.Load(root, known)
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
			if len(problems) != 0 {
				t.Fatalf("Load() problems = %q, want none", problems)
			}
			for lint, want := range tc.sections {
				if got := string(cfg.Section(lint)); got != want {
					t.Errorf("Section(%q) = %s, want %s", lint, got, want)
				}
			}
		})
	}
}

func TestConfig_zeroValueHasNoSections(t *testing.T) {
	t.Parallel()
	var cfg config.Config
	if got := string(cfg.Section("check-issue-numbers")); got != "null" {
		t.Errorf("Section on the zero value = %s, want null", got)
	}
}

func TestConfig_sectionReturnsACopy(t *testing.T) {
	t.Parallel()
	cfg, _, err := config.Load(repoWith(t, "valid.yml"), known)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	first := cfg.Section("check-workflow-hermetic")
	first[0] = 'X'
	if got := cfg.Section("check-workflow-hermetic")[0]; got != '{' {
		t.Errorf("mutating a returned section changed the config: first byte %q, want '{'", got)
	}
}

func TestLoad_nonStringKeyInASection(t *testing.T) {
	t.Parallel()
	cfg, problems, err := config.Load(repoWith(t, "non-string-key.yml"), known)
	if err != nil || len(problems) != 0 {
		t.Fatalf("Load() = (%q, %v), want no problems", problems, err)
	}
	if got, want := string(cfg.Section("check-issue-numbers")), `{"1":"one"}`; got != want {
		t.Errorf("Section = %s, want %s", got, want)
	}
}

func TestLoad_anchorWithoutAliasIsAllowed(t *testing.T) {
	t.Parallel()
	cfg, problems, err := config.Load(repoWith(t, "anchor.yml"), known)
	if err != nil || len(problems) != 0 {
		t.Fatalf("Load() = (%q, %v), want no problems", problems, err)
	}
	if got, want := string(cfg.Section("check-issue-numbers")), `{"a":1}`; got != want {
		t.Errorf("Section = %s, want %s", got, want)
	}
}

func TestLoad_sectionThatCannotBeJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		fixture string
		want    string // the problem's prefix
	}{
		{name: "not a number", fixture: "not-a-number.yml", want: "check-issue-numbers: cannot be read as JSON: encode: "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, problems, err := config.Load(repoWith(t, tc.fixture), known)
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
			if len(problems) != 1 || !strings.HasPrefix(problems[0], tc.want) {
				t.Errorf("Load() problems = %q, want one starting with %q", problems, tc.want)
			}
			if section := string(cfg.Section("check-issue-numbers")); section != "null" {
				t.Errorf("Section after problems = %s, want null", section)
			}
		})
	}
}

// A section nested deeper than the decoder will go is a problem in that
// section, not a setup error.
func TestLoad_sectionNestedTooDeep(t *testing.T) {
	t.Parallel()
	const depth = 10001
	content := "check-issue-numbers:\n  a: " + strings.Repeat("[", depth) + strings.Repeat("]", depth) + "\n"
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, config.FileName), []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, problems, err := config.Load(root, known)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	const want = "check-issue-numbers: cannot be read as JSON: decode: "
	if len(problems) != 1 || !strings.HasPrefix(problems[0], want) {
		t.Errorf("Load() problems = %q, want one starting with %q", problems, want)
	}
	if section := string(cfg.Section("check-issue-numbers")); section != "null" {
		t.Errorf("Section after problems = %s, want null", section)
	}
}

func TestLoad_repoRootThatIsNotADirectoryIsASetupError(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "plain-file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, _, err := config.Load(file, known); err == nil {
		t.Fatal("Load() error = nil, want an error when the repo root is a file")
	}
}
