package coverage_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/coverage"
)

// moduleRoot returns a directory holding a go.mod with the given content.
func moduleRoot(t *testing.T, gomod string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestReadGo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		gomod   string
		profile string
		want    coverage.Report
	}{
		{
			name:    "set mode",
			gomod:   "module example.com/m\n\ngo 1.22\n",
			profile: "mode: set\nexample.com/m/a/x.go:3.10,5.2 2 1\nexample.com/m/a/x.go:7.1,9.2 3 0\n",
			want: coverage.Report{Blocks: []coverage.Block{
				{Path: "a/x.go", StartLine: 3, StartCol: 10, EndLine: 5, EndCol: 2, Statements: 2, Covered: true},
				{Path: "a/x.go", StartLine: 7, StartCol: 1, EndLine: 9, EndCol: 2, Statements: 3, Covered: false},
			}},
		},
		{
			name:    "count mode",
			gomod:   "module example.com/m\n",
			profile: "mode: count\nexample.com/m/a/x.go:3.10,5.2 2 17\n",
			want: coverage.Report{Blocks: []coverage.Block{
				{Path: "a/x.go", StartLine: 3, StartCol: 10, EndLine: 5, EndCol: 2, Statements: 2, Covered: true},
			}},
		},
		{
			name:    "atomic mode",
			gomod:   "module example.com/m\n",
			profile: "mode: atomic\nexample.com/m/a/x.go:3.10,5.2 2 0\n",
			want: coverage.Report{Blocks: []coverage.Block{
				{Path: "a/x.go", StartLine: 3, StartCol: 10, EndLine: 5, EndCol: 2, Statements: 2, Covered: false},
			}},
		},
		{
			name:  "a duplicate block is covered when any of its counts is",
			gomod: "module example.com/m\n",
			profile: "mode: set\nexample.com/m/a/x.go:3.10,5.2 2 0\nexample.com/m/a/x.go:7.1,9.2 1 0\n" +
				"example.com/m/a/x.go:3.10,5.2 2 1\nexample.com/m/a/x.go:7.1,9.2 1 0\n",
			want: coverage.Report{Blocks: []coverage.Block{
				{Path: "a/x.go", StartLine: 3, StartCol: 10, EndLine: 5, EndCol: 2, Statements: 2, Covered: true},
				{Path: "a/x.go", StartLine: 7, StartCol: 1, EndLine: 9, EndCol: 2, Statements: 1, Covered: false},
			}},
		},
		{
			name:    "a file of the module root package",
			gomod:   "module example.com/m\n",
			profile: "mode: set\nexample.com/m/main.go:1.1,2.2 1 1\n",
			want: coverage.Report{Blocks: []coverage.Block{
				{Path: "main.go", StartLine: 1, StartCol: 1, EndLine: 2, EndCol: 2, Statements: 1, Covered: true},
			}},
		},
		{
			name:    "a quoted module path and CRLF lines",
			gomod:   "module \"example.com/m\"\r\n",
			profile: "mode: set\r\nexample.com/m/a/x.go:3.10,5.2 2 1\r\n",
			want: coverage.Report{Blocks: []coverage.Block{
				{Path: "a/x.go", StartLine: 3, StartCol: 10, EndLine: 5, EndCol: 2, Statements: 2, Covered: true},
			}},
		},
		{
			name:    "blocks sorted by file then position",
			gomod:   "module example.com/m\n",
			profile: "mode: set\nexample.com/m/b.go:9.1,9.9 1 1\nexample.com/m/a.go:5.1,6.1 1 1\nexample.com/m/a.go:5.0,5.9 1 0\n",
			want: coverage.Report{Blocks: []coverage.Block{
				{Path: "a.go", StartLine: 5, StartCol: 0, EndLine: 5, EndCol: 9, Statements: 1, Covered: false},
				{Path: "a.go", StartLine: 5, StartCol: 1, EndLine: 6, EndCol: 1, Statements: 1, Covered: true},
				{Path: "b.go", StartLine: 9, StartCol: 1, EndLine: 9, EndCol: 9, Statements: 1, Covered: true},
			}},
		},
		{
			name:    "a repeated identical mode line, as from concatenated profiles",
			gomod:   "module example.com/m\n",
			profile: "mode: set\nexample.com/m/a.go:1.1,2.2 1 1\nmode: set\nexample.com/m/b.go:1.1,2.2 1 0\n",
			want: coverage.Report{Blocks: []coverage.Block{
				{Path: "a.go", StartLine: 1, StartCol: 1, EndLine: 2, EndCol: 2, Statements: 1, Covered: true},
				{Path: "b.go", StartLine: 1, StartCol: 1, EndLine: 2, EndCol: 2, Statements: 1, Covered: false},
			}},
		},
		{
			name:    "a mode line only",
			gomod:   "module example.com/m\n",
			profile: "mode: set\n",
			want:    coverage.Report{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := moduleRoot(t, tc.gomod)
			got, err := coverage.ReadGo(root, strings.NewReader(tc.profile))
			if err != nil {
				t.Fatalf("ReadGo error = %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ReadGo mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestReadGo_fileOutsideModule(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"example.com/other/x.go", "example.com/mod/x.go", "/abs/x.go"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := moduleRoot(t, "module example.com/m\n")
			_, err := coverage.ReadGo(root, strings.NewReader("mode: set\n"+name+":1.1,2.2 1 1\n"))
			var outside *coverage.OutsideError
			if !errors.As(err, &outside) {
				t.Fatalf("ReadGo error = %v, want an OutsideError", err)
			}
			if outside.Name != name {
				t.Errorf("OutsideError.Name = %q, want %q", outside.Name, name)
			}
			if !strings.Contains(err.Error(), name) {
				t.Errorf("error %q does not name %q", err, name)
			}
		})
	}
}

func TestReadGo_malformed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		profile string
		where   string
	}{
		{name: "empty input", profile: "", where: "mode"},
		{name: "no mode line", profile: "example.com/m/a.go:1.1,2.2 1 1\n", where: "line 1"},
		{name: "unknown mode", profile: "mode: sometimes\n", where: "line 1"},
		{name: "a second, different mode", profile: "mode: set\nmode: count\n", where: "line 2"},
		{name: "no colon", profile: "mode: set\nexample.com/m/a.go 1 1\n", where: "line 2"},
		{name: "too few fields", profile: "mode: set\nexample.com/m/a.go:1.1,2.2 1\n", where: "line 2"},
		{name: "too many fields", profile: "mode: set\nexample.com/m/a.go:1.1,2.2 1 1 1\n", where: "line 2"},
		{name: "no comma in the position", profile: "mode: set\nexample.com/m/a.go:1.1 1 1\n", where: "line 2"},
		{name: "no column", profile: "mode: set\nexample.com/m/a.go:1,2.2 1 1\n", where: "line 2"},
		{name: "a letter in a line number", profile: "mode: set\nexample.com/m/a.go:x.1,2.2 1 1\n", where: "line 2"},
		{name: "a letter in an end line", profile: "mode: set\nexample.com/m/a.go:1.1,y.2 1 1\n", where: "line 2"},
		{name: "a letter in a column", profile: "mode: set\nexample.com/m/a.go:1.z,2.2 1 1\n", where: "line 2"},
		{name: "a letter in the statements", profile: "mode: set\nexample.com/m/a.go:1.1,2.2 n 1\n", where: "line 2"},
		{name: "a negative count", profile: "mode: set\nexample.com/m/a.go:1.1,2.2 1 -1\n", where: "line 2"},
		{name: "a negative statement count", profile: "mode: set\nexample.com/m/a.go:1.1,2.2 -1 1\n", where: "line 2"},
		{name: "a duplicate block with other statements", profile: "mode: set\nexample.com/m/a.go:1.1,2.2 1 1\nexample.com/m/a.go:1.1,2.2 2 1\n", where: "line 3"},
		{name: "a line over the length limit", profile: "mode: set\nexample.com/m/" + strings.Repeat("a", 2<<20) + ".go:1.1,2.2 1 1\n", where: "line 2"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := moduleRoot(t, "module example.com/m\n")
			got, err := coverage.ReadGo(root, strings.NewReader(tc.profile))
			if err == nil {
				t.Fatalf("ReadGo = %+v, want an error", got)
			}
			if _, ok := errors.AsType[*coverage.OutsideError](err); ok {
				t.Errorf("ReadGo error = %v, want a malformed-report error, not an OutsideError", err)
			}
			if !strings.Contains(err.Error(), tc.where) {
				t.Errorf("error %q does not say %q", err, tc.where)
			}
		})
	}
}

