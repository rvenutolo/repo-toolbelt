// Command check-coverage-drop is the repo-toolbelt lint that holds each
// unit's uncovered count to the committed coverage baseline, and requires a
// Coverage-Drop trailer on every commit that raises a baseline line.
package main

import (
	"os"

	"github.com/rvenutolo/repo-toolbelt/internal/coveragedrop"
	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
)

func main() { // coverage-exempt: go test cannot run main; lintkit.Main and the check it runs are tested
	os.Exit(lintkit.Main("check-coverage-drop", "coverage problem", coveragedrop.Check))
}
