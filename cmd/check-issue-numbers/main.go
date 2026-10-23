// Command check-issue-numbers is the repo-toolbelt lint that reports a local
// issue or PR reference, `#N`, in any tracked text file.
package main

import (
	"os"

	"github.com/rvenutolo/repo-toolbelt/internal/issuerefs"
	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
)

func main() { // coverage-exempt: main runs only as a built binary; the e2e suite runs it in the image
	os.Exit(lintkit.Main("check-issue-numbers", "issue reference", issuerefs.Check))
}
