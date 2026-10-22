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

func line(path string, n int, covered bool) coverage.Block {
	return coverage.Block{Path: path, StartLine: n, EndLine: n, Statements: 1, Covered: covered}
}

// cobertura wraps sources and classes in a Cobertura document.
func cobertura(sources []string, classes string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" ?>` + "\n" +
		`<!DOCTYPE coverage SYSTEM 'http://cobertura.sourceforge.net/xml/coverage-04.dtd'>` + "\n" +
		`<coverage line-rate="0.5"><sources>`)
	for _, s := range sources {
		b.WriteString("<source>" + s + "</source>")
	}
	b.WriteString(`</sources><packages><package name="p"><classes>` + classes + `</classes></package></packages></coverage>`)
	return b.String()
}

func class(filename, lines string) string {
	return `<class name="c" filename="` + filename + `"><lines>` + lines + `</lines></class>`
}

func TestReadCobertura(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "second", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "second", "b", "y.sh"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		xml  string
		want coverage.Report
	}{
		{
			name: "an absolute filename is made relative to the root",
			xml: cobertura([]string{filepath.Join(root, "lib")},
				class(filepath.Join(root, "lib", "x.bash"), `<line number="3" hits="2"/><line number="4" hits="0"/>`)),
			want: coverage.Report{Blocks: []coverage.Block{line("lib/x.bash", 3, true), line("lib/x.bash", 4, false)}},
		},
		{
			name: "a relative filename is joined to the source",
			xml:  cobertura([]string{root + "/lints/"}, class("a/x.sh", `<line number="1" hits="1"/>`)),
			want: coverage.Report{Blocks: []coverage.Block{line("lints/a/x.sh", 1, true)}},
		},
		{
			name: "a relative source is relative to the root",
			xml:  cobertura([]string{"src"}, class("x.py", `<line number="1" hits="0"/>`)),
			want: coverage.Report{Blocks: []coverage.Block{line("src/x.py", 1, false)}},
		},
		{
			name: "with no source a relative filename is relative to the root",
			xml:  cobertura(nil, class("x.sh", `<line number="1" hits="0"/>`)),
			want: coverage.Report{Blocks: []coverage.Block{line("x.sh", 1, false)}},
		},
		{
			name: "with several sources the first holding the file is used",
			xml: cobertura([]string{filepath.Join(root, "first"), filepath.Join(root, "second")},
				class("b/y.sh", `<line number="1" hits="0"/>`)),
			want: coverage.Report{Blocks: []coverage.Block{line("second/b/y.sh", 1, false)}},
		},
		{
			name: "with several sources and none holding the file the first is used",
			xml: cobertura([]string{filepath.Join(root, "first"), filepath.Join(root, "second")},
				class("b/z.sh", `<line number="1" hits="0"/>`)),
			want: coverage.Report{Blocks: []coverage.Block{line("first/b/z.sh", 1, false)}},
		},
		{
			name: "method lines are not counted twice",
			xml: cobertura([]string{root}, `<class name="c" filename="m.py"><methods><method name="f"><lines>`+
				`<line number="2" hits="0"/><line number="9" hits="0"/></lines></method></methods>`+
				`<lines><line number="2" hits="0"/></lines></class>`),
			want: coverage.Report{Blocks: []coverage.Block{line("m.py", 2, false)}},
		},
		{
			name: "a line in two classes is covered when either covers it",
			xml: cobertura([]string{root},
				class("a.sh", `<line number="5" hits="0"/><line number="6" hits="0"/>`)+
					class("a.sh", `<line number="5" hits="4"/><line number="6" hits="0"/>`)),
			want: coverage.Report{Blocks: []coverage.Block{line("a.sh", 5, true), line("a.sh", 6, false)}},
		},
		{
			name: "lines sorted by file then number",
			xml: cobertura([]string{root},
				class("b.sh", `<line number="1" hits="1"/>`)+class("a.sh", `<line number="10" hits="1"/><line number="2" hits="1"/>`)),
			want: coverage.Report{Blocks: []coverage.Block{line("a.sh", 2, true), line("a.sh", 10, true), line("b.sh", 1, true)}},
		},
		{
			name: "a report with no classes",
			xml:  `<coverage><sources/><packages/></coverage>`,
			want: coverage.Report{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := coverage.ReadCobertura(root, strings.NewReader(tc.xml))
			if err != nil {
				t.Fatalf("ReadCobertura error = %v", err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ReadCobertura mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestReadCobertura_outsideRoot(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "repo")
	tests := []struct {
		name     string
		xml      string
		filename string
	}{
		{name: "an absolute filename elsewhere", xml: cobertura(nil, class("/etc/x.sh", `<line number="1" hits="1"/>`)), filename: "/etc/x.sh"},
		{name: "an absolute filename in a sibling with the root as prefix", xml: cobertura(nil, class(root+"-other/x.sh", `<line number="1" hits="1"/>`)), filename: root + "-other/x.sh"},
		{name: "a relative filename climbing out", xml: cobertura([]string{root}, class("../x.sh", `<line number="1" hits="1"/>`)), filename: "../x.sh"},
		{name: "a source outside the root", xml: cobertura([]string{"/opt/src"}, class("x.sh", `<line number="1" hits="1"/>`)), filename: "x.sh"},
		{name: "the root itself", xml: cobertura(nil, class(root, `<line number="1" hits="1"/>`)), filename: root},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := coverage.ReadCobertura(root, strings.NewReader(tc.xml))
			var outside *coverage.OutsideError
			if !errors.As(err, &outside) {
				t.Fatalf("ReadCobertura error = %v, want an OutsideError", err)
			}
			if outside.Name != tc.filename {
				t.Errorf("OutsideError.Name = %q, want %q", outside.Name, tc.filename)
			}
		})
	}
}

func TestReadCobertura_malformed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		xml  string
	}{
		{name: "not XML", xml: "mode: set\n"},
		{name: "truncated", xml: `<coverage><packages><package><classes><class filename="a.sh"><lines>`},
		{name: "empty input", xml: ""},
		{name: "another root element", xml: `<report><class filename="a.sh"/></report>`},
		{name: "a class with no filename", xml: cobertura(nil, `<class name="c"><lines><line number="1" hits="1"/></lines></class>`)},
		{name: "a line with no number", xml: cobertura(nil, class("a.sh", `<line hits="1"/>`))},
		{name: "a line number of zero", xml: cobertura(nil, class("a.sh", `<line number="0" hits="1"/>`))},
		{name: "a line number that is not a number", xml: cobertura(nil, class("a.sh", `<line number="x" hits="1"/>`))},
		{name: "a line with no hits", xml: cobertura(nil, class("a.sh", `<line number="1"/>`))},
		{name: "negative hits", xml: cobertura(nil, class("a.sh", `<line number="1" hits="-1"/>`))},
		{name: "hits that are not a number", xml: cobertura(nil, class("a.sh", `<line number="1" hits="many"/>`))},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := coverage.ReadCobertura(t.TempDir(), strings.NewReader(tc.xml))
			if err == nil {
				t.Fatalf("ReadCobertura = %+v, want an error", got)
			}
			if _, ok := errors.AsType[*coverage.OutsideError](err); ok {
				t.Errorf("ReadCobertura error = %v, want a malformed-report error", err)
			}
		})
	}
}

