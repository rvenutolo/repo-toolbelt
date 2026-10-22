package patchcov_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/patchcov"
)

func TestGoMarkers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  string
		want []patchcov.Marker
	}{
		{name: "no marker", src: "package p\n\nfunc f() {}\n"},
		{
			name: "a trailing line comment with a reason",
			src:  "package p\n\nfunc f(x int) {\n\tif x < 0 { // coverage-exempt: unreachable here\n\t\treturn\n\t}\n}\n",
			want: []patchcov.Marker{{Line: 4, Reason: "unreachable here"}},
		},
		{
			name: "a whole-line comment, no space after the slashes",
			src:  "package p\n\n//coverage-exempt:   spaced reason  \nfunc f() {}\n",
			want: []patchcov.Marker{{Line: 3, Reason: "spaced reason"}},
		},
		{
			name: "a marker with no reason",
			src:  "package p\n\n// coverage-exempt:\nfunc f() {}\n// coverage-exempt:   \n",
			want: []patchcov.Marker{{Line: 3}, {Line: 5}},
		},
		{
			name: "a block comment is not a marker",
			src:  "package p\n\n/* coverage-exempt: no */\n/*\n// coverage-exempt: inside a block comment\n*/\nfunc f() {}\n",
		},
		{
			name: "the word in a string is not a marker",
			src:  "package p\n\nconst s = \"// coverage-exempt: in a string\"\nconst r = `\n// coverage-exempt: in a raw string\n`\n",
		},
		{
			name: "a comment that only mentions the marker is not one",
			src:  "package p\n\n// Write coverage-exempt: <reason> on the line.\nfunc f() {}\n",
		},
		{
			name: "source that does not parse is still scanned",
			src:  "package p\n\nfunc f( {\n// coverage-exempt: still read\n",
			want: []patchcov.Marker{{Line: 4, Reason: "still read"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := patchcov.GoMarkers([]byte(tc.src))
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("GoMarkers mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHashMarkers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  string
		want []patchcov.Marker
	}{
		{name: "no marker", src: "#!/usr/bin/env bash\necho hi\n"},
		{
			name: "a trailing comment with a reason",
			src:  "#!/usr/bin/env bash\ndone < \"${f}\" # coverage-exempt: kcov never scores this\n",
			want: []patchcov.Marker{{Line: 2, Reason: "kcov never scores this"}},
		},
		{
			name: "a comment at the start of a line, tab separated",
			src:  "#coverage-exempt:\tx\n\t#  coverage-exempt: y\n",
			want: []patchcov.Marker{{Line: 1, Reason: "x"}, {Line: 2, Reason: "y"}},
		},
		{
			name: "a marker with no reason",
			src:  "a # coverage-exempt:\nb # coverage-exempt:   \r\n",
			want: []patchcov.Marker{{Line: 1}, {Line: 2}},
		},
		{
			name: "a # inside a word is not a comment",
			src:  "echo ${x#coverage-exempt: y}\n",
		},
		{
			name: "a comment that only mentions the marker is not one",
			src:  "# Mark the line with coverage-exempt: and a reason.\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := patchcov.HashMarkers([]byte(tc.src))
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("HashMarkers mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
