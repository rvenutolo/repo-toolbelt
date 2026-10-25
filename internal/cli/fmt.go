package cli

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/rvenutolo/repo-toolbelt/internal/config"
	"github.com/rvenutolo/repo-toolbelt/internal/manifest"
	"github.com/rvenutolo/repo-toolbelt/internal/runner"
	"github.com/rvenutolo/repo-toolbelt/internal/selection"
)

// accessWrite is access(2)'s W_OK, which package syscall does not export.
const accessWrite = 2

// newFmtCmd rewrites the checkout's files with the named formatters.
func (r Runner) newFmtCmd(getenv func(string) string) *cobra.Command {
	var failOnChange bool
	cmd := &cobra.Command{
		Use:   "fmt [--fail-on-change] [<formatter>...]",
		Short: "Format the checkout's files in place",
		Long: "Format the checkout's files in place with the named formatters, run\n" +
			"concurrently, and print each one's output as one block, in the order named,\n" +
			"then a summary.\n\n" +
			"With no formatters as arguments, reads them from TOOLBELT_FORMATTERS: one per\n" +
			"line, blank lines and # comments ignored. Formatter executables are found\n" +
			"under TOOLBELT_FORMATTERS_DIR (default " + defaultFormattersDir + "). A formatter\n" +
			"reads the section of " + config.FileName + " named after its check,\n" +
			"check-<name>-formatted.\n\n" +
			"fmt refuses a checkout it cannot write to. Each block lists the files its\n" +
			"formatter rewrote as \"<path>: formatted\", from hashes of every selected file\n" +
			"taken before and after; nothing is staged.\n\n" +
			"Each block's header gives the formatter's verdict:\n" +
			"pass (exit 0, nothing changed), formatted (exit 0, files changed),\n" +
			"fail (exit 1), error (exit 2).\n\n" +
			"Exit codes: 0 every formatter passed or formatted its files;\n" +
			"1 a formatter reported a finding (an unparseable file, a bad config section),\n" +
			"the config file has problems, or with --fail-on-change a file was rewritten;\n" +
			"2 a formatter errored, or a usage or setup error (a read-only checkout among\n" +
			"them).",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := r.format(cmd.Context(), cmd, args, failOnChange, getenv)
			// An interrupted run fails in whatever step it reached; report it
			// as the interruption.
			if err != nil && cmd.Context().Err() != nil {
				return fmt.Errorf("fmt interrupted: %w", cmd.Context().Err())
			}
			return err
		},
	}
	cmd.Flags().BoolVar(&failOnChange, "fail-on-change", false, "exit 1 when any file was rewritten")
	return cmd
}

func (r Runner) format(ctx context.Context, cmd *cobra.Command, args []string, failOnChange bool, getenv func(string) string) error {
	formatters, err := selection.ParseFormatters(args, getenv("TOOLBELT_FORMATTERS"), r.Formatters)
	if err != nil {
		return usageError{err}
	}
	root, err := r.repoRoot(ctx)
	if err != nil {
		return err
	}
	// access(2) reports EROFS for a read-only bind mount even to the owner,
	// and writes no probe file into the checkout.
	if err := syscall.Access(root, accessWrite); err != nil {
		return readOnlyError(root, formatters, err)
	}
	stdout, stderr := cmd.OutOrStdout(), cmd.ErrOrStderr()

	cfg, err := r.loadConfig(root, stdout)
	if err != nil {
		return err
	}
	lints, err := r.derivedLints(formatters)
	if err != nil {
		return err
	}
	plan, err := r.prepareFormatters(ctx, root, cfg, lints)
	if err != nil {
		return err
	}

	tree, err := os.OpenRoot(root)
	if err != nil {
		return setupf("open the repo root: %w", err) // coverage-exempt: the root was just found by git and passed access(2)
	}
	defer tree.Close()
	before := make(map[string][sha256.Size]byte)
	for _, files := range plan.files {
		for _, file := range files {
			sum, err := hashFile(tree, file)
			if err != nil {
				return setupf("hash %s before formatting: %w", file, err)
			}
			before[file] = sum
		}
	}

	sections := make(map[string][]byte, len(lints))
	for _, lint := range lints {
		sections[lint.Name] = cfg.Section(lint.Name)
	}
	results := plan.run(ctx, lints, runner.Options{
		// An empty value takes the default: an empty directory would resolve
		// against the checkout.
		FormattersDir: cmp.Or(getenv("TOOLBELT_FORMATTERS_DIR"), defaultFormattersDir),
		RepoRoot:      root,
		Sections:      sections,
		Mode:          "write",
	})

	var tally fmtTally
	for i, result := range results {
		changed := slices.DeleteFunc(slices.Clone(plan.files[result.Lint]), func(file string) bool {
			sum, err := hashFile(tree, file)
			return err == nil && sum == before[file]
		})
		tally.add(result.Exit, len(changed))
		if err := printFormatted(stdout, stderr, formatters[i].Name, changed, result); err != nil {
			return err
		}
	}
	summary := fmt.Sprintf("toolbelt: %d file(s) formatted, %d failed, %d errored", tally.formatted, tally.failed, tally.errored)
	if tally.formatted > 0 {
		summary += "; review and stage them"
	}
	if _, err := fmt.Fprintln(stdout, summary); err != nil {
		return fmt.Errorf("write summary: %w", err)
	}
	return tally.err(failOnChange)
}

