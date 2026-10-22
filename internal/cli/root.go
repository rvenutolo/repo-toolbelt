package cli

import (
	"io"

	"github.com/spf13/cobra"
)

// newRootCmd builds the command tree. Nothing is shared between calls, so
// concurrent runs do not interfere.
func (r Runner) newRootCmd(getenv func(string) string, stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:     "toolbelt",
		Long:    rootLong,
		Version: r.Version,
		// Errors are returned to the caller; cobra must not print them or the
		// usage text on its own.
		SilenceErrors: true,
		SilenceUsage:  true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usagef("unknown subcommand %q", args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			// No subcommand: show the usage, as a usage error.
			cmd.SetOut(cmd.ErrOrStderr())
			if err := cmd.Help(); err != nil {
				return err
			}
			return usagef("no command given")
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return usageError{err} })
	root.AddCommand(r.newRunCmd(getenv), r.newPlanCmd(getenv), r.newListCmd())
	return root
}
