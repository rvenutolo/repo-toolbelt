package pathglob_test

import (
	"testing"

	"github.com/rvenutolo/repo-toolbelt/internal/pathglob"
)

func TestMatch(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		pattern string
		path    string
		want    bool
	}{
		{pattern: "a/*.go", path: "a/x.go", want: true},
		{pattern: "a/*.go", path: "a/b/x.go", want: false},
		{pattern: "**/x.go", path: "x.go", want: true},
		{pattern: "**/x.go", path: "a/b/x.go", want: true},
		{pattern: "**/x.go", path: "a/b/y.go", want: false},
		{pattern: "a/**", path: "a/b/c.go", want: true},
		{pattern: "a/**/z", path: "a/z", want: true},
		{pattern: "a/**/z", path: "a/b/c/z", want: true},
		{pattern: "test", path: "test/a/b.sh", want: true},
		{pattern: "test", path: "tests/a.sh", want: false},
		{pattern: "test/a", path: "test", want: false},
		{pattern: "te?t", path: "test", want: true},
		{pattern: "te?t", path: "te/t", want: false},
		{pattern: "[ab]", path: "a", want: true},
		{pattern: "[ab]", path: "c", want: false},
	} {
		t.Run(tt.pattern+" vs "+tt.path, func(t *testing.T) {
			t.Parallel()
			if got := pathglob.Match(tt.pattern, tt.path); got != tt.want {
				t.Errorf("Match(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

func TestValid(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		pattern string
		want    bool
	}{
		{pattern: "a/[", want: false},
		{pattern: "[a-", want: false},
		{pattern: "a/b\\", want: false},
		{pattern: "**/x", want: true},
		{pattern: "a/*.go", want: true},
		{pattern: "[ab]/x", want: true},
	} {
		t.Run(tt.pattern, func(t *testing.T) {
			t.Parallel()
			if got := pathglob.Valid(tt.pattern); got != tt.want {
				t.Errorf("Valid(%q) = %v, want %v", tt.pattern, got, tt.want)
			}
		})
	}
}
