package coverage_test

import (
	"os"
	"path/filepath"
	"slices"
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
	if diff := cmp.Diff(want, coverage.Tally(specs, reports, nil)); diff != "" {
		t.Errorf("Tally mismatch (-want +got):\n%s", diff)
	}
}

func TestTally_markers(t *testing.T) {
	t.Parallel()
	specs := []coverage.ReportSpec{{Format: coverage.FormatGo, Path: "unit.out"}}
	// a/x.go has an uncovered block on lines 3-5 (3 statements) and another
	// on line 5 (1 statement); b/y.go has a covered block on lines 1-2.
	report := coverage.Report{Blocks: []coverage.Block{
		{Path: "a/x.go", StartLine: 3, StartCol: 1, EndLine: 5, EndCol: 2, Statements: 3},
		{Path: "a/x.go", StartLine: 5, StartCol: 3, EndLine: 5, EndCol: 9, Statements: 1},
		{Path: "b/y.go", StartLine: 1, EndLine: 2, Statements: 2, Covered: true},
	}}
	tests := []struct {
		name    string
		markers coverage.MarkerSource
		want    coverage.Baseline
	}{
		{
			name: "marker inside an uncovered block",
			markers: fakeMarkers(map[string][]coverage.Marker{
				"a/x.go": {{Line: 4, Reason: "r"}},
			}),
			want: coverage.Baseline{{Format: "go", Name: "a"}: 1, {Format: "go", Name: "b"}: 0},
		},
		{
			name: "marker without reason",
			markers: fakeMarkers(map[string][]coverage.Marker{
				"a/x.go": {{Line: 4}},
			}),
			want: coverage.Baseline{{Format: "go", Name: "a"}: 4, {Format: "go", Name: "b"}: 0},
		},
		{
			name: "marker outside every block",
			markers: fakeMarkers(map[string][]coverage.Marker{
				"a/x.go": {{Line: 9, Reason: "r"}},
			}),
			want: coverage.Baseline{{Format: "go", Name: "a"}: 4, {Format: "go", Name: "b"}: 0},
		},
		{
			name: "marker inside a covered block",
			markers: fakeMarkers(map[string][]coverage.Marker{
				"b/y.go": {{Line: 1, Reason: "r"}},
			}),
			want: coverage.Baseline{{Format: "go", Name: "a"}: 4, {Format: "go", Name: "b"}: 0},
		},
		{
			name: "one marker on a line two blocks share",
			markers: fakeMarkers(map[string][]coverage.Marker{
				"a/x.go": {{Line: 5, Reason: "r"}},
			}),
			want: coverage.Baseline{{Format: "go", Name: "a"}: 0, {Format: "go", Name: "b"}: 0},
		},
		{
			name:    "nil source",
			markers: nil,
			want:    coverage.Baseline{{Format: "go", Name: "a"}: 4, {Format: "go", Name: "b"}: 0},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := coverage.Tally(specs, []coverage.Report{report}, tc.markers)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Tally mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTally_markersAreReadPerFormat(t *testing.T) {
	t.Parallel()
	specs := []coverage.ReportSpec{
		{Format: coverage.FormatGo, Path: "unit.out"},
		{Format: coverage.FormatCobertura, Path: "bats.xml"},
	}
	reports := []coverage.Report{
		{Blocks: []coverage.Block{{Path: "x.go", StartLine: 1, EndLine: 1, Statements: 1}}},
		{Blocks: []coverage.Block{line("run-tests", 4, false)}},
	}
	var asked []string
	source := func(path, format string) []coverage.Marker {
		asked = append(asked, format+" "+path)
		return nil
	}
	coverage.Tally(specs, reports, source)
	slices.Sort(asked)
	want := []string{"cobertura run-tests", "go x.go"}
	if diff := cmp.Diff(want, asked); diff != "" {
		t.Errorf("marker lookups mismatch (-want +got):\n%s", diff)
	}
}

// fakeMarkers serves markers by path, whatever the format.
func fakeMarkers(byPath map[string][]coverage.Marker) coverage.MarkerSource {
	return func(path, _ string) []coverage.Marker { return byPath[path] }
}

func TestSourceMarkers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, "a.go", "package a\n\nfunc f() {} // coverage-exempt: r\n")
	writeTree(t, dir, "run.sh", "echo\nx # coverage-exempt: r\n")
	root := openRoot(t, dir)
	source := coverage.SourceMarkers(root)
	tests := []struct {
		name, path, format string
		want               []coverage.Marker
	}{
		{name: "go", path: "a.go", format: coverage.FormatGo, want: []coverage.Marker{{Line: 3, Reason: "r"}}},
		{name: "cobertura", path: "run.sh", format: coverage.FormatCobertura, want: []coverage.Marker{{Line: 2, Reason: "r"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tc.want, source(tc.path, tc.format)); diff != "" {
				t.Errorf("markers mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSourceMarkers_unreadable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTree(t, dir, "real.go", "package a\n\nfunc f() {} // coverage-exempt: r\n")
	if err := os.Symlink("real.go", filepath.Join(dir, "link.go")); err != nil {
		t.Fatal(err)
	}
	writeTree(t, dir, "locked.go", "package a\n\nfunc f() {} // coverage-exempt: r\n")
	if err := os.Chmod(filepath.Join(dir, "locked.go"), 0o000); err != nil {
		t.Fatal(err)
	}
	source := coverage.SourceMarkers(openRoot(t, dir))
	tests := []struct {
		name, path string
		skipRoot   bool
	}{
		{name: "missing", path: "nope.go"},
		{name: "symlink", path: "link.go"},
		{name: "no read permission", path: "locked.go", skipRoot: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.skipRoot && os.Geteuid() == 0 {
				t.Skip("root reads files whatever their mode")
			}
			if got := source(tc.path, coverage.FormatGo); got != nil {
				t.Errorf("markers = %v, want nil", got)
			}
		})
	}
}

func writeTree(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func openRoot(t *testing.T, dir string) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root
}

func TestExempts(t *testing.T) {
	t.Parallel()
	b := coverage.Block{StartLine: 3, EndLine: 5}
	tests := []struct {
		name string
		m    coverage.Marker
		want bool
	}{
		{name: "first line", m: coverage.Marker{Line: 3, Reason: "r"}, want: true},
		{name: "last line", m: coverage.Marker{Line: 5, Reason: "r"}, want: true},
		{name: "before", m: coverage.Marker{Line: 2, Reason: "r"}},
		{name: "after", m: coverage.Marker{Line: 6, Reason: "r"}},
		{name: "no reason", m: coverage.Marker{Line: 4}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := coverage.Exempts(tc.m, b); got != tc.want {
				t.Errorf("Exempts(%+v) = %v, want %v", tc.m, got, tc.want)
			}
		})
	}
}
