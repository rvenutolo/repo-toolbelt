// Package patchcov is the check-patch-coverage lint: every code block (Go) or
// line (Cobertura) that a change since the base touches is covered by the
// tests, or carries a coverage-exempt marker giving a reason, and no marker
// in the repository is malformed, misplaced or stale.
package patchcov

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"slices"
	"strings"
	"syscall"

	"github.com/rvenutolo/repo-toolbelt/internal/coverage"
	"github.com/rvenutolo/repo-toolbelt/internal/coveragedrop"
	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
)

// Check reads the reports the section names and the working tree's changes
// since lc.Base, and reports each changed block or line no test ran and no
// marker exempts. Markers are judged in every tracked file that can hold
// them, changed or not: `// coverage-exempt: <reason>` line comments in .go
// files, and `# coverage-exempt: <reason>` comments in files a Cobertura
// report scores. One with no reason, one on no measured line, and one whose
// code is all covered (stale) are findings. An empty base is a setup
// failure: the runner sets it for every hook.
func Check(ctx context.Context, lc lintkit.Context) ([]lintkit.Finding, error) {
	if lc.Base == "" {
		return nil, errors.New("TOOLBELT_BASE is empty; this lint compares the working tree against it")
	}
	section, problems, err := coverage.ParseSection(lc.Section)
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
	reports, findings, err := coveragedrop.ReadReports(lc.Root, section)
	if err != nil || len(findings) > 0 {
		return findings, err
	}
	m := measure(section.Reports, reports)
	if findings := m.missingFiles(root); len(findings) > 0 {
		return findings, nil
	}
	markers, err := m.scanMarkers(ctx, lc.Root, root)
	if err != nil {
		return nil, err
	}
	if _, err := gitOutput(ctx, lc.Root, "rev-parse", "--verify", "--quiet", "--end-of-options", lc.Base+"^{commit}"); err != nil {
		return nil, fmt.Errorf("TOOLBELT_BASE %s is not a commit", lc.Base)
	}
	changed, err := ChangedLines(ctx, lc.Root, lc.Base, m.files)
	if err != nil {
		return nil, err
	}
	findings = m.judge(changed, markers)
	slices.SortStableFunc(findings, func(a, b lintkit.Finding) int {
		return cmp.Or(strings.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line))
	})
	return findings, nil
}

// measured is what the reports score, merged per format.
type measured struct {
	// blocks are every format's blocks, by repo-relative file.
	blocks map[string][]coverage.Block
	// files holds every file a report scores; cobertura those a Cobertura
	// report scores.
	files, cobertura map[string]bool
}

func measure(specs []coverage.ReportSpec, reports []coverage.Report) measured {
	byFormat := map[string][]coverage.Report{}
	for i, spec := range specs {
		byFormat[spec.Format] = append(byFormat[spec.Format], reports[i])
	}
	m := measured{blocks: map[string][]coverage.Block{}, files: map[string]bool{}, cobertura: map[string]bool{}}
	for format, rs := range byFormat {
		for _, b := range coverage.Merge(rs...).Blocks {
			if b.Statements == 0 {
				// An empty function body: nothing in it can run.
				continue
			}
			m.blocks[b.Path] = append(m.blocks[b.Path], b)
			m.files[b.Path] = true
			if format == coverage.FormatCobertura {
				m.cobertura[b.Path] = true
			}
		}
	}
	return m
}

// missingFiles reports each scored file that is not in the working tree: the
// reports predate a rename or deletion, and judging them would be guesswork.
func (m measured) missingFiles(root *os.Root) []lintkit.Finding {
	var findings []lintkit.Finding
	for _, file := range slices.Sorted(maps.Keys(m.files)) {
		if _, err := root.Stat(file); errors.Is(err, fs.ErrNotExist) {
			findings = append(findings, lintkit.Finding{Path: file, Message: "a coverage report names this file, which does not exist; rerun the tests"})
		}
	}
	return findings
}

// fileMarker is a marker in a file.
type fileMarker struct {
	path string
	Marker
}

// scanMarkers returns the markers in every tracked file that can hold them.
func (m measured) scanMarkers(ctx context.Context, dir string, root *os.Root) ([]fileMarker, error) {
	tracked, err := gitOutput(ctx, dir, "ls-files", "-z", "--")
	if err != nil {
		return nil, err
	}
	var markers []fileMarker
	for file := range strings.SplitSeq(strings.TrimSuffix(tracked, "\x00"), "\x00") {
		var scan func([]byte) []Marker
		switch {
		case path.Ext(file) == ".go":
			scan = GoMarkers
		case m.cobertura[file]:
			scan = HashMarkers
		default:
			continue
		}
		src, ok, err := readRegular(root, file)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		for _, marker := range scan(src) {
			markers = append(markers, fileMarker{path: file, Marker: marker})
		}
	}
	return markers, nil
}

// readRegular reads a tracked file; false for one missing from the working
// tree (its directory may now be a file) or that is not a regular file, such as a symlink, whose target is
// scanned under its own name if it is tracked.
func readRegular(root *os.Root, file string) ([]byte, bool, error) {
	info, err := root.Lstat(file)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("stat %s: %w", file, err)
	}
	if !info.Mode().IsRegular() {
		return nil, false, nil
	}
	src, err := root.ReadFile(file)
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", file, err)
	}
	return src, true, nil
}

// judge reports the markers' problems, then each changed, uncovered block no
// valid marker exempts.
func (m measured) judge(changed map[string][]Span, markers []fileMarker) []lintkit.Finding {
	var findings []lintkit.Finding
	exempt := map[*coverage.Block]bool{}
	for _, marker := range markers {
		inside, uncovered := false, false
		blocks := m.blocks[marker.path]
		for i := range blocks {
			b := &blocks[i]
			if marker.Line < b.StartLine || marker.Line > b.EndLine {
				continue
			}
			inside = true
			uncovered = uncovered || !b.Covered
			if marker.Reason != "" {
				exempt[b] = true
			}
		}
		var problem string
		switch {
		case marker.Reason == "":
			problem = "coverage-exempt marker gives no reason"
		case !inside:
			problem = "coverage-exempt marker is not inside any measured block"
		case !uncovered:
			problem = "coverage-exempt marker is stale: the code it marks is covered"
		default:
			continue
		}
		findings = append(findings, lintkit.Finding{Path: marker.path, Line: marker.Line, Message: problem})
	}
	for _, file := range slices.Sorted(maps.Keys(changed)) {
		blocks := m.blocks[file]
		for i := range blocks {
			b := &blocks[i]
			if b.Covered || exempt[b] || !touches(changed[file], *b) {
				continue
			}
			findings = append(findings, blockFinding(*b, m.cobertura[file]))
		}
	}
	return findings
}

func touches(spans []Span, b coverage.Block) bool {
	return slices.ContainsFunc(spans, func(s Span) bool { return s.Start <= b.EndLine && s.End >= b.StartLine })
}

func blockFinding(b coverage.Block, cobertura bool) lintkit.Finding {
	if cobertura {
		return lintkit.Finding{Path: b.Path, Line: b.StartLine, Message: "changed line not covered"}
	}
	noun := "statements"
	if b.Statements == 1 {
		noun = "statement"
	}
	return lintkit.Finding{
		Path: b.Path, Line: b.StartLine, EndLine: b.EndLine,
		Message: fmt.Sprintf("changed block not covered (%d %s)", b.Statements, noun),
	}
}
