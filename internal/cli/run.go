package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rvenutolo/repo-toolbelt/internal/config"
	"github.com/rvenutolo/repo-toolbelt/internal/manifest"
	"github.com/rvenutolo/repo-toolbelt/internal/runner"
)

const (
	hookPreCommit = "pre-commit"
	hookPrePush   = "pre-push"
	hookCI        = "ci"

	defaultLintsDir = "/opt/toolbelt/lints"
	speedSlow       = "slow"
)

// Finding lines GitHub can annotate: "<path>:<line>: <message>" and
// "<path>: <message>". The lazy path takes the first separator, so a colon in
// the message stays in the message.
var (
	lineFinding = regexp.MustCompile(`^(.+?):([0-9]+): (.+)$`)
	fileFinding = regexp.MustCompile(`^(.+?): (.+)$`)
)

// runFlags are run's flag values.
type runFlags struct {
	hook string
	base string
}

// target is one TOOLBELT_BASE and TOOLBELT_RANGE pair the lints judge.
type target struct {
	base, rng string
}

// newRunCmd runs the named lints concurrently against the checkout.
func (r Runner) newRunCmd(getenv func(string) string) *cobra.Command {
	var flags runFlags
	cmd := &cobra.Command{
		Use:   "run --hook pre-commit|pre-push|ci [--base <rev>] [<lint>...]",
		Short: "Run lints against the checkout",
		Long: "Run lints concurrently against the checkout and print each one's output as\n" +
			"one block, in the order named, then a summary.\n\n" +
			"With no lints as arguments, reads them from TOOLBELT_LINTS: one per line,\n" +
			"blank lines and # comments ignored. Lint executables are found under\n" +
			"TOOLBELT_LINTS_DIR (default " + defaultLintsDir + ").\n\n" +
			"Exit codes: 0 every lint passed, 1 a lint reported findings or the config\n" +
			"file has problems, 2 a lint errored or a usage or setup error.",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := r.run(cmd.Context(), cmd, args, flags, getenv)
			// An interrupted run fails in whatever step it reached; report it
			// as the interruption.
			if err != nil && cmd.Context().Err() != nil {
				return fmt.Errorf("run interrupted: %w", cmd.Context().Err())
			}
			return err
		},
	}
	cmd.Flags().StringVar(&flags.hook, "hook", "", "what is running the lints: pre-commit, pre-push or ci (required)")
	cmd.Flags().StringVar(&flags.base, "base", "", "the commit diff-aware lints compare against (TOOLBELT_BASE)")
	return cmd
}

