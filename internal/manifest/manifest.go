// Package manifest parses lints.toml and formatters.toml, the manifests of
// everything the image ships. A lint entry names a lint and says how it is
// written and scheduled; a formatter entry does the same for a formatter and
// adds the files it owns. Every formatter also contributes a derived lint that
// checks its files are formatted.
package manifest

import (
	"errors"
	"fmt"
	"path"
	"regexp"
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
	// Formatter is the name of the formatter whose check this lint is. It is
	// empty for lints.toml entries, and a lints.toml entry cannot set it.
	Formatter string `toml:"-"`
}

type file struct {
	Lints []Lint `toml:"lint"`
}

// Formatter is one entry of the formatter manifest.
type Formatter struct {
	// Name is the language the formatter formats, a lowercase word. It yields
	// the lint name through LintName.
	Name string `toml:"name"`
	// Language is the language the formatter is written in: "bash" or "go".
	Language string `toml:"language"`
	// Speed is the scheduling class of the formatter's check: "quick" or "slow".
	Speed string `toml:"speed"`
	// Description is a one-line summary of what the formatter formats.
	Description string `toml:"description"`
	// Include is the set of globs, matched against a file's base name, that
	// select the files the formatter owns. It is never empty.
	Include []string `toml:"include"`
	// Shebangs is the set of interpreters an extensionless file's shebang may
	// name for the formatter to own it. It may be empty.
	Shebangs []string `toml:"shebangs"`
	// Settings is the allowlist of settings a consumer may override. It may be
	// empty.
	Settings []string `toml:"settings"`
}

type formatterFile struct {
	Formatters []Formatter `toml:"formatter"`
}

// LintName is the name of the lint that checks the formatter's files are
// formatted.
func (f Formatter) LintName() string {
	return "check-" + f.Name + "-formatted"
}

var formatterNamePattern = regexp.MustCompile(`^[a-z][a-z0-9]*$`)

