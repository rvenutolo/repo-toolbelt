package fmtconfig_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/fmtconfig"
	"github.com/rvenutolo/repo-toolbelt/internal/manifest"
)

var (
	withIndent = manifest.Formatter{Name: "x", Settings: []string{"indent"}}
	noSettings = manifest.Formatter{Name: "x"}
)

func TestParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		raw          string
		formatter    manifest.Formatter
		want         fmtconfig.Section
		wantProblems []string
	}{
		{name: "null is no section", raw: `null`, formatter: withIndent},
		{name: "empty map is no section", raw: `{}`, formatter: withIndent},
		{
			name:      "valid section",
			raw:       `{"exclude":["test/fixtures/**","*.gen.sh"],"settings":{"indent":4}}`,
			formatter: withIndent,
			want: fmtconfig.Section{
				Exclude:  []string{"test/fixtures/**", "*.gen.sh"},
				Settings: map[string]int{"indent": 4},
			},
		},
		{
			name:      "empty exclude and settings are no entries",
			raw:       `{"exclude":[],"settings":{}}`,
			formatter: noSettings,
		},
		{name: "section is a list", raw: `[]`, formatter: withIndent, wantProblems: []string{"must be a map"}},
		{name: "section is a string", raw: `"x"`, formatter: withIndent, wantProblems: []string{"must be a map"}},
		{
			name:         "section is not json",
			raw:          `{`,
			formatter:    withIndent,
			wantProblems: []string{"is not valid JSON: unexpected EOF"},
		},
		{
			name:         "section is followed by more data",
			raw:          `{} {}`,
			formatter:    withIndent,
			wantProblems: []string{"is not valid JSON: unexpected trailing data"},
		},
		{
			name:         "section is followed by a closing brace",
			raw:          `{} }`,
			formatter:    withIndent,
			wantProblems: []string{"is not valid JSON: unexpected trailing data"},
		},
		{
			name:         "section is followed by a closing bracket",
			raw:          `{} ]`,
			formatter:    withIndent,
			wantProblems: []string{"is not valid JSON: unexpected trailing data"},
		},
		{
			name:         "unknown key",
			raw:          `{"x":1}`,
			formatter:    withIndent,
			wantProblems: []string{"unknown key 'x'; allowed: exclude, settings"},
		},
		{
			name:         "unknown keys are escaped and sorted",
			raw:          `{"b\nc":1,"a":2}`,
			formatter:    withIndent,
			wantProblems: []string{"unknown key 'a'; allowed: exclude, settings", `unknown key 'b\nc'; allowed: exclude, settings`},
		},
		{name: "exclude is a map", raw: `{"exclude":{}}`, formatter: withIndent, wantProblems: []string{"exclude: must be a list"}},
		{name: "exclude is null", raw: `{"exclude":null}`, formatter: withIndent, wantProblems: []string{"exclude: must be a list"}},
		{name: "exclude entry is a number", raw: `{"exclude":["a",1]}`, formatter: withIndent, wantProblems: []string{"exclude[1]: must be a string"}},
		{name: "exclude entry is empty", raw: `{"exclude":[""]}`, formatter: withIndent, wantProblems: []string{"exclude[0]: must not be empty"}},
		{name: "exclude entry is blank", raw: `{"exclude":["  "]}`, formatter: withIndent, wantProblems: []string{"exclude[0]: must not be empty"}},
		{name: "exclude entry is an invalid glob", raw: `{"exclude":["ok","a/["]}`, formatter: withIndent, wantProblems: []string{"exclude[1]: invalid glob 'a/['"}},
		{name: "invalid glob is escaped", raw: `{"exclude":["a\n["]}`, formatter: withIndent, wantProblems: []string{`exclude[0]: invalid glob 'a\n['`}},
		{
			name:         "every bad exclude entry is reported",
			raw:          `{"exclude":[1,"","a/["]}`,
			formatter:    withIndent,
			wantProblems: []string{"exclude[0]: must be a string", "exclude[1]: must not be empty", "exclude[2]: invalid glob 'a/['"},
		},
		{name: "settings is a list", raw: `{"settings":[]}`, formatter: withIndent, wantProblems: []string{"settings: must be a map"}},
		{name: "settings is null", raw: `{"settings":null}`, formatter: withIndent, wantProblems: []string{"settings: must be a map"}},
		{
			name:         "setting outside the allowlist",
			raw:          `{"settings":{"tabs":1}}`,
			formatter:    withIndent,
			wantProblems: []string{"settings: unknown setting 'tabs'; allowed: indent"},
		},
		{
			name:         "setting names are escaped and allowlist is listed in full",
			raw:          `{"settings":{"t\nabs":1}}`,
			formatter:    manifest.Formatter{Name: "x", Settings: []string{"indent", "width"}},
			wantProblems: []string{`settings: unknown setting 't\nabs'; allowed: indent, width`},
		},
		{
			name:         "formatter with an empty allowlist takes no settings",
			raw:          `{"settings":{"indent":4}}`,
			formatter:    noSettings,
			wantProblems: []string{"settings: check-x-formatted takes no settings"},
		},
		{name: "setting is zero", raw: `{"settings":{"indent":0}}`, formatter: withIndent, wantProblems: []string{"settings.indent: must be a positive integer"}},
		{name: "setting is negative", raw: `{"settings":{"indent":-1}}`, formatter: withIndent, wantProblems: []string{"settings.indent: must be a positive integer"}},
		{name: "setting is fractional", raw: `{"settings":{"indent":1.5}}`, formatter: withIndent, wantProblems: []string{"settings.indent: must be a positive integer"}},
		{name: "setting is a string", raw: `{"settings":{"indent":"4"}}`, formatter: withIndent, wantProblems: []string{"settings.indent: must be a positive integer"}},
		{name: "setting is null", raw: `{"settings":{"indent":null}}`, formatter: withIndent, wantProblems: []string{"settings.indent: must be a positive integer"}},
		{name: "setting overflows an int", raw: `{"settings":{"indent":99999999999999999999}}`, formatter: withIndent, wantProblems: []string{"settings.indent: must be a positive integer"}},
		{
			name:      "problems come in section order, settings by key",
			raw:       `{"settings":{"width":0,"indent":0,"zzz":1},"exclude":[1],"other":1}`,
			formatter: manifest.Formatter{Name: "x", Settings: []string{"indent", "width"}},
			wantProblems: []string{
				"unknown key 'other'; allowed: exclude, settings",
				"exclude[0]: must be a string",
				"settings.indent: must be a positive integer",
				"settings.width: must be a positive integer",
				"settings: unknown setting 'zzz'; allowed: indent, width",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, problems := fmtconfig.Parse([]byte(tc.raw), tc.formatter)
			if diff := cmp.Diff(tc.wantProblems, problems); diff != "" {
				t.Errorf("Parse(%s) problems mismatch (-want +got):\n%s", tc.raw, diff)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Parse(%s) section mismatch (-want +got):\n%s", tc.raw, diff)
			}
		})
	}
}