func (r Runner) run(ctx context.Context, cmd *cobra.Command, args []string, flags runFlags, getenv func(string) string) error {
	switch flags.hook {
	case hookPreCommit, hookPrePush, hookCI:
	case "":
		return usagef("--hook is required: pre-commit, pre-push or ci")
	default:
		return usagef("unknown hook %q, want pre-commit, pre-push or ci", flags.hook)
	}
	lints, err := r.selectLints(args, getenv)
	if err != nil {
		return err
	}
	targets := resolveTargets(flags)

	root, err := r.repoRoot(ctx)
	if err != nil {
		return err
	}
	stdout, stderr := cmd.OutOrStdout(), cmd.ErrOrStderr()

	known := make([]string, len(r.Lints))
	for i, lint := range r.Lints {
		known[i] = lint.Name
	}
	cfg, problems, err := config.Load(root, known)
	if err != nil {
		return setupf("%w", err)
	}
	if len(problems) > 0 {
		for _, problem := range problems {
			if _, err := fmt.Fprintf(stdout, "%s: %s\n", config.FileName, problem); err != nil {
				return fmt.Errorf("write config problems: %w", err)
			}
		}
		return codeError{code: exitError, err: fmt.Errorf("%s has %d problem(s)", config.FileName, len(problems))}
	}

	if err := warn(ctx, root, flags.hook, lints, stderr); err != nil {
		return err
	}

	sections := make(map[string][]byte, len(lints))
	for _, lint := range lints {
		sections[lint.Name] = cfg.Section(lint.Name)
	}
	lintsDir := getenv("TOOLBELT_LINTS_DIR")
	if lintsDir == "" {
		lintsDir = defaultLintsDir
	}

	var tally tally
	for _, t := range targets {
		results := runner.Run(ctx, lints, runner.Options{
			LintsDir: lintsDir,
			RepoRoot: root,
			Hook:     flags.hook,
			Base:     t.base,
			Range:    t.rng,
			Sections: sections,
		})
		for _, result := range results {
			tally.add(result.Exit)
			if err := printResult(stdout, stderr, root, flags.hook, result); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintf(stdout, "toolbelt: %d passed, %d failed, %d errored\n", tally.passed, tally.failed, tally.errored); err != nil {
		return fmt.Errorf("write summary: %w", err)
	}
	return tally.err()
}

// resolveTargets turns the revision flags into the targets the lints judge.
// Each target is one full run of the lints.
//
// TODO: derive base and range per hook from --head, --default-branch, --remote
// and pre-push's stdin; until then --base passes through with an empty range.
func resolveTargets(flags runFlags) []target {
	return []target{{base: flags.base}}
}

// repoRoot is the top of the git checkout containing r.Dir.
func (r Runner) repoRoot(ctx context.Context) (string, error) {
	out, err := gitOutput(ctx, r.Dir, "rev-parse", "--show-toplevel")
	if err == nil {
		return strings.TrimSuffix(string(out), "\n"), nil
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return "", setupf("run git: %w", err)
	}
	msg := firstLine(exitErr.Stderr)
	if bytes.Contains(exitErr.Stderr, []byte("dubious ownership")) {
		return "", setupf("the checkout must be owned by the user running the container; git says: %s", msg)
	}
	return "", setupf("lints run in a git checkout; git says: %s", msg)
}

// warn prints the hook's warnings to stderr: slow lints named in pre-commit,
// and edits the lints will see that are not part of the commit or push.
func warn(ctx context.Context, root, hook string, lints []manifest.Lint, stderr io.Writer) error {
	var lines []string
	switch hook {
	case hookPreCommit:
		for _, lint := range lints {
			if lint.Speed == speedSlow {
				lines = append(lines, fmt.Sprintf("toolbelt: %s is a slow lint; consider running it from pre-push", lint.Name))
			}
		}
		out, err := gitOutput(ctx, root, "diff", "--name-only", "-z")
		if err != nil {
			return setupf("list unstaged changes: %w", err)
		}
		if n := bytes.Count(out, []byte{0}); n > 0 {
			lines = append(lines, fmt.Sprintf("toolbelt: %d tracked file(s) have unstaged changes; lints saw the working tree", n))
		}
	case hookPrePush:
		out, err := gitOutput(ctx, root, "status", "--porcelain")
		if err != nil {
			return setupf("list uncommitted changes: %w", err)
		}
		if len(out) > 0 {
			lines = append(lines, "toolbelt: the working tree has uncommitted changes; lints saw them")
		}
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(stderr, line); err != nil {
			return fmt.Errorf("write warning: %w", err)
		}
	}
	return nil
}

// printResult writes one lint's block: a header with its verdict and its
// stdout on stdout, its stderr on stderr, then in ci its annotations.
func printResult(stdout, stderr io.Writer, root, hook string, result runner.Result) error {
	var block bytes.Buffer
	fmt.Fprintf(&block, "== %s: %s ==\n", result.Lint, verdict(result.Exit))
	block.Write(result.Output)
	if len(result.Output) > 0 && !bytes.HasSuffix(result.Output, []byte("\n")) {
		block.WriteByte('\n')
	}
	if _, err := stdout.Write(block.Bytes()); err != nil {
		return fmt.Errorf("write output of %s: %w", result.Lint, err)
	}
	if _, err := stderr.Write(result.Stderr); err != nil {
		return fmt.Errorf("write stderr of %s: %w", result.Lint, err)
	}
	if hook != hookCI || result.Exit != runner.ExitFindings {
		return nil
	}
	return annotate(stdout, root, result.Output)
}

// annotate writes a GitHub ::error command for each finding line that names a
// line, and for each that names only a path when that path exists in the
// checkout.
func annotate(w io.Writer, root string, output []byte) error {
	var annotations bytes.Buffer
	for raw := range bytes.Lines(output) {
		line := strings.TrimSuffix(string(raw), "\n")
		if m := lineFinding.FindStringSubmatch(line); m != nil {
			fmt.Fprintf(&annotations, "::error file=%s,line=%s::%s\n", escapeProperty(m[1]), m[2], escapeData(m[3]))
			continue
		}
		if m := fileFinding.FindStringSubmatch(line); m != nil && inCheckout(root, m[1]) {
			fmt.Fprintf(&annotations, "::error file=%s::%s\n", escapeProperty(m[1]), escapeData(m[2]))
		}
	}
	if _, err := w.Write(annotations.Bytes()); err != nil {
		return fmt.Errorf("write annotations: %w", err)
	}
	return nil
}

// inCheckout reports whether path is a repo-relative path to something that
// exists in the checkout.
func inCheckout(root, path string) bool {
	if !filepath.IsLocal(path) {
		return false
	}
	_, err := os.Lstat(filepath.Join(root, path))
	return err == nil
}

// escapeData escapes a workflow command's message.
func escapeData(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(s)
}

// escapeProperty escapes a workflow command's property value.
func escapeProperty(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C").Replace(s)
}

func verdict(exit int) string {
	switch exit {
	case runner.ExitPass:
		return "pass"
	case runner.ExitFindings:
		return "fail"
	default:
		return "error"
	}
}

// tally counts verdicts across a run.
type tally struct {
	passed, failed, errored int
}

func (t *tally) add(exit int) {
	switch exit {
	case runner.ExitPass:
		t.passed++
	case runner.ExitFindings:
		t.failed++
	default:
		t.errored++
	}
}

// err is the run's outcome: any errored lint makes it exit 2, otherwise any
// failed lint makes it exit 1.
func (t *tally) err() error {
	switch {
	case t.errored > 0:
		return codeError{code: exitSetup, err: fmt.Errorf("%d lint(s) errored, %d failed", t.errored, t.failed)}
	case t.failed > 0:
		return codeError{code: exitError, err: fmt.Errorf("%d lint(s) failed", t.failed)}
	default:
		return nil
	}
}

// gitOutput runs git in dir and returns its stdout. A failed git's stderr is
// in the *exec.ExitError.
func gitOutput(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // fixed program; the arguments are this package's own
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

func firstLine(b []byte) string {
	line, _, _ := strings.Cut(strings.TrimSpace(string(b)), "\n")
	return line
}
