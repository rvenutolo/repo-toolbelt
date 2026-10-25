// Package fmtconfig validates a formatter's section of the consumer config
// file, the one keyed by the formatter's lint name.
package fmtconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/rvenutolo/repo-toolbelt/internal/manifest"
	"github.com/rvenutolo/repo-toolbelt/internal/pathglob"
)

const (
	excludeKey  = "exclude"
	settingsKey = "settings"
)

//nolint:gochecknoglobals // immutable lookup table
var escaper = strings.NewReplacer(
	"\\", `\\`, "\"", `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`,
)

// Section is a formatter's validated config section. A section that is absent
// or empty is the zero Section; Exclude and Settings are nil when they hold no
// entries.
type Section struct {
	// Exclude is the path globs of the files the formatter must leave alone.
	Exclude []string
	// Settings maps an allowlisted setting name to its positive integer value.
	Settings map[string]int
}

// Parse validates raw, the JSON of the formatter's section (`null` when the
// section is absent), against the formatter's manifest entry. It returns the
// section and no problems when the section is valid, and the zero Section and
// every problem otherwise. A problem is a one-line message without the
// `.repo-toolbelt.yml: <lint>:` prefix, so the caller can print it as the
// section error of the formatter's lint. Problems are ordered: unknown keys,
// then exclude entries, then settings in key order. User-supplied text in a
// problem is escaped onto one line.
func Parse(raw []byte, f manifest.Formatter) (Section, []string) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber() // keeps integer values exact; float64 would round large ones
	if err := decoder.Decode(&value); err != nil {
		return Section{}, []string{"is not valid JSON: " + err.Error()}
	}
	if decoder.More() {
		return Section{}, []string{"is not valid JSON: unexpected trailing data"}
	}
	if value == nil {
		return Section{}, nil
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return Section{}, []string{"must be a map"}
	}

	var problems []string
	for _, key := range sortedKeys(fields) {
		if key != excludeKey && key != settingsKey {
			problems = append(problems, fmt.Sprintf("unknown key '%s'; allowed: %s, %s", escape(key), excludeKey, settingsKey))
		}
	}
	var section Section
	if list, present := fields[excludeKey]; present {
		var excludeProblems []string
		section.Exclude, excludeProblems = parseExclude(list)
		problems = append(problems, excludeProblems...)
	}
	if settings, present := fields[settingsKey]; present {
		var settingsProblems []string
		section.Settings, settingsProblems = parseSettings(settings, f)
		problems = append(problems, settingsProblems...)
	}
	if len(problems) > 0 {
		return Section{}, problems
	}
	return section, nil
}

func parseExclude(value any) ([]string, []string) {
	items, ok := value.([]any)
	if !ok {
		return nil, []string{excludeKey + ": must be a list"}
	}
	var globs, problems []string
	for i, item := range items {
		where := fmt.Sprintf("%s[%d]", excludeKey, i)
		glob, ok := item.(string)
		switch {
		case !ok:
			problems = append(problems, where+": must be a string")
		case strings.TrimSpace(glob) == "":
			problems = append(problems, where+": must not be empty")
		case !pathglob.Valid(glob):
			problems = append(problems, fmt.Sprintf("%s: invalid glob '%s'", where, escape(glob)))
		default:
			globs = append(globs, glob)
		}
	}
	return globs, problems
}

func parseSettings(value any, f manifest.Formatter) (map[string]int, []string) {
	fields, ok := value.(map[string]any)
	if !ok {
		return nil, []string{settingsKey + ": must be a map"}
	}
	if len(fields) > 0 && len(f.Settings) == 0 {
		return nil, []string{fmt.Sprintf("%s: %s takes no settings", settingsKey, f.LintName())}
	}
	var problems []string
	var settings map[string]int
	for _, name := range sortedKeys(fields) {
		if !slices.Contains(f.Settings, name) {
			problems = append(problems, fmt.Sprintf("%s: unknown setting '%s'; allowed: %s",
				settingsKey, escape(name), strings.Join(f.Settings, ", ")))
			continue
		}
		width, ok := positiveInt(fields[name])
		if !ok {
			problems = append(problems, fmt.Sprintf("%s.%s: must be a positive integer", settingsKey, name))
			continue
		}
		if settings == nil {
			settings = make(map[string]int)
		}
		settings[name] = width
	}
	return settings, problems
}

// positiveInt returns value as an int when it is a JSON number written as a
// positive integer that fits in an int.
func positiveInt(value any) (int, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	width, err := strconv.Atoi(number.String())
	if err != nil || width <= 0 {
		return 0, false
	}
	return width, true
}

func sortedKeys(fields map[string]any) []string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// escape prints user-supplied text on one line, as JSON string escaping
// without the quotes: a raw newline would make one message look like two, and
// a raw carriage return would garble the terminal line.
func escape(text string) string {
	return escaper.Replace(text)
}
