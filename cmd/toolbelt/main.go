// Command toolbelt runs the repo-toolbelt lints.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rvenutolo/repo-toolbelt/internal/cli"
)

// version is set at link time by the Nix build (-X main.version=...); a plain
// `go build` leaves it empty and the runner falls back to the build info.
var version string

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	context.AfterFunc(ctx, stop) // first signal cancels ctx; a second one kills the process
	// coverage-exempt: main runs only as a built binary; the e2e suite runs it in the image
	err := cli.Run(ctx, version, os.Args[1:], os.Getenv, os.Stdin, os.Stdout, os.Stderr)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(cli.ExitCode(err))
	}
}
