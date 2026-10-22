// Package selection turns the lints a caller names into manifest entries.
package selection

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rvenutolo/repo-toolbelt/internal/manifest"
)

// Parse resolves the lints to run, in the order named. Names come from args or,
// when args is empty, from envLints: one name per line, with blank lines and
// "#" comments (whole-line or trailing) ignored and surrounding whitespace
// trimmed. Naming lints in both places, or in neither, is an error, as is an
// unknown name or one named twice.
func Parse(args []string, envLints string, known []manifest.Lint) ([]manifest.Lint, error) {
	fromEnv := envNames(envLints)
	switch {
	case len(args) > 0 && len(fromEnv) > 0:
		return nil, errors.New("lints given both as arguments and in TOOLBELT_LINTS")
	case len(args) == 0 && len(fromEnv) == 0:
		return nil, errors.New("no lint named")
	}
	names := args
	if len(args) == 0 {
		names = fromEnv
	}

	byName := make(map[string]manifest.Lint, len(known))
	for _, lint := range known {
		byName[lint.Name] = lint
	}
	seen := make(map[string]bool, len(names))
	selected := make([]manifest.Lint, 0, len(names))
	for _, name := range names {
		lint, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("unknown lint %q", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("lint %q named twice", name)
		}
		seen[name] = true
		selected = append(selected, lint)
	}
	return selected, nil
}

func envNames(text string) []string {
	var names []string
	for line := range strings.SplitSeq(text, "\n") {
		line, _, _ = strings.Cut(line, "#")
		if line = strings.TrimSpace(line); line != "" {
			names = append(names, line)
		}
	}
	return names
}
