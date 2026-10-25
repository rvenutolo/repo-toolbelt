package coverage_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/coverage"
)

func gitInit(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", "init", "--quiet")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
}

func TestWithoutVendored(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	gitInit(t, root)
	attributes := "gen/** linguist-vendored\nold.sh linguist-vendored=true\nkept.sh -linguist-vendored\n"
	if err := os.WriteFile(filepath.Join(root, ".gitattributes"), []byte(attributes), 0o644); err != nil {
		t.Fatal(err)
	}
	reports := []coverage.Report{
		{Blocks: []coverage.Block{
			{Path: "a/x.go", StartLine: 1, Statements: 2},
			{Path: "gen/g.go", StartLine: 1, Statements: 4},
			{Path: "vendor/example.org/v/v.go", StartLine: 1, Statements: 3},
			{Path: "z.go", StartLine: 1, Statements: 1, Covered: true},
		}},
		// A file named in both reports, as two runs over shared code would.
		{Blocks: []coverage.Block{
			{Path: "a/x.go", StartLine: 7, Statements: 1},
			{Path: "gen/g.go", StartLine: 7, Statements: 1},
			{Path: "kept.sh", StartLine: 1, Statements: 1},
			{Path: "lib/t.sh", StartLine: 2, Statements: 1},
			{Path: "old.sh", StartLine: 1, Statements: 1},
			{Path: "third_party/vendor/x.sh", StartLine: 1, Statements: 1},
		}},
		{},
	}
	want := []coverage.Report{
		{Blocks: []coverage.Block{
			{Path: "a/x.go", StartLine: 1, Statements: 2},
			{Path: "z.go", StartLine: 1, Statements: 1, Covered: true},
		}},
		{Blocks: []coverage.Block{
			{Path: "a/x.go", StartLine: 7, Statements: 1},
			{Path: "kept.sh", StartLine: 1, Statements: 1},
			{Path: "lib/t.sh", StartLine: 2, Statements: 1},
		}},
		{},
	}
	got, err := coverage.WithoutVendored(t.Context(), root, reports)
	if err != nil {
		t.Fatalf("WithoutVendored error = %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("WithoutVendored mismatch (-want +got):\n%s", diff)
	}
}

func TestWithoutVendored_gitFailureIsAnError(t *testing.T) {
	t.Parallel()
	reports := []coverage.Report{{Blocks: []coverage.Block{{Path: "a/x.go", Statements: 1}}}}
	if _, err := coverage.WithoutVendored(t.Context(), t.TempDir(), reports); err == nil {
		t.Error("WithoutVendored outside a git repository error = nil, want the git failure")
	}
}
