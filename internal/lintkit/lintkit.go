// Package lintkit is the Go side of the lint calling convention: it reads a
// lint's inputs from the environment, runs its check, and prints the findings
// and exit status every lint shares, whatever its language.
package lintkit

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"
)

// Exit statuses of the calling convention.
const (
	exitPass     = 0
	exitFindings = 1
	exitSetup    = 2
)

// configFile is the consumer config file, named in section errors.
const configFile = ".repo-toolbelt.yml"

// Context is what a check is told about its run.
type Context struct {
	// Root is the repository root, the lint's working directory.
	Root string
	// Section is the lint's config section as JSON: the JSON null when the
	// config file is absent or has no section for the lint.
	Section []byte
	// Hook is TOOLBELT_HOOK: pre-commit, pre-push or ci.
	Hook string
	// Base is TOOLBELT_BASE, the commit a diff-aware lint compares against;
	// it may be empty.
	Base string
	// Range is TOOLBELT_RANGE, the commits a history-aware lint judges in git
	// rev-list syntax; it may be empty.
	Range string
}

// Finding is one problem a check reports.
type Finding struct {
	// Path is the repo-relative file the finding is in; empty for a finding
	// with no location.
	Path string
	// Line is the 1-based line in Path; 0 for none.
	Line int
	// EndLine is the last line of a finding that spans lines from Line,
	// printed as `<path>:<line>-<end>:`; ignored unless it is past Line.
	EndLine int
	// Message says what is wrong.
	Message string
	// section marks an error in the lint's config section, whose message Run
	// prefixes with the lint's name.
	section bool
}

// Check is a lint's check. ctx is cancelled when the lint is interrupted. A
// returned error is a setup failure, not a finding, and any findings returned
// with it are dropped.
type Check func(ctx context.Context, lc Context) ([]Finding, error)

// Env is the process a lint runs in.
type Env struct {
	// Args are the command-line arguments after the program name.
	Args []string
	// Getenv reads an environment variable.
	Getenv func(string) string
	// Dir is the working directory, the repository root.
	Dir string
	// Stdout receives the findings, Stderr the diagnostics.
	Stdout, Stderr io.Writer
}

// SectionError is a finding in the lint's own section of the config file,
// printed as `.repo-toolbelt.yml: <lint>: <where>: <problem>`, or without the
// `<where>: ` when where is empty.
func SectionError(where, problem string) Finding {
	message := problem
	if where != "" {
		message = where + ": " + problem
	}
	return Finding{Path: configFile, Message: message, section: true}
}

// Main runs check as the lint called name in this process and returns the
// exit status for os.Exit. noun names what the lint counts, in the singular,
// for the count line. SIGINT or SIGTERM cancels the check's context.
func Main(name, noun string, check Check) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	context.AfterFunc(ctx, stop) // the first signal cancels ctx; a second one kills the process
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
		return exitSetup
	}
	return Run(ctx, name, noun, check, Env{Args: os.Args[1:], Getenv: os.Getenv, Dir: dir, Stdout: os.Stdout, Stderr: os.Stderr})
}

// Run runs check as the lint called name in env and returns its exit status:
// 0 with no findings; 1 with findings, printed one per line sorted by path
// then line and followed by `<N> <noun>(s)`; 2 on a setup failure, an
// interruption included, printed to stderr as `<name>: <reason>`. `--help`
// prints the usage and returns 0.
func Run(ctx context.Context, name, noun string, check Check, env Env) int {
	lc, help, err := setup(name, env)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%s: %v\n", name, err)
		return exitSetup
	}
	if help {
		if _, err := fmt.Fprint(env.Stdout, usage(name)); err != nil {
			fmt.Fprintf(env.Stderr, "%s: write usage: %v\n", name, err)
			return exitSetup
		}
		return exitPass
	}
	findings, err := check(ctx, lc)
	if err != nil {
		fmt.Fprintf(env.Stderr, "%s: %v\n", name, err)
		return exitSetup
	}
	if len(findings) == 0 {
		return exitPass
	}
	if err := report(env.Stdout, name, noun, findings); err != nil {
		fmt.Fprintf(env.Stderr, "%s: write findings: %v\n", name, err)
		return exitSetup
	}
	return exitFindings
}

// setup parses the arguments and reads the inputs from the environment. It
// reports help when the arguments ask for the usage.
func setup(name string, env Env) (Context, bool, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(env.Args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return Context{}, true, nil
		}
		return Context{}, false, fmt.Errorf("%w; usage: %s [--help]", err, name)
	}
	if fs.NArg() > 0 {
		return Context{}, false, fmt.Errorf("unexpected argument %q; usage: %s [--help]", fs.Arg(0), name)
	}
	sectionFile := env.Getenv("TOOLBELT_SECTION")
	if sectionFile == "" {
		return Context{}, false, errors.New("TOOLBELT_SECTION is not set")
	}
	section, err := os.ReadFile(sectionFile) //nolint:gosec // the runner names the file; reading it is the convention
	if err != nil {
		return Context{}, false, fmt.Errorf("read config section: %w", err)
	}
	return Context{
		Root:    env.Dir,
		Section: section,
		Hook:    env.Getenv("TOOLBELT_HOOK"),
		Base:    env.Getenv("TOOLBELT_BASE"),
		Range:   env.Getenv("TOOLBELT_RANGE"),
	}, false, nil
}

func usage(name string) string {
	return "Usage: " + name + "\n\n" +
		"Runs as a repo-toolbelt lint, with the repository root as the working\n" +
		"directory and no arguments other than --help. Reads:\n" +
		"  TOOLBELT_SECTION  path to a JSON file holding this lint's config section (required)\n" +
		"  TOOLBELT_HOOK     pre-commit, pre-push or ci\n" +
		"  TOOLBELT_BASE     the commit a diff-aware lint compares against; may be empty\n" +
		"  TOOLBELT_RANGE    the commits a history-aware lint judges; may be empty\n\n" +
		"Prints one finding per line on stdout, then a count.\n\n" +
		"Exit codes: 0 no findings, 1 findings, 2 a setup failure.\n"
}

// report prints the findings sorted by path then line, keeping the check's
// order among equals, then the count.
func report(w io.Writer, name, noun string, findings []Finding) error {
	sorted := slices.Clone(findings)
	slices.SortStableFunc(sorted, func(a, b Finding) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return a.Line - b.Line
	})
	for _, f := range sorted {
		message := f.Message
		if f.section {
			message = name + ": " + message
		}
		var line string
		switch {
		case f.Path == "":
			line = printable(message)
		case f.Line == 0:
			line = printable(f.Path) + ": " + printable(message)
		case f.EndLine > f.Line:
			line = printable(f.Path) + ":" + strconv.Itoa(f.Line) + "-" + strconv.Itoa(f.EndLine) + ": " + printable(message)
		default:
			line = printable(f.Path) + ":" + strconv.Itoa(f.Line) + ": " + printable(message)
		}
		if _, err := io.WriteString(w, line+"\n"); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "%d %s(s)\n", len(sorted), noun)
	return err
}

// printable shows s on one line: newline, carriage return and tab as `\n`,
// `\r` and `\t`, and any other control character or invalid UTF-8 byte as
// `?`, so a value from the repo cannot split or rewrite a finding line.
func printable(s string) string {
	var b strings.Builder
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		switch {
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == utf8.RuneError && size == 1, unicode.IsControl(r):
			b.WriteByte('?')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
