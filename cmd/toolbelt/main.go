// Command toolbelt runs the repo-toolbelt lints.
package main

import (
	"os"

	"github.com/rvenutolo/repo-toolbelt/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Getenv, os.Stdin, os.Stdout, os.Stderr))
}
