package cli

import (
	"bytes"
	"context"
	"fmt"
	"slices"

	"github.com/rvenutolo/repo-toolbelt/internal/config"
	"github.com/rvenutolo/repo-toolbelt/internal/filesel"
	"github.com/rvenutolo/repo-toolbelt/internal/fmtconfig"
	"github.com/rvenutolo/repo-toolbelt/internal/manifest"
	"github.com/rvenutolo/repo-toolbelt/internal/runner"
)

// formatterPlan is what running a set of lints needs for the derived lints
// among them.
type formatterPlan struct {
	// files maps each derived lint whose section is valid to the files its
	// formatter is to format: the formatter's selected files, minus the
	// section's excludes.
	files map[string][]string
	// failed maps each derived lint whose section has problems to the failing
	// result it gets in place of running.
	failed map[string]runner.Result
}

// prepareFormatters validates the config section of each derived lint among
// lints and, for those whose section is valid, takes their formatters' files
// and removes the excludes. Other lints are left alone. Files are selected
// once, for every formatter in r.Formatters, since which formatter owns a file
// depends on the others' includes; nothing is selected when lints holds no
// derived lint. A derived lint naming a formatter r.Formatters lacks, and a
// failed selection, are setup failures.
func (r Runner) prepareFormatters(ctx context.Context, root string, cfg config.Config, lints []manifest.Lint) (formatterPlan, error) {
	plan := formatterPlan{files: map[string][]string{}, failed: map[string]runner.Result{}}
	type derived struct {
		lint, formatter string
		exclude         []string
	}
	var valid []derived
	named := false
	for _, lint := range lints {
		if lint.Formatter == "" {
			continue
		}
		named = true
		i := slices.IndexFunc(r.Formatters, func(f manifest.Formatter) bool { return f.Name == lint.Formatter })
		if i < 0 {
			return formatterPlan{}, setupf("lint %q checks formatter %q, which the manifest lacks", lint.Name, lint.Formatter)
		}
		section, problems := fmtconfig.Parse(cfg.Section(lint.Name), r.Formatters[i])
		if len(problems) > 0 {
			plan.failed[lint.Name] = sectionFailure(lint.Name, problems)
			continue
		}
		valid = append(valid, derived{lint: lint.Name, formatter: lint.Formatter, exclude: section.Exclude})
	}
	if !named {
		return plan, nil
	}
	selected, err := filesel.Select(ctx, root, r.Formatters)
	if err != nil {
		return formatterPlan{}, setupf("select the formatters' files: %w", err)
	}
	for _, d := range valid {
		plan.files[d.lint] = filesel.Exclude(selected[d.formatter], d.exclude)
	}
	return plan, nil
}

// onlyStaged keeps in each derived lint's files, changing p.files in place,
// only those staged for the commit, and returns, sorted, the files it took
// out because they also have unstaged edits: formatting one would rewrite
// those edits too. A file no formatter selected is never returned. A failed
// listing is a setup failure.
func (p formatterPlan) onlyStaged(ctx context.Context, root string) ([]string, error) {
	staged, partly, err := filesel.Staged(ctx, root)
	if err != nil {
		return nil, setupf("list the staged files: %w", err)
	}
	isStaged, isPartly := make(map[string]bool, len(staged)), make(map[string]bool, len(partly))
	for _, file := range staged {
		isStaged[file] = true
	}
	for _, file := range partly {
		isPartly[file] = true
	}
	var held []string
	for lint, files := range p.files {
		p.files[lint] = slices.DeleteFunc(files, func(file string) bool {
			if isPartly[file] {
				held = append(held, file)
				return true
			}
			return !isStaged[file]
		})
	}
	slices.Sort(held)
	return held, nil
}

// sectionFailure is the failing result of a derived lint whose section has
// problems: each problem as a lint prints a problem in its own section, then
// the count line.
func sectionFailure(lint string, problems []string) runner.Result {
	var out bytes.Buffer
	for _, problem := range problems {
		fmt.Fprintf(&out, "%s: %s: %s\n", config.FileName, lint, problem)
	}
	fmt.Fprintf(&out, "%d problem(s)\n", len(problems))
	return runner.Result{Lint: lint, Exit: runner.ExitFindings, Output: out.Bytes()}
}

// run runs every lint that has no failing result yet, with the plan's files,
// and returns each lint's result in the order of lints.
func (p formatterPlan) run(ctx context.Context, lints []manifest.Lint, opts runner.Options) []runner.Result {
	toRun := slices.DeleteFunc(slices.Clone(lints), func(lint manifest.Lint) bool {
		_, failed := p.failed[lint.Name]
		return failed
	})
	opts.Files = p.files
	ran := runner.Run(ctx, toRun, opts)
	results := make([]runner.Result, 0, len(lints))
	for _, lint := range lints {
		if result, failed := p.failed[lint.Name]; failed {
			results = append(results, result)
			continue
		}
		results = append(results, ran[0])
		ran = ran[1:]
	}
	return results
}
