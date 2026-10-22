package coverage_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/coverage"
)

func TestUnits(t *testing.T) {
	t.Parallel()
	report := coverage.Report{Blocks: []coverage.Block{
		{Path: "main.go", Statements: 2, Covered: false},
		{Path: "main.go", Statements: 5, Covered: true},
		{Path: "a/x.go", Statements: 3, Covered: false},
		{Path: "a/y.go", Statements: 1, Covered: false},
		{Path: "a/b/z.go", Statements: 4, Covered: true},
	}}
	want := map[string]int{".": 2, "a": 4, "a/b": 0}
	if diff := cmp.Diff(want, coverage.Units(report)); diff != "" {
		t.Errorf("Units mismatch (-want +got):\n%s", diff)
	}
}

func TestMerge(t *testing.T) {
	t.Parallel()
	a := coverage.Report{Blocks: []coverage.Block{
		{Path: "x.go", StartLine: 1, StartCol: 1, EndLine: 2, EndCol: 1, Statements: 2, Covered: false},
		{Path: "x.go", StartLine: 4, StartCol: 1, EndLine: 5, EndCol: 1, Statements: 1, Covered: true},
	}}
	b := coverage.Report{Blocks: []coverage.Block{
		{Path: "x.go", StartLine: 1, StartCol: 1, EndLine: 2, EndCol: 1, Statements: 2, Covered: true},
		{Path: "w.go", StartLine: 1, StartCol: 1, EndLine: 1, EndCol: 9, Statements: 1, Covered: false},
	}}
	want := coverage.Report{Blocks: []coverage.Block{
		{Path: "w.go", StartLine: 1, StartCol: 1, EndLine: 1, EndCol: 9, Statements: 1, Covered: false},
		{Path: "x.go", StartLine: 1, StartCol: 1, EndLine: 2, EndCol: 1, Statements: 2, Covered: true},
		{Path: "x.go", StartLine: 4, StartCol: 1, EndLine: 5, EndCol: 1, Statements: 1, Covered: true},
	}}
	if diff := cmp.Diff(want, coverage.Merge(a, b)); diff != "" {
		t.Errorf("Merge mismatch (-want +got):\n%s", diff)
	}
}

func TestTally(t *testing.T) {
	t.Parallel()
	specs := []coverage.ReportSpec{
		{Format: coverage.FormatGo, Path: "unit.out"},
		{Format: coverage.FormatCobertura, Path: "bats.xml"},
		{Format: coverage.FormatGo, Path: "integration.out"},
	}
	reports := []coverage.Report{
		{Blocks: []coverage.Block{{Path: "a/x.go", StartLine: 1, Statements: 3}, {Path: "main.go", StartLine: 1, Statements: 1}}},
		{Blocks: []coverage.Block{line("run-tests", 4, false), line("lib/t.bash", 1, true)}},
		{Blocks: []coverage.Block{{Path: "a/x.go", StartLine: 1, Statements: 3, Covered: true}}},
	}
	want := coverage.Baseline{
		{Format: "go", Name: "a"}:          0,
		{Format: "go", Name: "."}:          1,
		{Format: "cobertura", Name: "."}:   1,
		{Format: "cobertura", Name: "lib"}: 0,
	}
	if diff := cmp.Diff(want, coverage.Tally(specs, reports)); diff != "" {
		t.Errorf("Tally mismatch (-want +got):\n%s", diff)
	}
}