func TestReadCobertura_capturedKcovReport(t *testing.T) {
	t.Parallel()
	f, err := os.Open(filepath.Join("testdata", "kcov-cobertura.xml"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	// The capture's <source> was rewritten from the checkout it ran in to this
	// root, which need not exist: the reader does not touch the files.
	report, err := coverage.ReadCobertura("/work/repo-toolbelt", f)
	if err != nil {
		t.Fatalf("ReadCobertura error = %v", err)
	}
	if len(report.Blocks) != 785 {
		t.Errorf("got %d lines, want the report's lines-valid, 785", len(report.Blocks))
	}
	want := map[string]int{
		".":                             96,
		".ci":                           173,
		"lib":                           19,
		"lints/check-workflow-hermetic": 45,
	}
	if diff := cmp.Diff(want, coverage.Units(report)); diff != "" {
		t.Errorf("Units mismatch (-want +got):\n%s", diff)
	}
}

func FuzzReadCobertura(f *testing.F) {
	f.Add(cobertura([]string{"/r"}, class("/r/a.sh", `<line number="1" hits="1"/>`)))
	f.Add(cobertura(nil, class("../a.sh", `<line number="1" hits="0"/>`)))
	f.Add(`<coverage/>`)
	f.Fuzz(func(t *testing.T, doc string) {
		report, err := coverage.ReadCobertura("/r", strings.NewReader(doc))
		if err != nil && len(report.Blocks) != 0 {
			t.Errorf("ReadCobertura returned blocks with error %v", err)
		}
		for _, b := range report.Blocks {
			if !filepath.IsLocal(b.Path) {
				t.Errorf("block path %q is not repo-relative", b.Path)
			}
		}
	})
}
