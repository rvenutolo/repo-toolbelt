package patchcov

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// Span is a run of lines, 1-based and inclusive.
type Span struct {
	Start, End int
}

// hunkHeader is a hunk header's start; the counts are bounded so that they
// cannot overflow.
var hunkHeader = regexp.MustCompile(`^@@ -[0-9]+(?:,[0-9]+)? \+([0-9]{1,9})(?:,([0-9]{1,9}))? @@`)

// diffOptions pins every diff setting that git config could otherwise
// change in a way that hides or invents a changed line: a textconv driver or
// the binary attribute (-diff) blanks the text, diff.external replaces the
// output, diff.relative narrows it, diff.interHunkContext merges hunks across
// unchanged lines, and renames would compare a file against another.
// -O/dev/null also keeps a diff.orderFile naming a host file the container
// lacks from making git refuse to diff at all.
var diffOptions = []string{ //nolint:gochecknoglobals // immutable lookup table
	"--unified=0", "--inter-hunk-context=0", "--no-renames", "--no-color",
	"--no-ext-diff", "--no-textconv", "--no-relative", "--text", "-O/dev/null",
}

// ChangedLines returns the lines the working tree adds or changes relative
// to base, by repo-relative file, for each file in wanted that has any.
// Uncommitted edits count; untracked files are not seen. A deleted line
// changes nothing: the lines around it are unchanged. git runs in root,
// which must be the repository root.
func ChangedLines(ctx context.Context, root, base string, wanted map[string]bool) (map[string][]Span, error) {
	names, err := gitOutput(ctx, root, append([]string{"diff", "--name-only", "-z"}, append(diffOptions, "--end-of-options", base, "--")...)...)
	if err != nil {
		return nil, err
	}
	changed := map[string][]Span{}
	for name := range strings.SplitSeq(strings.TrimSuffix(names, "\x00"), "\x00") {
		if !wanted[name] {
			continue
		}
		spans, err := fileSpans(ctx, root, base, name)
		if err != nil {
			return nil, err
		}
		if len(spans) > 0 {
			changed[name] = spans
		}
	}
	return changed, nil
}

// fileSpans diffs one file and reads the new side of each hunk header. Only
// one file is diffed, so no file header needs parsing, and every body line
// starts with a +, -, space or backslash, so a line starting "@@ " is always a
// hunk header.
func fileSpans(ctx context.Context, root, base, name string) ([]Span, error) {
	args := append([]string{"-c", "core.quotePath=false", "diff"}, diffOptions...)
	out, err := gitOutput(ctx, root, append(args, "--end-of-options", base, "--", ":(literal)"+name)...)
	if err != nil {
		return nil, err
	}
	var spans []Span
	for line := range strings.Lines(out) {
		if !strings.HasPrefix(line, "@@ ") {
			continue
		}
		span, ok, err := parseHunkHeader(line)
		if err != nil {
			return nil, fmt.Errorf("git diff %s: %w", name, err)
		}
		if ok {
			spans = append(spans, span)
		}
	}
	return spans, nil
}

// parseHunkHeader reads `@@ -a[,b] +c[,d] @@`: d lines from c are new. It
// reports false for a hunk that only deletes.
func parseHunkHeader(header string) (Span, bool, error) {
	m := hunkHeader.FindStringSubmatch(header)
	if m == nil {
		return Span{}, false, fmt.Errorf("unexpected hunk header %q", strings.TrimSpace(header))
	}
	start, count := digits(m[1]), 1
	if m[2] != "" {
		count = digits(m[2])
	}
	if count == 0 {
		return Span{}, false, nil
	}
	return Span{Start: start, End: start + count - 1}, true, nil
}

// digits reads a run of at most nine ASCII digits, which cannot overflow.
func digits(text string) int {
	n := 0
	for _, c := range text {
		n = n*10 + int(c-'0')
	}
	return n
}

// gitOutput runs git in dir, in the C locale, and returns its stdout.
func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // fixed program; revisions follow --end-of-options
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANGUAGE=")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg, _, _ := strings.Cut(strings.TrimSpace(stderr.String()), "\n")
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
	}
	return stdout.String(), nil
}
