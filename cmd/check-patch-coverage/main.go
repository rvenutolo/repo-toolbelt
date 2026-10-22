// Command check-patch-coverage is the repo-toolbelt lint that requires every
// code block or line a change touches to be covered by the tests, or marked
// exempt with a reason.
package main

import (
	"os"

	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
	"github.com/rvenutolo/repo-toolbelt/internal/patchcov"
)

func main() { // coverage-exempt: go test cannot run main; lintkit.Main and the check it runs are tested
	os.Exit(lintkit.Main("check-patch-coverage", "coverage problem", patchcov.Check))
}
