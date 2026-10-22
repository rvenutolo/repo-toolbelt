package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rvenutolo/repo-toolbelt/internal/manifest"
)

const (
	modeSplit    = "split"
	modeParallel = "parallel"
	modeMatrix   = "matrix"
)

type planOutput struct {
	Groups [][]string `json:"groups"`
}

// newPlanCmd prints the job groups for the named lints. It touches no
// repository: the shared workflow runs it with no checkout mounted.
func (r Runner) newPlanCmd(getenv func(string) string) *cobra.Command {
	var mode string
	cmd := &cobra.Command{
		Use:   "plan [--mode split|parallel|matrix] [<lint>...]",
		Short: "Print the job groups for a set of lints as JSON",
		Long: "Print the job groups for a set of lints as JSON.\n\n" +
			"With no lints as arguments, reads them from TOOLBELT_LINTS: one per line,\n" +
			"blank lines and # comments ignored.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if mode != modeSplit && mode != modeParallel && mode != modeMatrix {
				return usagef("unknown mode %q, want split, parallel or matrix", mode)
			}
			lints, err := r.selectLints(args, getenv)
			if err != nil {
				return err
			}
			if err := json.NewEncoder(cmd.OutOrStdout()).Encode(planOutput{Groups: group(lints, mode)}); err != nil {
				return fmt.Errorf("write plan: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&mode, "mode", modeSplit, "grouping: split, parallel or matrix")
	return cmd
}

// group arranges lints into jobs. Within a group, lints keep the order given.
func group(lints []manifest.Lint, mode string) [][]string {
	groups := [][]string{}
	switch mode {
	case modeParallel:
		all := make([]string, len(lints))
		for i, lint := range lints {
			all[i] = lint.Name
		}
		groups = append(groups, all)
	case modeMatrix:
		for _, lint := range lints {
			groups = append(groups, []string{lint.Name})
		}
	default:
		quick := []string{}
		slow := [][]string{}
		for _, lint := range lints {
			if lint.Speed == "slow" {
				slow = append(slow, []string{lint.Name})
			} else {
				quick = append(quick, lint.Name)
			}
		}
		if len(quick) > 0 {
			groups = append(groups, quick)
		}
		groups = append(groups, slow...)
	}
	return groups
}
