package cli

import (
	"github.com/spf13/cobra"

	"github.com/rvenutolo/repo-toolbelt/internal/manifest"
	"github.com/rvenutolo/repo-toolbelt/internal/selection"
)

// selectLints resolves the lints named by positional arguments or by
// TOOLBELT_LINTS, reporting a bad selection as a usage error.
func (r Runner) selectLints(args []string, getenv func(string) string) ([]manifest.Lint, error) {
	lints, err := selection.Parse(args, getenv("TOOLBELT_LINTS"), r.Lints)
	if err != nil {
		return nil, usageError{err}
	}
	return lints, nil
}

// noArgs rejects positional arguments as a usage error.
func noArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return usagef("%s takes no arguments, got %q", cmd.Name(), args[0])
	}
	return nil
}
