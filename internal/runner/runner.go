// Package runner runs lint executables concurrently, each with its own
// buffered output, following the lint calling convention: the repo root as the
// working directory, no arguments, and its inputs in TOOLBELT_* variables. A
// derived lint runs its formatter's executable instead, which follows the
// formatter calling convention: the same, plus TOOLBELT_MODE and
// TOOLBELT_FILES.
package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rvenutolo/repo-toolbelt/internal/manifest"
)

// Verdict exit codes a Result carries.
const (
	ExitPass     = 0
	ExitFindings = 1
	ExitError    = 2
)

// waitDelay is the grace period: how long a cancelled lint may take to exit
// after SIGTERM, and how long a process a lint started may hold its output
// open after the lint exits, before the run ends.
const waitDelay = 5 * time.Second

// Result is one lint's outcome.
type Result struct {
	// Lint is the lint's name.
	Lint string
	// Exit is the verdict: ExitPass, ExitFindings or ExitError. A lint that
	// exits 3 or above, is killed by a signal or cannot be started is
	// ExitError, with the reason appended to Stderr.
	Exit int
	// Output is everything the lint wrote to stdout, byte for byte.
	Output []byte
	// Stderr is everything the lint wrote to stderr, then any reason the
	// runner adds.
	Stderr []byte
}

// Options says where lints are and what they are told.
type Options struct {
	// LintsDir holds each lint's executable at <LintsDir>/<name>/<name>.
	LintsDir string
	// FormattersDir holds each formatter's executable at
	// <FormattersDir>/<name>/<name>. A lint with a Formatter runs that
	// executable instead of its own.
	FormattersDir string
	// RepoRoot is every lint's working directory.
	RepoRoot string
	// Hook, Base and Range become TOOLBELT_HOOK, TOOLBELT_BASE and
	// TOOLBELT_RANGE.
	Hook, Base, Range string
	// Sections maps a lint's name to its config section as JSON. A lint with
	// no entry gets the JSON null.
	Sections map[string][]byte
	// Mode becomes TOOLBELT_MODE for a lint with a Formatter: "check" or
	// "write". Empty means "check". Other lints never get TOOLBELT_MODE.
	Mode string
	// Files maps a lint's name to the repo-relative files its formatter is to
	// format. The runner writes them NUL-terminated to a file and passes its
	// path as TOOLBELT_FILES; a lint with no entry gets an empty file. Other
	// lints never get TOOLBELT_FILES, and their entries are ignored.
	Files map[string][]string
	// Env holds extra NAME=value pairs for every lint, added after the
	// process's own environment. A TOOLBELT_MODE or TOOLBELT_FILES pair in it
	// is dropped, as one in the process's environment is: only the runner
	// sets them.
	Env []string
}

// Run runs every lint at once and returns their results in the order of
// lints. Cancelling ctx sends SIGTERM to each lint's process group. When a
// lint is still running a grace period later, or has exited while a process
// it started still holds its output open, the grace period ends its run. Once
// a lint's run ends, SIGKILL goes to its whole process group, so nothing it
// started outlives it.
func Run(ctx context.Context, lints []manifest.Lint, opts Options) []Result {
	results := make([]Result, len(lints))
	sectionsDir, err := os.MkdirTemp("", "toolbelt-sections-")
	if err != nil {
		for i, lint := range lints {
			results[i] = errored(lint.Name, nil, nil, fmt.Sprintf("cannot create a directory for config sections: %v", err))
		}
		return results
	}
	defer os.RemoveAll(sectionsDir)

	// One goroutine per lint is bounded by the manifest: a run names each
	// lint at most once.
	var wg sync.WaitGroup
	for i, lint := range lints {
		wg.Go(func() {
			results[i] = runOne(ctx, lint, sectionsDir, opts)
		})
	}
	wg.Wait()
	return results
}

