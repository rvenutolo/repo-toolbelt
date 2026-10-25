// Package coveragedrop is the check-coverage-drop lint: the count of
// uncovered statements in each unit never rises above the committed baseline,
// and a commit that raises a baseline line says why in a Coverage-Drop
// trailer.
package coveragedrop

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/rvenutolo/repo-toolbelt/internal/coverage"
	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
)

// Check runs both checks. The tree check reads the reports the section names
// and reports every unit whose uncovered count is above its baseline, a unit
// missing from the baseline counting as 0; vendored files are not counted, so
// lc.Root must be a git checkout. The history check, run only when
// lc.Range names commits, reports every non-merge commit in the range that
// raises a baseline line without a Coverage-Drop trailer for that unit
// giving a reason.
func Check(ctx context.Context, lc lintkit.Context) ([]lintkit.Finding, error) {
	section, problems, err := coverage.ParseSection(lc.Section)
	if err != nil {
		return nil, err
	}
	if len(problems) > 0 {
		return problems, nil
	}
	findings, err := checkTree(ctx, lc.Root, section)
	if err != nil {
		return nil, err
	}
	if lc.Range == "" {
		return findings, nil
	}
	history, err := checkHistory(ctx, lc.Root, section.Baseline, lc.Range)
	if err != nil {
		return nil, err
	}
	return append(findings, history...), nil
}

func checkTree(ctx context.Context, root string, section coverage.Section) ([]lintkit.Finding, error) {
	reports, findings, err := coverage.ReadReports(root, section)
	if err != nil || len(findings) > 0 {
		return findings, err
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open repository root: %w", err)
	}
	defer dir.Close()
	reports, err = coverage.WithoutVendored(ctx, root, reports)
	if err != nil {
		return nil, err
	}
	current := coverage.Tally(section.Reports, reports, coverage.SourceMarkers(dir))
	baseline, err := readBaseline(dir, section.Baseline)
	if err != nil {
		return nil, err
	}
	// By directory first, the order the findings are printed in.
	units := slices.SortedFunc(maps.Keys(current), func(x, y coverage.Unit) int {
		return cmp.Or(strings.Compare(x.Name, y.Name), strings.Compare(x.Format, y.Format))
	})
	for _, unit := range units {
		if n, held := current[unit], baseline[unit]; n > held {
			findings = append(findings, lintkit.Finding{
				Path:    unit.Name,
				Message: fmt.Sprintf("%d uncovered %s, baseline %d", n, measure(unit.Format, n), held),
			})
		}
	}
	return findings, nil
}

// readBaseline reads the baseline in the working tree. No file is an empty
// baseline, so every unit is held to 0.
func readBaseline(dir *os.Root, path string) (coverage.Baseline, error) {
	f, err := dir.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return coverage.Baseline{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	baseline, err := coverage.ParseBaseline(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return baseline, nil
}

func checkHistory(ctx context.Context, root, path, rangeSpec string) ([]lintkit.Finding, error) {
	revs, err := gitOutput(ctx, root, "rev-list", "--no-merges", "--parents", "--end-of-options", rangeSpec)
	if err != nil {
		return nil, err
	}
	var findings []lintkit.Finding
	for line := range strings.Lines(revs) {
		commits := strings.Fields(line)
		// --no-merges leaves at most one parent, and none for a root
		// commit, which has no earlier baseline to raise.
		if len(commits) != 2 {
			continue
		}
		commitFindings, err := judgeCommit(ctx, root, path, commits[0], commits[1])
		if err != nil {
			return nil, err
		}
		findings = append(findings, commitFindings...)
	}
	return findings, nil
}

// judgeCommit reports each unit commit raises above parent's baseline with no
// trailer justifying it. A commit that adds or deletes the baseline is not
// judged.
func judgeCommit(ctx context.Context, root, path, commit, parent string) ([]lintkit.Finding, error) {
	after, ok, err := baselineAt(ctx, root, commit, path)
	if err != nil || !ok {
		return nil, err
	}
	before, ok, err := baselineAt(ctx, root, parent, path)
	if err != nil || !ok {
		return nil, err
	}
	var raised []coverage.Unit
	for _, unit := range sortedUnits(after) {
		if after[unit] > before[unit] {
			raised = append(raised, unit)
		}
	}
	if len(raised) == 0 {
		return nil, nil
	}
	out, err := gitOutput(ctx, root, "log", "-1", "--no-show-signature", "--format=%h%x00%B", commit)
	if err != nil {
		return nil, err
	}
	short, message, _ := strings.Cut(out, "\x00")
	trailers, err := DropTrailers(ctx, root, message)
	if err != nil {
		return nil, err
	}
	reasons := map[string]bool{}
	for _, tr := range trailers {
		reasons[tr.Unit] = reasons[tr.Unit] || tr.Reason != ""
	}
	var findings []lintkit.Finding
	for _, unit := range raised {
		justified, named := reasons[unit.Name]
		if justified {
			continue
		}
		message := fmt.Sprintf("%s raises %s %s %d -> %d uncovered without a \"Coverage-Drop: %s - <reason>\" trailer in its last paragraph",
			short, unit.Format, unit.Name, before[unit], after[unit], unit.Name)
		if named {
			message += " (the trailer gives no reason)"
		}
		findings = append(findings, lintkit.Finding{Path: path, Message: message})
	}
	return findings, nil
}

// baselineAt reads the baseline in commit's tree; false when it has none.
func baselineAt(ctx context.Context, root, commit, path string) (coverage.Baseline, bool, error) {
	// ls-tree prints nothing for a missing path and fails only when git
	// does, so a failure can never read as "no baseline".
	entry, err := gitOutput(ctx, root, "ls-tree", "-z", commit, "--", path)
	if err != nil {
		return nil, false, err
	}
	meta, _, _ := strings.Cut(entry, "\t")
	fields := strings.Fields(meta)
	if len(fields) != 3 || fields[1] != "blob" {
		return nil, false, nil
	}
	content, err := gitOutput(ctx, root, "cat-file", "blob", fields[2])
	if err != nil {
		return nil, false, err
	}
	baseline, err := coverage.ParseBaseline(strings.NewReader(content))
	if err != nil {
		return nil, false, fmt.Errorf("%s:%s: %w", commit, path, err)
	}
	return baseline, true, nil
}

func sortedUnits(b coverage.Baseline) []coverage.Unit {
	return slices.SortedFunc(maps.Keys(b), func(x, y coverage.Unit) int {
		return cmp.Or(strings.Compare(x.Format, y.Format), strings.Compare(x.Name, y.Name))
	})
}

// measure names what a format counts, in the singular for a count of 1.
func measure(format string, n int) string {
	noun := "line"
	if format == coverage.FormatGo {
		noun = "statement"
	}
	if n == 1 {
		return noun
	}
	return noun + "s"
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	return gitInput(ctx, dir, "", args...)
}

// gitInput runs git in dir, in the C locale, with stdin as its input.
func gitInput(ctx context.Context, dir, stdin string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // fixed program; revisions follow --end-of-options or are object names git printed
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANGUAGE=")
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg, _, _ := strings.Cut(strings.TrimSpace(stderr.String()), "\n")
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
	}
	return stdout.String(), nil
}