func TestReadGo_goMod(t *testing.T) {
	t.Parallel()
	t.Run("missing", func(t *testing.T) {
		t.Parallel()
		_, err := coverage.ReadGo(t.TempDir(), strings.NewReader("mode: set\n"))
		if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("ReadGo error = %v, want one wrapping os.ErrNotExist", err)
		}
	})
	t.Run("no module line", func(t *testing.T) {
		t.Parallel()
		root := moduleRoot(t, "go 1.22\n")
		_, err := coverage.ReadGo(root, strings.NewReader("mode: set\n"))
		if err == nil || !strings.Contains(err.Error(), "go.mod") {
			t.Errorf("ReadGo error = %v, want one naming go.mod", err)
		}
	})
}

func TestReadGo_capturedProfile(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t, "module github.com/rvenutolo/repo-toolbelt\n")
	f, err := os.Open(filepath.Join("testdata", "go-coverage.out"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	report, err := coverage.ReadGo(root, f)
	if err != nil {
		t.Fatalf("ReadGo error = %v", err)
	}
	want := map[string]int{
		"cmd/check-issue-numbers": 1,
		"cmd/toolbelt":            7,
		"internal/cli":            11,
		"internal/config":         3,
		"internal/issuerefs":      6,
		"internal/lintkit":        8,
		"internal/manifest":       1,
		"internal/refs":           2,
		"internal/runner":         2,
		"internal/selection":      0,
	}
	if diff := cmp.Diff(want, coverage.Units(report)); diff != "" {
		t.Errorf("Units mismatch (-want +got):\n%s", diff)
	}
}

func FuzzReadGo(f *testing.F) {
	f.Add("mode: set\nexample.com/m/a/x.go:3.10,5.2 2 1\n")
	f.Add("mode: count\nexample.com/m/a.go:1.1,2.2 1 1\nexample.com/m/a.go:1.1,2.2 1 0\n")
	f.Add("mode: atomic\nexample.com/other/x.go:1.1,2.2 1 1\n")
	f.Add("")
	root := f.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/m\n"), 0o644); err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, profile string) {
		report, err := coverage.ReadGo(root, strings.NewReader(profile))
		if err != nil && len(report.Blocks) != 0 {
			t.Errorf("ReadGo returned blocks with error %v", err)
		}
		for _, b := range report.Blocks {
			if !filepath.IsLocal(b.Path) {
				t.Errorf("block path %q is not repo-relative", b.Path)
			}
		}
	})
}