func runOne(ctx context.Context, lint manifest.Lint, sectionsDir string, opts Options) Result {
	name := lint.Name
	section, ok := opts.Sections[name]
	if !ok {
		section = []byte("null")
	}
	sectionFile := filepath.Join(sectionsDir, name+".json")
	if err := os.WriteFile(sectionFile, section, 0o600); err != nil {
		return errored(name, nil, nil, fmt.Sprintf("cannot write config section: %v", err))
	}

	path := filepath.Join(opts.LintsDir, name, name)
	// TOOLBELT_MODE and TOOLBELT_FILES, from the process's environment or
	// opts.Env, must not leak into a lint that takes neither; a formatter
	// gets the runner's own values below.
	env := slices.DeleteFunc(append(os.Environ(), opts.Env...), func(kv string) bool {
		return strings.HasPrefix(kv, "TOOLBELT_MODE=") || strings.HasPrefix(kv, "TOOLBELT_FILES=")
	})
	if lint.Formatter != "" {
		path = filepath.Join(opts.FormattersDir, lint.Formatter, lint.Formatter)
		filesFile := filepath.Join(sectionsDir, name+".files")
		if err := os.WriteFile(filesFile, nulTerminated(opts.Files[name]), 0o600); err != nil {
			return errored(name, nil, nil, fmt.Sprintf("cannot write file list: %v", err))
		}
		mode := opts.Mode
		if mode == "" {
			mode = "check"
		}
		env = append(env, "TOOLBELT_MODE="+mode, "TOOLBELT_FILES="+filesFile)
	}
	cmd := exec.CommandContext(ctx, path) //nolint:gosec // running the named lint's executable is the purpose of this package
	cmd.Dir = opts.RepoRoot
	env = append(env,
		"TOOLBELT_SECTION="+sectionFile,
		"TOOLBELT_HOOK="+opts.Hook,
		"TOOLBELT_BASE="+opts.Base,
		"TOOLBELT_RANGE="+opts.Range,
	)
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// Its own process group, so cancelling reaches the tools a lint starts,
	// not only the lint.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	cmd.WaitDelay = waitDelay

	err := cmd.Run()
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) //nolint:errcheck // ESRCH: the group has already exited
	}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return Result{Lint: name, Exit: ExitPass, Output: stdout.Bytes(), Stderr: stderr.Bytes()}
	case errors.As(err, &exitErr):
		return fromExit(name, exitErr, stdout.Bytes(), stderr.Bytes())
	case errors.Is(err, exec.ErrWaitDelay):
		return errored(name, stdout.Bytes(), stderr.Bytes(), "lint exited, but a process it started kept its output open")
	case errors.Is(err, os.ErrNotExist):
		return errored(name, nil, nil, "lint executable "+path+" not found")
	default:
		return errored(name, stdout.Bytes(), stderr.Bytes(), fmt.Sprintf("cannot run lint executable %s: %v", path, err))
	}
}

// nulTerminated is files, each followed by a NUL byte.
func nulTerminated(files []string) []byte {
	var buf bytes.Buffer
	for _, file := range files {
		buf.WriteString(file)
		buf.WriteByte(0)
	}
	return buf.Bytes()
}

func fromExit(name string, exitErr *exec.ExitError, stdout, stderr []byte) Result {
	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return errored(name, stdout, stderr, "lint killed by signal: "+status.Signal().String())
	}
	code := exitErr.ExitCode()
	if code == ExitFindings || code == ExitError {
		return Result{Lint: name, Exit: code, Output: stdout, Stderr: stderr}
	}
	return errored(name, stdout, stderr, fmt.Sprintf("lint exited %d", code))
}

// errored is an ExitError result with reason added to stderr on a line of its
// own.
func errored(name string, stdout, stderr []byte, reason string) Result {
	if len(stderr) > 0 && stderr[len(stderr)-1] != '\n' {
		stderr = append(stderr, '\n')
	}
	stderr = append(stderr, "toolbelt: "+reason+"\n"...)
	return Result{Lint: name, Exit: ExitError, Output: stdout, Stderr: stderr}
}
