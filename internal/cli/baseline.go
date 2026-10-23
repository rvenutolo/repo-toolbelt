package cli

import (
	"bytes"
	"cmp"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rvenutolo/repo-toolbelt/internal/config"
	"github.com/rvenutolo/repo-toolbelt/internal/coverage"
	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
)

// coverageDropLint is the lint whose section names the reports and baseline.
const coverageDropLint = "check-coverage-drop"

// newBaselineCmd rewrites the coverage baseline from the current reports.
func (r Runner) newBaselineCmd() *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:   "update-coverage-baseline",
		Short: "Rewrite the coverage baseline from the current coverage reports",
		Long: "Rewrite the coverage baseline from the current coverage reports.\n\n" +
			"Reads the reports and the baseline path from the " + coverageDropLint + " section\n" +
			"of " + config.FileName + ", counts the uncovered statements in each unit, and\n" +
			"writes the baseline, recording falls and rises alike. A rise also needs a\n" +
			"Coverage-Drop trailer on the commit. Run the tests first, so the reports\n" +
			"are current.\n\n" +
			"With --check, writes nothing: it compares the committed baseline (an absent\n" +
			"file counts as empty) with what would be written, and prints one line per\n" +
			"differing unit.\n\n" +
			"Exit codes: 0 written, or with --check the baseline matches; 1 a config\n" +
			"problem, or with --check the baseline differs; 2 no section, a missing or\n" +
			"malformed report, an unreadable baseline, or a write failure.",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := r.repoRoot(cmd.Context())
			if err != nil {
				return err
			}
			written, err := r.updateBaseline(root, check, cmd.OutOrStdout(), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			if check {
				return nil
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", written); err != nil {
				return fmt.Errorf("write result: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "compare the committed baseline with the current reports and write nothing")
	return cmd
}

// updateBaseline writes the baseline under root and returns its path. With
// check set it writes nothing and fails when the committed baseline differs.
func (r Runner) updateBaseline(root string, check bool, stdout, stderr io.Writer) (string, error) {
	cfg, err := r.loadConfig(root, stdout)
	if err != nil {
		return "", err
	}
	raw := cfg.Section(coverageDropLint)
	if string(raw) == "null" {
		return "", setupf("%s has no %s section; it names the coverage reports to read", config.FileName, coverageDropLint)
	}
	section, problems, err := coverage.ParseSection(raw)
	if err != nil { // coverage-exempt: the config loader only hands over sections it parsed, so raw is always JSON
		return "", setupf("%w", err)
	}
	if len(problems) > 0 {
		if err := printFindings(stdout, config.FileName+": "+coverageDropLint+": ", problems); err != nil {
			return "", err
		}
		return "", codeError{code: exitError, err: fmt.Errorf("the %s section has %d problem(s)", coverageDropLint, len(problems))}
	}
	reports, findings, err := coverage.ReadReports(root, section)
	if err != nil {
		return "", setupf("%w", err)
	}
	if len(findings) > 0 {
		if err := printFindings(stderr, "", findings); err != nil {
			return "", err
		}
		return "", setupf("cannot read the coverage reports; run the tests that write them first")
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		return "", setupf("open repository root: %w", err)
	}
	defer dir.Close()
	tally := coverage.Tally(section.Reports, reports, coverage.SourceMarkers(dir))
	if check {
		return section.Baseline, checkBaseline(dir, section.Baseline, tally, stdout)
	}
	if err := writeBaseline(dir, section.Baseline, tally); err != nil {
		return "", setupf("write %s: %w", section.Baseline, err)
	}
	return section.Baseline, nil
}

// checkBaseline compares the committed baseline file with current, printing
// one line per differing unit. An absent file counts as empty.
func checkBaseline(dir *os.Root, file string, current coverage.Baseline, stdout io.Writer) error {
	committed := coverage.Baseline{}
	data, err := dir.ReadFile(file)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return setupf("read %s: %w", file, err)
	default:
		if committed, err = coverage.ParseBaseline(bytes.NewReader(data)); err != nil {
			return setupf("%s: %w", file, err)
		}
	}
	units := slices.SortedFunc(maps.Keys(mergeUnits(committed, current)), func(x, y coverage.Unit) int {
		return cmp.Or(strings.Compare(x.Format, y.Format), strings.Compare(x.Name, y.Name))
	})
	var report strings.Builder
	for _, u := range units {
		old, inOld := committed[u]
		now, inNow := current[u]
		if inOld && inNow && old == now {
			continue
		}
		fmt.Fprintf(&report, "%s %s: baseline %s, current %s\n", u.Format, u.Name, countText(old, inOld), countText(now, inNow))
	}
	if report.Len() == 0 {
		return nil
	}
	fmt.Fprintf(&report, "run toolbelt update-coverage-baseline to rewrite %s\n", file)
	if _, err := io.WriteString(stdout, report.String()); err != nil {
		return fmt.Errorf("write difference: %w", err)
	}
	return codeError{code: exitError, err: fmt.Errorf("%s differs from the current coverage", file)}
}

func mergeUnits(a, b coverage.Baseline) map[coverage.Unit]struct{} {
	units := map[coverage.Unit]struct{}{}
	for u := range a {
		units[u] = struct{}{}
	}
	for u := range b {
		units[u] = struct{}{}
	}
	return units
}

// countText renders a unit's count, or none when the baseline lacks the unit.
func countText(count int, present bool) string {
	if !present {
		return "none"
	}
	return strconv.Itoa(count)
}

func printFindings(w io.Writer, prefix string, findings []lintkit.Finding) error {
	for _, f := range findings {
		line := prefix + f.Message
		if f.Path != "" && prefix == "" {
			line = f.Path + ": " + f.Message
		}
		if _, err := fmt.Fprintln(w, line); err != nil {
			return fmt.Errorf("write problems: %w", err)
		}
	}
	return nil
}

// writeBaseline replaces the baseline file through a temporary file beside
// it, so an interrupted write never leaves a truncated baseline.
func writeBaseline(dir *os.Root, file string, baseline coverage.Baseline) error {
	var content bytes.Buffer
	if err := baseline.Write(&content); err != nil {
		return err
	}
	if err := dir.MkdirAll(path.Dir(file), 0o755); err != nil {
		return err
	}
	tmp := file + ".tmp-" + rand.Text()
	if err := dir.WriteFile(tmp, content.Bytes(), 0o644); err != nil {
		_ = dir.Remove(tmp) //nolint:errcheck // best effort: a create that failed leaves nothing to remove
		return err
	}
	if err := dir.Rename(tmp, file); err != nil {
		return errors.Join(err, dir.Remove(tmp))
	}
	return nil
}
