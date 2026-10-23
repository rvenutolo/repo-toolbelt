package issuerefs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
)

// Check is the check-issue-numbers lint: it validates the section, then
// reports every reference in the tracked text files that no exemption covers,
// and every exemption that covered nothing.
func Check(ctx context.Context, lc lintkit.Context) ([]lintkit.Finding, error) {
	exemptions, problems, err := ParseSection(lc.Section)
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		return problems, nil
	}
	root, err := os.OpenRoot(lc.Root)
	if err != nil {
		return nil, fmt.Errorf("open repository root: %w", err)
	}
	defer root.Close()
	files, err := textFiles(ctx, lc.Root)
	if err != nil {
		return nil, err
	}
	var findings []lintkit.Finding
	for _, file := range files {
		content, ok, err := lintkit.ReadRegular(root, file)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		findings = append(findings, scanFile(file, content, exemptions)...)
	}
	return append(findings, exemptions.Stale()...), nil
}

// textFiles lists the tracked files git takes for text, by git's own test,
// in git's order. A tracked file missing from the working tree is left out.
func textFiles(ctx context.Context, dir string) ([]string, error) {
	// --no-recurse-submodules: a repo's submodule.recurse would otherwise
	// widen the scan to files the repo does not track.
	// An empty pattern matches every line, so this lists every tracked text
	// file with any content; an empty file holds no reference to report.
	cmd := exec.CommandContext(ctx, "git", "grep", "--no-recurse-submodules", "-I", "-z", "--files-with-matches", "--color=never", "-e", "", "--")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && stderr.Len() == 0:
		// git grep exits 1 when nothing matched: no text file has content.
		return nil, nil
	default:
		return nil, fmt.Errorf("list tracked text files: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var files []string
	for name := range strings.SplitSeq(strings.TrimSuffix(stdout.String(), "\x00"), "\x00") {
		files = append(files, name)
	}
	return files, nil
}

func scanFile(file string, content []byte, exemptions *Exemptions) []lintkit.Finding {
	refs := Scan(file, content)
	if len(refs) == 0 {
		return nil
	}
	lines := strings.Split(string(content), "\n")
	var findings []lintkit.Finding
	for _, ref := range refs {
		if exemptions.Exempts(file, strings.TrimSuffix(lines[ref.Line-1], "\r")) {
			continue
		}
		findings = append(findings, lintkit.Finding{
			Path:    file,
			Line:    ref.Line,
			Message: "issue or PR reference " + ref.Text + "; state the reason instead",
		})
	}
	return findings
}
