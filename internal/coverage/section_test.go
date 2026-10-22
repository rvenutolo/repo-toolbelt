package coverage_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/coverage"
	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
)

func TestParseSection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		section string
		want    coverage.Section
	}{
		{
			name:    "reports and the default baseline",
			section: `{"reports":[{"format":"go","path":"coverage.out"},{"format":"cobertura","path":".coverage/bats/cobertura.xml"}]}`,
			want: coverage.Section{
				Reports: []coverage.ReportSpec{
					{Format: coverage.FormatGo, Path: "coverage.out"},
					{Format: coverage.FormatCobertura, Path: ".coverage/bats/cobertura.xml"},
				},
				Baseline: ".coverage-baseline.txt",
			},
		},
		{
			name:    "a baseline named",
			section: `{"reports":[{"format":"go","path":"c.out"}],"baseline":"ci/baseline.txt"}`,
			want: coverage.Section{
				Reports:  []coverage.ReportSpec{{Format: coverage.FormatGo, Path: "c.out"}},
				Baseline: "ci/baseline.txt",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, problems, err := coverage.ParseSection([]byte(tc.section))
			if err != nil {
				t.Fatalf("ParseSection error = %v", err)
			}
			if len(problems) > 0 {
				t.Fatalf("ParseSection problems = %+v", problems)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ParseSection mismatch (-want +got):\n%s", diff)
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
		{name: "no section", section: `null`, want: []lintkit.Finding{lintkit.SectionError("", "reports is missing or empty")}},
		{name: "not a map", section: `[1]`, want: []lintkit.Finding{lintkit.SectionError("", "must be a map")}},
		{name: "no reports", section: `{}`, want: []lintkit.Finding{lintkit.SectionError("", "reports is missing or empty")}},
		{name: "empty reports", section: `{"reports":[]}`, want: []lintkit.Finding{lintkit.SectionError("", "reports is missing or empty")}},
		{name: "reports not a list", section: `{"reports":"c.out"}`, want: []lintkit.Finding{lintkit.SectionError("", "reports must be a list")}},
		{name: "an unknown key", section: `{"reports":[{"format":"go","path":"c.out"}],"exempt":[],"base":1}`, want: []lintkit.Finding{
			lintkit.SectionError("", "unknown key 'base'"),
			lintkit.SectionError("", "unknown key 'exempt'"),
		}},
		{name: "an entry not a map", section: `{"reports":["c.out"]}`, want: []lintkit.Finding{lintkit.SectionError("", "reports[0] must be a map")}},
		{name: "an entry with no format or path", section: `{"reports":[{}]}`, want: []lintkit.Finding{
			lintkit.SectionError("reports[0]", "format is missing or empty"),
			lintkit.SectionError("reports[0]", "path is missing or empty"),
		}},
		{name: "an unknown format", section: `{"reports":[{"format":"lcov","path":"c.info"}]}`, want: []lintkit.Finding{
			lintkit.SectionError("reports[0]", "format 'lcov' is not go or cobertura"),
		}},
		{name: "a format not a string", section: `{"reports":[{"format":1,"path":"c"}]}`, want: []lintkit.Finding{
			lintkit.SectionError("reports[0]", "format must be a string"),
		}},
		{name: "an entry with an unknown key", section: `{"reports":[{"format":"go","path":"c.out","name":"x"}]}`, want: []lintkit.Finding{
			lintkit.SectionError("reports[0]", "unknown key 'name'"),
		}},
		{name: "an absolute report path", section: `{"reports":[{"format":"go","path":"/tmp/c.out"}]}`, want: []lintkit.Finding{
			lintkit.SectionError("reports[0]", "path '/tmp/c.out' must be relative to the repo root and inside it"),
		}},
		{name: "a report path climbing out", section: `{"reports":[{"format":"go","path":"../c.out"}]}`, want: []lintkit.Finding{
			lintkit.SectionError("reports[0]", "path '../c.out' must be relative to the repo root and inside it"),
		}},
		{name: "a report listed twice", section: `{"reports":[{"format":"go","path":"c.out"},{"format":"go","path":"c.out"}]}`, want: []lintkit.Finding{
			lintkit.SectionError("reports[1]", "path 'c.out' is already listed"),
		}},
		{name: "a baseline not a string", section: `{"reports":[{"format":"go","path":"c.out"}],"baseline":3}`, want: []lintkit.Finding{
			lintkit.SectionError("", "baseline must be a string"),
		}},
		{name: "an empty baseline", section: `{"reports":[{"format":"go","path":"c.out"}],"baseline":" "}`, want: []lintkit.Finding{
			lintkit.SectionError("", "baseline is missing or empty"),
		}},
		{name: "a baseline outside the repo", section: `{"reports":[{"format":"go","path":"c.out"}],"baseline":"/b.txt"}`, want: []lintkit.Finding{
			lintkit.SectionError("", "baseline '/b.txt' must be relative to the repo root and inside it"),
		}},
		{name: "a baseline that is also a report", section: `{"reports":[{"format":"go","path":"c.out"}],"baseline":"c.out"}`, want: []lintkit.Finding{
			lintkit.SectionError("", "baseline 'c.out' is also a report"),
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, problems, err := coverage.ParseSection([]byte(tc.section))
			if err != nil {
				t.Fatalf("ParseSection error = %v", err)
			}
			if diff := cmp.Diff(tc.want, problems, cmp.AllowUnexported(lintkit.Finding{})); diff != "" {
				t.Errorf("ParseSection problems mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(coverage.Section{}, got); diff != "" {
				t.Errorf("ParseSection returned a section with problems (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseSection_notJSON(t *testing.T) {
	t.Parallel()
	if _, _, err := coverage.ParseSection([]byte("{")); err == nil {
		t.Error("ParseSection of invalid JSON succeeded, want an error")
	}
}
