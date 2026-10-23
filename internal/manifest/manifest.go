// Package manifest parses lints.toml, the manifest of every lint the image
// ships. Each entry names a lint and says how it is written and scheduled.
package manifest

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"

	lintsmanifest "github.com/rvenutolo/repo-toolbelt"
)

// Lint is one entry of the manifest.
type Lint struct {
	// Name is the lint's executable name and its unique identifier.
	Name string `toml:"name"`
	// Language is the language the lint is written in: "bash" or "go".
	Language string `toml:"language"`
	// Speed is the scheduling class: "quick" or "slow".
	Speed string `toml:"speed"`
	// Description is a one-line summary of what the lint checks.
	Description string `toml:"description"`
}

type file struct {
	Lints []Lint `toml:"lint"`
}

// Parse decodes manifest TOML into its lints, in file order. It rejects
// unknown keys, missing fields, values outside the allowed languages and
// speeds, and duplicate names. Input with no [[lint]] table yields an empty
// slice and a nil error.
func Parse(data []byte) ([]Lint, error) {
	var parsed file
	meta, err := toml.Decode(string(data), &parsed)
	if err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, key := range undecoded {
			keys[i] = key.String()
		}
		return nil, fmt.Errorf("manifest has unknown keys: %s", strings.Join(keys, ", "))
	}

	seen := make(map[string]bool, len(parsed.Lints))
	for i, lint := range parsed.Lints {
		if err := validate(lint); err != nil {
			return nil, fmt.Errorf("lint %d: %w", i+1, err)
		}
		if seen[lint.Name] {
			return nil, fmt.Errorf("lint %d: duplicate name %q", i+1, lint.Name)
		}
		seen[lint.Name] = true
	}

	lints := make([]Lint, len(parsed.Lints))
	copy(lints, parsed.Lints)
	return lints, nil
}

func validate(lint Lint) error {
	required := []struct{ field, value string }{
		{"name", lint.Name},
		{"language", lint.Language},
		{"speed", lint.Speed},
		{"description", lint.Description},
	}
	for _, r := range required {
		if r.value == "" {
			return fmt.Errorf("missing field %q", r.field)
		}
	}
	if validLanguages := []string{"bash", "go"}; !slices.Contains(validLanguages, lint.Language) {
		return fmt.Errorf("lint %q: unknown language %q, want one of %v", lint.Name, lint.Language, validLanguages)
	}
	if validSpeeds := []string{"quick", "slow"}; !slices.Contains(validSpeeds, lint.Speed) {
		return fmt.Errorf("lint %q: unknown speed %q, want one of %v", lint.Name, lint.Speed, validSpeeds)
	}
	return nil
}

// Load parses the manifest embedded in the binary. It panics when the embedded
// manifest is malformed: the file ships with the binary, so a bad one is a
// build defect, not a runtime condition.
func Load() []Lint {
	lints, err := Parse(lintsmanifest.TOML)
	if err != nil {
		panic(errors.Join(errors.New("embedded lints.toml is malformed"), err)) // coverage-exempt: the manifest is embedded at build time; a malformed one fails every test
	}
	return lints
}