// decodeStrict decodes data into dst and rejects keys dst has no field for.
func decodeStrict(data []byte, dst any) error {
	meta, err := toml.Decode(string(data), dst)
	if err != nil {
		return fmt.Errorf("decode manifest: %w", err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, key := range undecoded {
			keys[i] = key.String()
		}
		return fmt.Errorf("manifest has unknown keys: %s", strings.Join(keys, ", "))
	}
	return nil
}

// Parse decodes manifest TOML into its lints, in file order. It rejects
// unknown keys, missing fields, values outside the allowed languages and
// speeds, and duplicate names. Input with no [[lint]] table yields an empty
// slice and a nil error.
func Parse(data []byte) ([]Lint, error) {
	var parsed file
	if err := decodeStrict(data, &parsed); err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(parsed.Lints))
	for i, lint := range parsed.Lints {
		if err := validateCommon(lint.Name, lint.Language, lint.Speed, lint.Description); err != nil {
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

// validateCommon checks the fields a lint and a formatter share.
func validateCommon(name, language, speed, description string) error {
	required := []struct{ field, value string }{
		{"name", name},
		{"language", language},
		{"speed", speed},
		{"description", description},
	}
	for _, r := range required {
		if r.value == "" {
			return fmt.Errorf("missing field %q", r.field)
		}
	}
	if validLanguages := []string{"bash", "go"}; !slices.Contains(validLanguages, language) {
		return fmt.Errorf("%q: unknown language %q, want one of %v", name, language, validLanguages)
	}
	if validSpeeds := []string{"quick", "slow"}; !slices.Contains(validSpeeds, speed) {
		return fmt.Errorf("%q: unknown speed %q, want one of %v", name, speed, validSpeeds)
	}
	return nil
}

// ParseFormatters decodes formatter manifest TOML into its formatters, in file
// order. It rejects everything Parse rejects, plus a name that is not a
// lowercase word, an empty include list, an include entry that is not a valid
// path.Match pattern, and an include glob or shebang that two formatters
// share, which would let two formatters own one file. Input with no
// [[formatter]] table yields an empty slice and a nil error.
func ParseFormatters(data []byte) ([]Formatter, error) {
	var parsed formatterFile
	if err := decodeStrict(data, &parsed); err != nil {
		return nil, err
	}

	names := make(map[string]bool, len(parsed.Formatters))
	includeOwner := make(map[string]string)
	shebangOwner := make(map[string]string)
	for i, f := range parsed.Formatters {
		if err := validateFormatter(f); err != nil {
			return nil, fmt.Errorf("formatter %d: %w", i+1, err)
		}
		if names[f.Name] {
			return nil, fmt.Errorf("formatter %d: duplicate name %q", i+1, f.Name)
		}
		names[f.Name] = true
		if err := claim(includeOwner, "include glob", f.Name, f.Include); err != nil {
			return nil, err
		}
		if err := claim(shebangOwner, "shebang", f.Name, f.Shebangs); err != nil {
			return nil, err
		}
	}

	formatters := make([]Formatter, len(parsed.Formatters))
	copy(formatters, parsed.Formatters)
	return formatters, nil
}

func validateFormatter(f Formatter) error {
	if err := validateCommon(f.Name, f.Language, f.Speed, f.Description); err != nil {
		return err
	}
	if !formatterNamePattern.MatchString(f.Name) {
		return fmt.Errorf("name %q must match %s", f.Name, formatterNamePattern)
	}
	if len(f.Include) == 0 {
		return fmt.Errorf("%q: include must not be empty", f.Name)
	}
	for _, glob := range f.Include {
		if _, err := path.Match(glob, ""); err != nil {
			return fmt.Errorf("%q: include %q: %w", f.Name, glob, err)
		}
	}
	return nil
}

// claim records formatter as the owner of each value and reports a value that
// another formatter already owns. kind names the value in the error.
func claim(owners map[string]string, kind, formatter string, values []string) error {
	for _, value := range values {
		if owner, taken := owners[value]; taken && owner != formatter {
			return fmt.Errorf("%s %q is owned by both formatter %q and formatter %q", kind, value, owner, formatter)
		}
		owners[value] = formatter
	}
	return nil
}

// Merge returns lints followed by one derived lint per formatter, in order. A
// derived lint takes its language, speed and description from its formatter
// and records the formatter's name in Formatter. It reports an error when a
// derived lint's name is already a lint's name.
func Merge(lints []Lint, formatters []Formatter) ([]Lint, error) {
	merged := make([]Lint, len(lints), len(lints)+len(formatters))
	copy(merged, lints)
	taken := make(map[string]bool, len(lints))
	for _, lint := range lints {
		taken[lint.Name] = true
	}
	for _, f := range formatters {
		name := f.LintName()
		if taken[name] {
			return nil, fmt.Errorf("formatter %q: its lint name %q collides with another lint", f.Name, name)
		}
		taken[name] = true
		merged = append(merged, Lint{
			Name:        name,
			Language:    f.Language,
			Speed:       f.Speed,
			Description: f.Description,
			Formatter:   f.Name,
		})
	}
	return merged, nil
}

// Load parses the manifest embedded in the binary. It panics when the embedded
// manifest is malformed: the file ships with the binary, so a bad one is a
// build defect, not a runtime condition.
func Load() []Lint {
	lints, err := Parse(lintsmanifest.TOML)
	if err != nil {
		panic(errors.Join(errors.New("embedded lints.toml is malformed"), err)) // coverage-exempt: only a malformed embedded lints.toml reaches it, and tests do not replace the embedded file
	}
	return lints
}

// LoadAll parses both embedded manifests and returns the lints, the
// formatters' derived lints included, and the formatters. It panics when
// either manifest is malformed or the two collide, for the reason Load does.
func LoadAll() ([]Lint, []Formatter) {
	formatters, err := ParseFormatters(lintsmanifest.Formatters)
	if err != nil {
		panic(errors.Join(errors.New("embedded formatters.toml is malformed"), err)) // coverage-exempt: only a malformed embedded formatters.toml reaches it, and tests do not replace the embedded file
	}
	lints, err := Merge(Load(), formatters)
	if err != nil {
		panic(errors.Join(errors.New("embedded manifests collide"), err)) // coverage-exempt: only colliding embedded manifests reach it, and tests do not replace the embedded files
	}
	return lints, formatters
}
