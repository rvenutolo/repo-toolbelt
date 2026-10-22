// Package issuerefs finds local issue and PR references, `#N`, in the text of
// tracked files, and applies the exemptions in the lint's config section.
//
// The scan is a token scan of the whole text, with no per-language parsing:
// comments, docs and message strings are covered by the same pass. What it
// skips is a fixed list of syntax that is never an issue reference: hex
// colours, bash's `${#…}` and `$#`, and a `#N` inside a full URL.
package issuerefs

import (
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Ref is one issue or PR reference.
type Ref struct {
	// Line is the 1-based line the reference is on.
	Line int
	// Text is the reference as written: `#N`, or `owner/repo#N` with an owner
	// and repository name before the hash.
	Text string
}

// styleExtensions are the stylesheet and markup files, where a `#` and three,
// four, six or eight digits is a hex colour.
//
//nolint:gochecknoglobals // immutable lookup table
var styleExtensions = map[string]bool{
	".css": true, ".scss": true, ".sass": true, ".less": true,
	".svg": true, ".html": true, ".htm": true, ".xml": true,
	".vue": true, ".jsx": true, ".tsx": true,
}

// urlBoundaries end the token a URL is looked for in: whitespace and the
// characters that commonly wrap a URL in prose, markup and code.
const urlBoundaries = " \t\"'`<>()[]{}"

// Scan returns every issue or PR reference in content, in order. path decides
// only whether short all-digit hex colours are expected, by its extension.
func Scan(path string, content []byte) []Ref {
	style := isStyleFile(path)
	var refs []Ref
	for i, line := range strings.Split(string(content), "\n") {
		for _, text := range scanLine(strings.TrimSuffix(line, "\r"), style) {
			refs = append(refs, Ref{Line: i + 1, Text: text})
		}
	}
	return refs
}

func isStyleFile(name string) bool {
	return styleExtensions[strings.ToLower(path.Ext(name))]
}

// scanLine returns the references on one line.
func scanLine(line string, style bool) []string {
	var found []string
	for i := 0; i < len(line); i++ {
		if line[i] != '#' {
			continue
		}
		end := i + 1
		for end < len(line) && isDigit(line[end]) {
			end++
		}
		if end == i+1 {
			continue
		}
		if next, _ := utf8.DecodeRuneInString(line[end:]); end < len(line) && isWord(next) {
			// `#1a2b3c`, `#12abc`: a hex colour or an identifier, not a number.
			continue
		}
		if text, ok := reference(line, i, end, style); ok {
			found = append(found, text)
		}
		i = end - 1
	}
	return found
}

// reference decides whether the `#` at hash, followed by digits up to end, is
// an issue reference, and returns its text.
func reference(line string, hash, end int, style bool) (string, bool) {
	text := line[hash:end]
	if inURL(line, hash) {
		return "", false
	}
	if hash > 0 {
		prev, _ := utf8.DecodeLastRuneInString(line[:hash])
		switch {
		case prev == '#', prev == '&', prev == '$':
			// `##12` is a heading marker or a doubled hash, `&#123;` an HTML
			// entity, `$#` bash's argument count.
			return "", false
		case prev == '{' && hash >= 2 && line[hash-2] == '$':
			// bash's `${#name}` length expansion.
			return "", false
		case isWord(prev):
			// After a word character, only `owner/repo#N` and the `PR#N`,
			// `GH#N` and `issue#N` prefixes read as issue references;
			// `C#N` and `foo#N` do not.
			start, ok := ownerRepoStart(line, hash)
			if !ok {
				start, ok = prefixStart(line, hash)
			}
			if !ok {
				return "", false
			}
			text = line[start:end]
		}
	}
	if isColour(line[:hash], line[end:], end-hash-1, style) {
		return "", false
	}
	return text, true
}

// inURL reports whether the `#` at hash belongs to a URL: the token it ends,
// up to whitespace or a wrapping character, holds a scheme separator.
func inURL(line string, hash int) bool {
	start := strings.LastIndexAny(line[:hash], urlBoundaries) + 1
	return strings.Contains(line[start:hash], "://")
}

// ownerRepoStart returns where an `owner/repo` before the `#` at hash starts.
func ownerRepoStart(line string, hash int) (int, bool) {
	start := hash
	for start > 0 && isRepoNameByte(line[start-1]) {
		start--
	}
	owner, repo, found := strings.Cut(line[start:hash], "/")
	if !found || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return 0, false
	}
	return start, true
}

// prefixStart returns where a `PR`, `GH` or `issue` prefix, in any case and
// as a whole word, starts before the `#` at hash.
func prefixStart(line string, hash int) (int, bool) {
	for _, prefix := range []string{"pr", "gh", "issue"} {
		start := hash - len(prefix)
		if start < 0 || !strings.EqualFold(line[start:hash], prefix) {
			continue
		}
		if before, _ := utf8.DecodeLastRuneInString(line[:start]); start > 0 && isWord(before) {
			continue
		}
		return start, true
	}
	return 0, false
}

// isColour reports whether a `#` and digits digits long, between before and
// after on its line, is a hex colour. Any colour length is one in a
// stylesheet or markup file, and so is a value that is the whole of a quoted
// string after a `:` or `=` (`"color": "#000000"`, `fill='#333'`). A full
// six- or eight-digit colour is also one as a declaration's bare value
// (`color: #123456`) or as the whole text of an element (`>#000000<`).
// Other all-digit values read as issue numbers.
func isColour(before, after string, digits int, style bool) bool {
	colourLength := digits == 3 || digits == 4 || digits == 6 || digits == 8
	if !colourLength {
		return false
	}
	if style || quotedValue(before, after) {
		return true
	}
	if digits != 6 && digits != 8 {
		return false
	}
	if strings.HasSuffix(before, ">") && strings.HasPrefix(after, "<") {
		return true
	}
	property, found := strings.CutSuffix(strings.TrimRight(before, " \t"), ":")
	if !found || property == "" {
		return false
	}
	last := property[len(property)-1]
	return isLetter(last) || last == '-'
}

// quotedValue reports whether the text between before and after is the whole
// of a quoted string assigned with `:` or `=`.
func quotedValue(before, after string) bool {
	if before == "" {
		return false
	}
	quote := before[len(before)-1]
	if quote != '"' && quote != '\'' {
		return false
	}
	if !strings.HasPrefix(after, string(quote)) {
		return false
	}
	assign := strings.TrimRight(before[:len(before)-1], " \t")
	return strings.HasSuffix(assign, ":") || strings.HasSuffix(assign, "=")
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isRepoNameByte(b byte) bool {
	return isLetter(b) || isDigit(b) || b == '_' || b == '.' || b == '-' || b == '/'
}

func isWord(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
