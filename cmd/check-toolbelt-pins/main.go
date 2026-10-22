// Command check-toolbelt-pins is the repo-toolbelt lint that reports a
// repo-toolbelt image or shared-workflow pin that is unpinned or disagrees
// with the others.
package main

import (
	"os"

	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
	"github.com/rvenutolo/repo-toolbelt/internal/pins"
)

func main() { // coverage-exempt: go test cannot run main; lintkit.Main and the check it runs are tested
	os.Exit(lintkit.Main("check-toolbelt-pins", "pin problem", pins.Check))
}
