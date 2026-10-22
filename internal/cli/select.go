package cli

import (
	"github.com/rvenutolo/repo-toolbelt/internal/manifest"
	"github.com/rvenutolo/repo-toolbelt/internal/selection"
)

// selectLints resolves the lints named by positional arguments or by
// TOOLBELT_LINTS, reporting a bad selection as a usage error.
func (r Runner) selectLints(args []string, env func(string) string) ([]manifest.Lint, error) {
	lints, err := selection.Parse(args, env("TOOLBELT_LINTS"), r.Lints)
	if err != nil {
		return nil, usageError{err}
	}
	return lints, nil
}
