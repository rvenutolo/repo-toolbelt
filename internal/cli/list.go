package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newListCmd prints every manifest lint as "<name>\t<speed>\t<description>".
func (r Runner) newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Print every lint with its speed and description",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			for _, lint := range r.Lints {
				if _, err := fmt.Fprintf(out, "%s\t%s\t%s\n", lint.Name, lint.Speed, lint.Description); err != nil {
					return fmt.Errorf("write list: %w", err)
				}
			}
			return nil
		},
	}
}
