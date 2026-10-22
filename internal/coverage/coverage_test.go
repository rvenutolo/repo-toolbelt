package coverage_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/coverage"
	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
)

func TestReadFile(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t, "module example.com/m\n")
	if err := os.MkdirAll(filepath.Join(root, "cov"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cov", "go.out"), []byte("mode: set\nexample.com/m/a.go:1.1,2.2 1 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cov", "c.xml"), []byte(cobertura(nil, class("x.sh", `<line number="1" hits="0"/>`))), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "o.out")
	if err := os.WriteFile(outside, []byte("mode: set\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "cov", "link.out")); err != nil {
		t.Fatal(err)
	}
	// kcov links its report directory by absolute path.
	if err := os.Symlink(filepath.Join(root, "cov"), filepath.Join(root, "abs")); err != nil {
		t.Fatal(err)
	}

	t.Run("go", func(t *testing.T) {
		t.Parallel()
		got, err := coverage.ReadFile(root, coverage.ReportSpec{Format: coverage.FormatGo, Path: "cov/go.out"})
		if err != nil {
			t.Fatalf("ReadFile error = %v", err)
		}
		want := coverage.Report{Blocks: []coverage.Block{{Path: "a.go", StartLine: 1, StartCol: 1, EndLine: 2, EndCol: 2, Statements: 1, Covered: true}}}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("ReadFile mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("cobertura", func(t *testing.T) {
		t.Parallel()
		got, err := coverage.ReadFile(root, coverage.ReportSpec{Format: coverage.FormatCobertura, Path: "cov/c.xml"})
		if err != nil {
			t.Fatalf("ReadFile error = %v", err)
		}
		if diff := cmp.Diff(coverage.Report{Blocks: []coverage.Block{line("x.sh", 1, false)}}, got); diff != "" {
			t.Errorf("ReadFile mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("through an absolute symlink inside the repo", func(t *testing.T) {
		t.Parallel()
		got, err := coverage.ReadFile(root, coverage.ReportSpec{Format: coverage.FormatCobertura, Path: "abs/c.xml"})
		if err != nil {
			t.Fatalf("ReadFile error = %v", err)
		}
		if diff := cmp.Diff(coverage.Report{Blocks: []coverage.Block{line("x.sh", 1, false)}}, got); diff != "" {
			t.Errorf("ReadFile mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("missing", func(t *testing.T) {
		t.Parallel()
		_, err := coverage.ReadFile(root, coverage.ReportSpec{Format: coverage.FormatGo, Path: "cov/none.out"})
		if !errors.Is(err, coverage.ErrNotFound) {
			t.Errorf("ReadFile error = %v, want ErrNotFound", err)
		}
	})
	t.Run("a symlink out of the repo", func(t *testing.T) {
		t.Parallel()
		_, err := coverage.ReadFile(root, coverage.ReportSpec{Format: coverage.FormatGo, Path: "cov/link.out"})
		// Only the repo is mounted in the image, so a link out of it fails
		// the read rather than reading a file elsewhere.
		if err == nil || errors.Is(err, coverage.ErrNotFound) {
			t.Errorf("ReadFile error = %v, want a read error", err)
		}
	})
	t.Run("a directory", func(t *testing.T) {
		t.Parallel()
		_, err := coverage.ReadFile(root, coverage.ReportSpec{Format: coverage.FormatGo, Path: "cov"})
		if err == nil || errors.Is(err, coverage.ErrNotFound) {
			t.Errorf("ReadFile error = %v, want a read error", err)
		}
	})
	t.Run("a missing repository root", func(t *testing.T) {
		t.Parallel()
		_, err := coverage.ReadFile(filepath.Join(root, "no-such-dir"), coverage.ReportSpec{Format: coverage.FormatGo, Path: "cov/go.out"})
		if err == nil || errors.Is(err, coverage.ErrNotFound) {
			t.Errorf("ReadFile error = %v, want an error that is not ErrNotFound", err)
		}
	})
	t.Run("a symlink loop", func(t *testing.T) {
		t.Parallel()
		if err := os.Symlink("loop.out", filepath.Join(root, "cov", "loop.out")); err != nil {
			t.Fatal(err)
		}
		_, err := coverage.ReadFile(root, coverage.ReportSpec{Format: coverage.FormatGo, Path: "cov/loop.out"})
		if err == nil || errors.Is(err, coverage.ErrNotFound) {
			t.Errorf("ReadFile error = %v, want an error that is not ErrNotFound", err)
		}
	})
	t.Run("an unreadable report", func(t *testing.T) {
		t.Parallel()
		if os.Geteuid() == 0 {
			t.Skip("root reads files whatever their mode")
		}
		if err := os.WriteFile(filepath.Join(root, "cov", "locked.out"), []byte("mode: set\n"), 0); err != nil {
			t.Fatal(err)
		}
		_, err := coverage.ReadFile(root, coverage.ReportSpec{Format: coverage.FormatGo, Path: "cov/locked.out"})
		if err == nil || errors.Is(err, coverage.ErrNotFound) {
			t.Errorf("ReadFile error = %v, want an error that is not ErrNotFound", err)
		}
	})
	t.Run("unknown format", func(t *testing.T) {
		t.Parallel()
		_, err := coverage.ReadFile(root, coverage.ReportSpec{Format: "lcov", Path: "cov/go.out"})
		if err == nil {
			t.Error("ReadFile of an unknown format succeeded, want an error")
		}
	})
}

func TestReadReports(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t, "module example.com/m\n")
	if err := os.MkdirAll(filepath.Join(root, "cov"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"go.out":      "mode: set\nexample.com/m/a.go:1.1,2.2 1 1\n",
		"outside.out": "mode: set\nexample.com/other/a.go:1.1,2.2 1 1\n",
		"bad.out":     "not a profile\n",
		"c.xml":       cobertura(nil, class("x.sh", `<line number="1" hits="0"/>`)),
	} {
		if err := os.WriteFile(filepath.Join(root, "cov", name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	goSpec := coverage.ReportSpec{Format: coverage.FormatGo, Path: "cov/go.out"}
	xmlSpec := coverage.ReportSpec{Format: coverage.FormatCobertura, Path: "cov/c.xml"}

	t.Run("every report read, in order", func(t *testing.T) {
		t.Parallel()
		reports, findings, err := coverage.ReadReports(root, coverage.Section{Reports: []coverage.ReportSpec{xmlSpec, goSpec}})
		if err != nil || findings != nil {
			t.Fatalf("ReadReports = %v, %v; want reports", findings, err)
		}
		want := []coverage.Report{
			{Blocks: []coverage.Block{{Path: "x.sh", StartLine: 1, EndLine: 1, Statements: 1}}},
			{Blocks: []coverage.Block{{Path: "a.go", StartLine: 1, StartCol: 1, EndLine: 2, EndCol: 2, Statements: 1, Covered: true}}},
		}
		if diff := cmp.Diff(want, reports); diff != "" {
			t.Errorf("ReadReports mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("missing and outside reports are findings", func(t *testing.T) {
		t.Parallel()
		reports, findings, err := coverage.ReadReports(root, coverage.Section{Reports: []coverage.ReportSpec{
			goSpec,
			{Format: coverage.FormatGo, Path: "cov/none.out"},
			{Format: coverage.FormatGo, Path: "cov/outside.out"},
		}})
		if err != nil || reports != nil {
			t.Fatalf("ReadReports = %v, %v; want findings only", reports, err)
		}
		want := []lintkit.Finding{
			{Path: "cov/none.out", Message: "coverage report not found"},
			{Path: "cov/outside.out", Message: "names example.com/other/a.go, which is outside the repository"},
		}
		if diff := cmp.Diff(want, findings, cmp.AllowUnexported(lintkit.Finding{})); diff != "" {
			t.Errorf("ReadReports findings mismatch (-want +got):\n%s", diff)
		}
	})
	t.Run("a malformed report is an error", func(t *testing.T) {
		t.Parallel()
		_, _, err := coverage.ReadReports(root, coverage.Section{Reports: []coverage.ReportSpec{{Format: coverage.FormatGo, Path: "cov/bad.out"}}})
		if err == nil || !strings.Contains(err.Error(), "cov/bad.out") {
			t.Errorf("ReadReports error = %v, want one naming cov/bad.out", err)
		}
	})
}
