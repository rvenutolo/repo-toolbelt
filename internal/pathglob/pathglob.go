// Package pathglob matches globs against repo-relative paths written with
// `/`: path.Match patterns within each segment, plus `**` across segments.
// Unlike in a git pathspec, `*` never matches a `/`.
package pathglob

import (
	"path"
	"strings"
)

// Valid reports whether every segment of pattern is a valid path.Match
// pattern; `**` is always valid.
func Valid(pattern string) bool {
	for segment := range strings.SplitSeq(pattern, "/") {
		if _, err := path.Match(segment, ""); err != nil {
			return false
		}
	}
	return true
}

// Match reports whether pattern matches the repo-relative path: `*`, `?` and
// classes stay within a segment, and a `**` segment matches any number of
// segments, none included. As with a git pathspec, a pattern that matches a
// leading directory of the path matches every file under it. The pattern must
// be Valid; an invalid pattern matches nothing.
func Match(pattern, file string) bool {
	return matchSegments(strings.Split(pattern, "/"), strings.Split(file, "/"))
}

func matchSegments(pattern, segments []string) bool {
	if len(pattern) == 0 {
		return true
	}
	if pattern[0] == "**" {
		for skip := range len(segments) + 1 {
			if matchSegments(pattern[1:], segments[skip:]) {
				return true
			}
		}
		return false
	}
	if len(segments) == 0 {
		return false
	}
	if ok, _ := path.Match(pattern[0], segments[0]); !ok { //nolint:errcheck // an invalid pattern is documented to match nothing
		return false
	}
	return matchSegments(pattern[1:], segments[1:])
}
