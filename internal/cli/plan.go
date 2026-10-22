package cli

import (
	"encoding/json"
	"fmt"
	"io"

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

// plan prints the job groups for the named lints. It touches no repository:
// the shared workflow runs it with no checkout mounted.
func (r Runner) plan(args []string, env func(string) string, stdout, _ io.Writer) error {
	fs := newFlagSet("plan")
	mode := fs.String("mode", modeSplit, "grouping: split, parallel or matrix")
	synopsis := "plan [--mode split|parallel|matrix] [<lint>...]\n\n" +
		"With no lints as arguments, reads them from TOOLBELT_LINTS: one per line,\n" +
		"blank lines and # comments ignored."
	if err := parseFlags(fs, args, synopsis, stdout); err != nil {
		return err
	}
	if *mode != modeSplit && *mode != modeParallel && *mode != modeMatrix {
		return usagef("unknown mode %q, want split, parallel or matrix", *mode)
	}
	lints, err := r.selectLints(fs.Args(), env)
	if err != nil {
		return err
	}
	groups := group(lints, *mode)
	if err := json.NewEncoder(stdout).Encode(planOutput{Groups: groups}); err != nil {
		return fmt.Errorf("write plan: %w", err)
	}
	return nil
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
