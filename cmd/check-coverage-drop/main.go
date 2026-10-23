// Command check-coverage-drop is the repo-toolbelt lint that holds each
// unit's uncovered count to the committed coverage baseline, and requires a
// Coverage-Drop trailer on every commit that raises a baseline line.
package main

import (
	"os"

	"github.com/rvenutolo/repo-toolbelt/internal/coveragedrop"
	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
)

func main() { // coverage-exempt: main ends in os.Exit, which would end the test binary
	os.Exit(lintkit.Main("check-coverage-drop", "coverage problem", coveragedrop.Check))
}