// readOnlyError is the refusal of a repo root fmt cannot write to. It gives
// the docker run line that mounts the checkout read-write.
func readOnlyError(root string, formatters []manifest.Formatter, err error) error {
	names := make([]string, len(formatters))
	for i, f := range formatters {
		names[i] = f.Name
	}
	return setupf("cannot write to the repo root %s: %w\n"+
		"fmt rewrites files, so the checkout must be mounted read-write:\n"+
		"  root=\"$(git rev-parse --show-toplevel)\"\n"+
		"  common=\"$(git rev-parse --path-format=absolute --git-common-dir)\"\n"+
		"  docker run --rm --network=none --user \"$(id -u):$(id -g)\" \\\n"+
		"    --volume \"${root}:${root}\" --volume \"${common}:${common}:ro\" --workdir \"${root}\" \\\n"+
		"    ghcr.io/rvenutolo/repo-toolbelt:<tag>@sha256:<digest> fmt %s",
		root, err, strings.Join(names, " "))
}

// derivedLints is the lint that checks each formatter's files, from r.Lints.
func (r Runner) derivedLints(formatters []manifest.Formatter) ([]manifest.Lint, error) {
	lints := make([]manifest.Lint, len(formatters))
	for i, f := range formatters {
		j := slices.IndexFunc(r.Lints, func(lint manifest.Lint) bool { return lint.Name == f.LintName() })
		if j < 0 {
			return nil, fmt.Errorf("formatter %q has no lint %q in the manifest", f.Name, f.LintName())
		}
		lints[i] = r.Lints[j]
	}
	return lints, nil
}

// hashFile is the SHA-256 of the content of file, a path under tree.
func hashFile(tree *os.Root, file string) ([sha256.Size]byte, error) {
	f, err := tree.Open(file)
	if err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("read: %w", err) // coverage-exempt: a regular file that opens reads, short of an I/O error tests cannot cause
	}
	return [sha256.Size]byte(h.Sum(nil)), nil
}

// printFormatted writes one formatter's block: a header with its verdict, a
// line per file it rewrote, then its stdout on stdout and its stderr on
// stderr.
func printFormatted(stdout, stderr io.Writer, name string, changed []string, result runner.Result) error {
	var block bytes.Buffer
	fmt.Fprintf(&block, "== %s: %s ==\n", name, fmtVerdict(result.Exit, len(changed)))
	for _, file := range changed {
		fmt.Fprintf(&block, "%s: formatted\n", file)
	}
	writeLines(&block, result.Output)
	if _, err := stdout.Write(block.Bytes()); err != nil {
		return fmt.Errorf("write output of %s: %w", name, err)
	}
	if len(result.Stderr) == 0 {
		return nil
	}
	if _, err := stderr.Write(result.Stderr); err != nil {
		return fmt.Errorf("write stderr of %s: %w", name, err)
	}
	return nil
}

// fmtVerdict is a formatter's verdict: a pass that rewrote files is
// "formatted".
func fmtVerdict(exit, changed int) string {
	if exit == runner.ExitPass && changed > 0 {
		return "formatted"
	}
	return verdict(exit)
}

// fmtTally counts rewritten files and verdicts across a fmt run.
type fmtTally struct {
	formatted, failed, errored int
}

func (t *fmtTally) add(exit, changed int) {
	t.formatted += changed
	switch exit {
	case runner.ExitPass:
	case runner.ExitFindings:
		t.failed++
	default:
		t.errored++
	}
}

// err is the run's outcome: any errored formatter makes it exit 2, otherwise
// any failed formatter, or with failOnChange any rewritten file, exit 1.
func (t *fmtTally) err(failOnChange bool) error {
	switch {
	case t.errored > 0:
		return codeError{code: exitSetup, err: fmt.Errorf("%d formatter(s) errored, %d failed", t.errored, t.failed)}
	case t.failed > 0:
		return codeError{code: exitError, err: fmt.Errorf("%d formatter(s) failed", t.failed)}
	case failOnChange && t.formatted > 0:
		return codeError{code: exitError, err: fmt.Errorf("%d file(s) formatted, and --fail-on-change is set", t.formatted)}
	default:
		return nil
	}
}
