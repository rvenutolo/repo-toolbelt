package cli

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/spf13/cobra"

	"github.com/rvenutolo/repo-toolbelt/internal/config"
	"github.com/rvenutolo/repo-toolbelt/internal/coverage"
	"github.com/rvenutolo/repo-toolbelt/internal/coveragedrop"
	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
)

// coverageDropLint is the lint whose section names the reports and baseline.
const coverageDropLint = "check-coverage-drop"

// newBaselineCmd rewrites the coverage baseline from the current reports.
func (r Runner) newBaselineCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update-coverage-baseline",
		Short: "Rewrite the coverage baseline from the current coverage reports",
		Long: "Rewrite the coverage baseline from the current coverage reports.\n\n" +
			"Reads the reports and the baseline path from the " + coverageDropLint + " section\n" +
			"of " + config.FileName + ", counts the uncovered statements in each unit, and\n" +
			"writes the baseline, recording falls and rises alike. A rise also needs a\n" +
			"Coverage-Drop trailer on the commit. Run the tests first, so the reports\n" +
			"are current.\n\n" +
			"Exit codes: 0 written, 1 a config problem, 2 no section, a missing or\n" +
			"malformed report, or a write failure.",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := r.repoRoot(cmd.Context())
			if err != nil {
				return err
			}
			written, err := r.updateBaseline(root, cmd.OutOrStdout(), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", written); err != nil {
				return fmt.Errorf("write result: %w", err)
			}
			return nil
		},
	}
}

// updateBaseline writes the baseline under root and returns its path.
func (r Runner) updateBaseline(root string, stdout, stderr io.Writer) (string, error) {
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
	reports, findings, err := coveragedrop.ReadReports(root, section)
	if err != nil {
		return "", setupf("%w", err)
	}
	if len(findings) > 0 {
		if err := printFindings(stderr, "", findings); err != nil {
			return "", err
		}
		return "", setupf("cannot read the coverage reports; run the tests that write them first")
	}
	if err := writeBaseline(root, section.Baseline, coverage.Tally(section.Reports, reports)); err != nil {
		return "", setupf("write %s: %w", section.Baseline, err)
	}
	return section.Baseline, nil
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
func writeBaseline(root, file string, baseline coverage.Baseline) (err error) {
	dir, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer dir.Close()
	if err := dir.MkdirAll(path.Dir(file), 0o755); err != nil {
		return err
	}
	tmp := file + ".tmp-" + rand.Text()
	f, err := dir.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, dir.Remove(tmp))
		}
	}()
	if err := baseline.Write(f); err != nil { // coverage-exempt: writing a new local file fails only on a full or failing disk
		return errors.Join(err, f.Close())
	}
	if err := f.Close(); err != nil { // coverage-exempt: closing a new local file fails only on a full or failing disk
		return err
	}
	return dir.Rename(tmp, file)
}
