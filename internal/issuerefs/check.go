package issuerefs

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"

	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
)

// Check is the check-issue-numbers lint: it validates the section, then
// reports every reference in the tracked text files that no exemption covers,
// and every exemption that covered nothing.
func Check(ctx lintkit.Context) ([]lintkit.Finding, error) {
	exemptions, problems, err := ParseSection(ctx.Section)
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		return problems, nil
	}
	root, err := os.OpenRoot(ctx.Root)
	if err != nil {
		return nil, fmt.Errorf("open repository root: %w", err)
	}
	defer root.Close()
	files, err := textFiles(ctx.Root)
	if err != nil {
		return nil, err
	}
	var findings []lintkit.Finding
	for _, file := range files {
		content, ok, err := readRegular(root, file)
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
func textFiles(dir string) ([]string, error) {
	// An empty pattern matches every line, so this lists every tracked text
	// file with any content; an empty file holds no reference to report.
	cmd := exec.Command("git", //nolint:noctx // a lint is cancelled by the runner signalling its whole process group, git included
		"grep", "-I", "-z", "--files-with-matches", "--color=never", "-e", "", "--")
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

// readRegular reads a tracked file under root. It reports false for anything
// but a regular file: git records a symlink's target as its content, and the
// file it points to is scanned under its own name if it is tracked.
func readRegular(root *os.Root, file string) ([]byte, bool, error) {
	info, err := root.Lstat(file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("stat %s: %w", file, err)
	}
	if !info.Mode().IsRegular() {
		return nil, false, nil
	}
	content, err := root.ReadFile(file)
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", file, err)
	}
	return content, true, nil
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
