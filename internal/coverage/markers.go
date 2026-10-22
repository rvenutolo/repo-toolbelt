package coverage

import (
	"bytes"
	"go/scanner"
	"go/token"
	"os"
	"regexp"
	"strings"
)

// markerWord starts the text of an exempt marker comment.
const markerWord = "coverage-exempt:"

// hashMarker is a `#` comment holding a marker: the `#` starts the line or
// follows whitespace, as a shell comment does.
var hashMarker = regexp.MustCompile(`(?:^|[ \t])#[ \t]*` + markerWord + `(.*)$`)

// Marker is a coverage-exempt marker comment.
type Marker struct {
	// Line is the 1-based line the comment is on.
	Line int
	// Reason is the text after the marker word, trimmed; empty when the
	// marker gives none.
	Reason string
}

// GoMarkers returns the markers in Go source: line comments whose text
// starts with the marker word. Go's own scanner finds the comments, so the
// word in a string or in a /* */ comment is not a marker. Source that does
// not compile is scanned as far as the scanner can read it.
func GoMarkers(src []byte) []Marker {
	fset := token.NewFileSet()
	file := fset.AddFile("", -1, len(src))
	var s scanner.Scanner
	s.Init(file, src, func(token.Position, string) {}, scanner.ScanComments)
	var markers []Marker
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			return markers
		}
		if tok != token.COMMENT || !strings.HasPrefix(lit, "//") {
			continue
		}
		text, ok := strings.CutPrefix(strings.TrimLeft(lit[2:], " \t"), markerWord)
		if !ok {
			continue
		}
		markers = append(markers, Marker{Line: fset.Position(pos).Line, Reason: strings.TrimSpace(text)})
	}
}

// HashMarkers returns the markers in a file whose comments start with `#`,
// such as a shell script: a `#` at the start of a line or after whitespace,
// then the marker word. Lines are read as text, without parsing the
// language, so a marker must not sit inside a string.
func HashMarkers(src []byte) []Marker {
	var markers []Marker
	n := 0
	for line := range bytes.Lines(src) {
		n++
		m := hashMarker.FindSubmatch(bytes.TrimRight(line, "\r\n"))
		if m == nil {
			continue
		}
		markers = append(markers, Marker{Line: n, Reason: strings.TrimSpace(string(m[1]))})
	}
	return markers
}

// Exempts reports whether m exempts b from the coverage count: it gives a
// reason and sits on a line b spans. A marker without a reason exempts
// nothing.
func Exempts(m Marker, b Block) bool {
	return m.Reason != "" && b.StartLine <= m.Line && m.Line <= b.EndLine
}

// MarkerSource returns the markers in the file at a repo-relative path that a
// report of the given format scores. It returns nil for a file it cannot
// read.
type MarkerSource func(path, format string) []Marker

// SourceMarkers reads markers from the files under root: Go markers for
// FormatGo and hash markers for FormatCobertura. A file that is missing, not
// regular or unreadable has none, so nothing in it is ever exempted.
func SourceMarkers(root *os.Root) MarkerSource {
	return func(path, format string) []Marker {
		info, err := root.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		src, err := root.ReadFile(path)
		if err != nil {
			return nil
		}
		if format == FormatGo {
			return GoMarkers(src)
		}
		return HashMarkers(src)
	}
}
