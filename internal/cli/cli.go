// Package cli implements the toolbelt command line: subcommands, usage text
// and the mapping from errors to exit codes.
package cli

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"runtime/debug"

	"github.com/rvenutolo/repo-toolbelt/internal/manifest"
)

// Exit codes returned by ExitCode.
const (
	exitError     = 1
	exitUsage     = 2
	exitSetup     = 2
	exitCancelled = 130
)

const rootLong = `toolbelt runs the lints and formatters this image ships.

Exit codes: 0 success, 1 findings, a config problem, a runtime error, a file
fmt --fail-on-change rewrote or a file fmt --staged found partly staged, 2 a
usage or setup error or a lint or formatter that errored, 130 interrupted.`

// usageError is the caller's mistake: a bad argument, flag or lint selection.
type usageError struct{ err error }

func (e usageError) Error() string { return e.err.Error() }
func (e usageError) Unwrap() error { return e.err }

func usagef(format string, args ...any) error {
	return usageError{fmt.Errorf(format, args...)}
}

// codeError is an error that sets its own exit code: a run whose lints
// reported findings or errored, or a setup failure.
type codeError struct {
	code int
	err  error
}

func (e codeError) Error() string { return e.err.Error() }
func (e codeError) Unwrap() error { return e.err }

// setupf is a setup failure: the run cannot start in this checkout.
func setupf(format string, args ...any) error {
	return codeError{code: exitSetup, err: fmt.Errorf(format, args...)}
}

// ExitCode maps an error returned by Run to a process exit code: 0 for nil,
// 130 when interrupted, 2 for a usage error, the code a run's outcome or setup
// failure carries, 1 otherwise.
func ExitCode(err error) int {
	var usage usageError
	var coded codeError
	switch {
	case err == nil:
		return 0
	case errors.Is(err, context.Canceled):
		return exitCancelled
	case errors.As(err, &usage):
		return exitUsage
	case errors.As(err, &coded):
		return coded.code
	default:
		return exitError
	}
}

// Runner runs toolbelt commands against a fixed set of lints.
type Runner struct {
	// Lints is the manifest of lints the commands operate on, the formatters'
	// derived lints included.
	Lints []manifest.Lint
	// Formatters is the manifest of formatters. Each derived lint in Lints
	// names one of them.
	Formatters []manifest.Formatter
	// Version is the string --version prints.
	Version string
	// Dir is the directory run starts in; empty means the process's working
	// directory.
	Dir string
}

// Run runs toolbelt with the embedded manifest. version is what --version
// prints; when it is empty, the module version from the build info is printed,
// or "dev" when the build recorded none. Help and version requests return nil.
// A non-nil error is for the caller to print and pass to ExitCode.
func Run(ctx context.Context, version string, args []string, getenv func(string) string, stdin io.Reader, stdout, stderr io.Writer) error {
	lints, formatters := manifest.LoadAll()
	return Runner{Lints: lints, Formatters: formatters, Version: cmp.Or(version, buildVersion())}.Run(ctx, args, getenv, stdin, stdout, stderr)
}

// Run executes args and reads the environment only through getenv and writes
// only to the given streams.
func (r Runner) Run(ctx context.Context, args []string, getenv func(string) string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := r.newRootCmd(getenv, stdin, stdout, stderr)
	cmd.SetArgs(args)
	return cmd.ExecuteContext(ctx)
}

func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version // coverage-exempt: go test binaries carry no module version
	}
	return "dev"
}
