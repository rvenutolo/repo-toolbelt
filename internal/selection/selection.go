// Package selection turns the lints or formatters a caller names into
// manifest entries.
package selection

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rvenutolo/repo-toolbelt/internal/manifest"
)

// kind names what is being selected, for the error messages.
type kind struct {
	noun, envVar string
}

// Parse resolves the lints to run, in the order named. Names come from args or,
// when args is empty, from envLints: one name per line, with blank lines and
// "#" comments (whole-line or trailing) ignored and surrounding whitespace
// trimmed. Naming lints in both places, or in neither, is an error, as is an
// unknown name or one named twice.
func Parse(args []string, envLints string, known []manifest.Lint) ([]manifest.Lint, error) {
	return parse(args, envLints, known, func(l manifest.Lint) string { return l.Name }, kind{noun: "lint", envVar: "TOOLBELT_LINTS"})
}

// ParseFormatters resolves the formatters to run by formatter name, in the
// order named, with Parse's rules; envFormatters is the text of
// TOOLBELT_FORMATTERS.
func ParseFormatters(args []string, envFormatters string, known []manifest.Formatter) ([]manifest.Formatter, error) {
	return parse(args, envFormatters, known, func(f manifest.Formatter) string { return f.Name }, kind{noun: "formatter", envVar: "TOOLBELT_FORMATTERS"})
}

func parse[T any](args []string, env string, known []T, name func(T) string, k kind) ([]T, error) {
	fromEnv := envNames(env)
	switch {
	case len(args) > 0 && len(fromEnv) > 0:
		return nil, fmt.Errorf("%ss given both as arguments and in %s", k.noun, k.envVar)
	case len(args) == 0 && len(fromEnv) == 0:
		return nil, errors.New("no " + k.noun + " named")
	}
	names := args
	if len(args) == 0 {
		names = fromEnv
	}

	byName := make(map[string]T, len(known))
	for _, entry := range known {
		byName[name(entry)] = entry
	}
	seen := make(map[string]bool, len(names))
	selected := make([]T, 0, len(names))
	for _, n := range names {
		entry, ok := byName[n]
		if !ok {
			return nil, fmt.Errorf("unknown %s %q", k.noun, n)
		}
		if seen[n] {
			return nil, fmt.Errorf("%s %q named twice", k.noun, n)
		}
		seen[n] = true
		selected = append(selected, entry)
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
