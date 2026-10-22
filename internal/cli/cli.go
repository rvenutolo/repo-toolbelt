// Package cli implements the toolbelt command line: subcommand dispatch,
// usage text and exit codes.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/rvenutolo/repo-toolbelt/internal/manifest"
)

// Exit codes. A usage error is any bad argument, flag or lint selection.
const (
	exitOK    = 0
	exitUsage = 2
)

const usageText = `Usage: toolbelt <command> [options]

Commands:
  plan    print the job groups for a set of lints as JSON
  list    print every lint with its speed and description
  help    print this help

Run 'toolbelt <command> --help' for a command's options.

Exit codes: 0 success, 1 findings, 2 usage or setup error.
`

// usageError is the caller's mistake: a bad argument, flag or lint selection.
type usageError struct{ err error }

func (e usageError) Error() string { return e.err.Error() }
func (e usageError) Unwrap() error { return e.err }

func usagef(format string, args ...any) error {
	return usageError{fmt.Errorf(format, args...)}
}

// Runner runs toolbelt commands against a fixed set of lints.
type Runner struct {
	// Lints is the manifest of lints the commands operate on.
	Lints []manifest.Lint
}

// Run runs toolbelt with the embedded manifest and returns the exit code.
func Run(args []string, env func(string) string, stdin io.Reader, stdout, stderr io.Writer) int {
	return Runner{Lints: manifest.Load()}.Run(args, env, stdin, stdout, stderr)
}

// Run executes the subcommand named by args[0] and returns the process exit
// code. It reads the environment only through env and writes only to the given
// streams.
func (r Runner) Run(args []string, env func(string) string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usageText)
		return exitUsage
	}
	var err error
	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usageText)
		return exitOK
	case "plan":
		err = r.plan(args[1:], env, stdout, stderr)
	case "list":
		err = r.list(args[1:], stdout, stderr)
	default:
		err = usagef("unknown subcommand %q", args[0])
	}
	return finish(err, stderr)
}

// finish maps a subcommand's error to an exit code, reporting it on stderr.
func finish(err error, stderr io.Writer) int {
	var usage usageError
	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
		return exitOK
	case errors.As(err, &usage):
		fmt.Fprintf(stderr, "toolbelt: %s\nRun 'toolbelt --help' for usage.\n", usage)
		return exitUsage
	default:
		fmt.Fprintf(stderr, "toolbelt: %s\n", err)
		return 1
	}
}

// newFlagSet returns a FlagSet that reports parse problems through Parse's
// error instead of printing them.
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet("toolbelt "+name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// parseFlags parses args. On -h or --help it prints the synopsis and flag
// defaults to stdout and returns flag.ErrHelp; any other parse failure is a
// usage error.
func parseFlags(fs *flag.FlagSet, args []string, synopsis string, stdout io.Writer) error {
	err := fs.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprintf(stdout, "Usage: toolbelt %s\n\n", synopsis)
		fs.SetOutput(stdout)
		fs.PrintDefaults()
		return flag.ErrHelp
	}
	if err != nil {
		return usageError{err}
	}
	return nil
}
