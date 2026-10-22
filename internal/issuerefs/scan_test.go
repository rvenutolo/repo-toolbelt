package issuerefs_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/issuerefs"
)

func TestScan(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string
		line string
		want []string
	}{
		{name: "issue in prose", line: "fixed in #540", want: []string{"#540"}},
		{name: "trailing period", line: "see #12.", want: []string{"#12"}},
		{name: "in parentheses", line: "(#7)", want: []string{"#7"}},
		{name: "at line start", line: "#1", want: []string{"#1"}},
		{name: "owner repo form", line: "owner/repo#12", want: []string{"owner/repo#12"}},
		{name: "dotted owner repo form", line: "see my.org/re-po#3 too", want: []string{"my.org/re-po#3"}},
		{name: "numbered step", line: "step #2", want: []string{"#2"}},
		{name: "two on one line", line: "#3 and #4", want: []string{"#3", "#4"}},
		{name: "after a backslash", line: `\#9`, want: []string{"#9"}},
		{name: "in quotes after a url", line: `"https://x.test/a" #8`, want: []string{"#8"}},
		{name: "issue comment url", line: "https://github.com/o/r/issues/12#issuecomment-5"},
		{name: "url fragment", line: "https://example.com/page#12"},
		{name: "url fragment in markdown link", line: "[x](https://example.com/page#12)"},
		{name: "url fragment in quotes", line: `href="https://example.com/#12"`},
		{name: "bash array length", line: "${#items[@]}"},
		{name: "bash positional length", line: "${#1}"},
		{name: "bash argument count", line: "$#"},
		{name: "bash argument count before digits", line: "(( $#12 ))"},
		{name: "bash string length", line: `echo "${#arr}"`},
		{name: "six digit colour in a declaration", path: "notes.md", line: "color: #123456;"},
		{name: "eight digit colour in a declaration", path: "notes.md", line: "fill:#12345678"},
		{name: "three digit number after a colon is not a colour outside stylesheets", path: "notes.md", line: "fixed in: #540", want: []string{"#540"}},
		{name: "six digits after a bare colon", path: "notes.md", line: ": #123456", want: []string{"#123456"}},
		{name: "six digits after a spaced colon", path: "notes.md", line: "see : #123456", want: []string{"#123456"}},
		{name: "six digits in prose", path: "notes.md", line: "see #123456", want: []string{"#123456"}},
		{name: "seven digits in a declaration", path: "notes.md", line: "color: #1234567", want: []string{"#1234567"}},
		{name: "quoted colour as a json value", path: "x.json", line: `"color": "#000000",`},
		{name: "single quoted colour as a yaml value", path: "x.yml", line: "color: '#333333'"},
		{name: "colour attribute in tsx", path: "x.tsx", line: `<path fill="#000000"/>`},
		{name: "quoted colour attribute in a markdown file", path: "x.md", line: `<span color="#000000">`},
		{name: "colour as element text in xml", path: "x.xml", line: `<color name="x">#000000</color>`},
		{name: "colour as element text in markdown", path: "x.md", line: `<td>#000000</td>`},
		{name: "short quoted colour in ts", path: "x.ts", line: "color: '#333',"},
		{name: "three digit colour in vue", path: "x.vue", line: "a { color: #123 }"},
		{name: "three digit colour in jsx", path: "x.jsx", line: "fill: #123,"},
		{name: "quoted reference with more text", path: "x.yml", line: "title: '#540 fix'", want: []string{"#540"}},
		{name: "quoted reference with no key", path: "x.json", line: `["#540"]`, want: []string{"#540"}},
		{name: "unclosed quote after a colon", path: "x.yml", line: `x: "#540`, want: []string{"#540"}},
		{name: "mismatched quotes after a colon", path: "x.yml", line: `x: "#540'`, want: []string{"#540"}},
		{name: "three digits as element text in markdown", path: "x.md", line: "<td>#123</td>", want: []string{"#123"}},
		{name: "three digits at line start in markdown", path: "x.md", line: "#123 is open", want: []string{"#123"}},
		{name: "letter hex colour", line: "#fff"},
		{name: "mixed hex colour", line: "#1a2b3c"},
		{name: "three digit colour in css", path: "a.css", line: "a { color: #123 }"},
		{name: "three digit colour in scss", path: "dir/a.scss", line: "$c: #123;"},
		{name: "three digit colour in svg", path: "a.svg", line: `<rect fill="#123"/>`},
		{name: "three digit colour in html", path: "a.html", line: `<p style="color:#123">`},
		{name: "three digits in markdown", path: "a.md", line: "see #123", want: []string{"#123"}},
		{name: "five digits in css", path: "a.css", line: "/* see #12345 */", want: []string{"#12345"}},
		{name: "space after hash", line: "# 12 items"},
		{name: "double hash", line: "##12"},
		{name: "html entity", line: "&#123;"},
		{name: "c sharp version", line: "C#12"},
		{name: "word before hash", line: "foo#12"},
		{name: "path with two slashes", line: "a/b/c#12"},
		{name: "letter after digits", line: "#12abc"},
		{name: "underscore after digits", line: "#12_x"},
		{name: "unicode letter before hash", line: "é#12"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := tc.path
			if path == "" {
				path = "file.txt"
			}
			var got []string
			for _, ref := range issuerefs.Scan(path, []byte(tc.line+"\n")) {
				if ref.Line != 1 {
					t.Errorf("ref %q on line %d, want 1", ref.Text, ref.Line)
				}
				got = append(got, ref.Text)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Scan(%q, %q) mismatch (-want +got):\n%s", path, tc.line, diff)
			}
		})
	}
}

func TestScan_lineNumbersAreOneBased(t *testing.T) {
	t.Parallel()
	content := "nothing\r\nsee #1\r\n\nalso #2 and #3\nlast #4"
	want := []issuerefs.Ref{
		{Line: 2, Text: "#1"},
		{Line: 4, Text: "#2"},
		{Line: 4, Text: "#3"},
		{Line: 5, Text: "#4"},
	}
	if diff := cmp.Diff(want, issuerefs.Scan("a.md", []byte(content))); diff != "" {
		t.Errorf("Scan mismatch (-want +got):\n%s", diff)
	}
}

func TestScan_emptyContent(t *testing.T) {
	t.Parallel()
	if got := issuerefs.Scan("a.md", nil); len(got) != 0 {
		t.Errorf("Scan(nil) = %v, want none", got)
	}
}

func FuzzScan(f *testing.F) {
	for _, seed := range []string{"fixed in #540", "owner/repo#12", "${#items[@]}", "https://e.test/p#12", "color: #123456;", "é#1", "#"} {
		f.Add("a.md", seed)
	}
	f.Fuzz(func(t *testing.T, path, content string) {
		lines := 1
		for _, c := range content {
			if c == '\n' {
				lines++
			}
		}
		for _, ref := range issuerefs.Scan(path, []byte(content)) {
			if ref.Line < 1 || ref.Line > lines {
				t.Errorf("ref %q on line %d of %d", ref.Text, ref.Line, lines)
			}
			if len(ref.Text) < 2 {
				t.Errorf("ref text %q too short", ref.Text)
			}
		}
	})
}
