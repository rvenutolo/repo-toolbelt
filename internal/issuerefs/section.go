package issuerefs

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
)

// configFile is the consumer config file, named in stale-exemption findings.
const configFile = ".repo-toolbelt.yml"

// Exemptions are the lint's `exempt` entries. Each records whether it has
// suppressed a reference, so an exemption that suppresses nothing can be
// reported as stale. Not safe for concurrent use.
type Exemptions struct {
	entries []*exemption
}

type exemption struct {
	pattern []string // the path glob, split on `/`
	path    string
	match   *regexp.Regexp // nil exempts the whole file
	used    bool
}

// ParseSection validates the lint's config section, given as JSON, and returns
// its exemptions. Every problem in the section is returned as a section
// error, and then the exemptions are nil. The JSON null is an empty section.
// An error means the section is not JSON at all.
func ParseSection(data []byte) (*Exemptions, []lintkit.Finding, error) {
	var section any
	if err := json.Unmarshal(data, &section); err != nil {
		return nil, nil, fmt.Errorf("parse config section: %w", err)
	}
	if section == nil {
		return &Exemptions{}, nil, nil
	}
	fields, ok := section.(map[string]any)
	if !ok {
		return nil, []lintkit.Finding{lintkit.SectionError("", "must be a map")}, nil
	}
	problems := unknownKeys("", fields, "exempt")
	var exemptions Exemptions
	if list, present := fields["exempt"]; present {
		items, ok := list.([]any)
		if !ok {
			problems = append(problems, lintkit.SectionError("", "exempt must be a list"))
		}
		for i, item := range items {
			entry, entryProblems := parseEntry(i, item)
			problems = append(problems, entryProblems...)
			if entry != nil {
				exemptions.entries = append(exemptions.entries, entry)
			}
		}
	}
	if len(problems) > 0 {
		return nil, problems, nil
	}
	return &exemptions, nil, nil
}

// parseEntry validates `exempt[index]`. It returns the exemption only when the
// entry has no problem.
func parseEntry(index int, item any) (*exemption, []lintkit.Finding) {
	where := fmt.Sprintf("exempt[%d]", index)
	fields, ok := item.(map[string]any)
	if !ok {
		return nil, []lintkit.Finding{lintkit.SectionError("", where+" must be a map")}
	}
	problems := unknownKeys(where, fields, "path", "match", "reason")
	entry := &exemption{}

	pattern, problem := requiredString(fields, "path")
	switch {
	case problem != "":
		problems = append(problems, lintkit.SectionError(where, problem))
	case !validGlob(pattern):
		problems = append(problems, lintkit.SectionError(where, fmt.Sprintf("path '%s' is not a valid glob", pattern)))
	default:
		entry.path = pattern
		entry.pattern = strings.Split(pattern, "/")
	}

	if value, present := fields["match"]; present {
		match, isString := value.(string)
		switch {
		case !isString:
			problems = append(problems, lintkit.SectionError(where, "match must be a string"))
		case match == "":
			problems = append(problems, lintkit.SectionError(where, "match must not be empty; drop it to exempt the whole file"))
		default:
			re, err := regexp.Compile(match)
			if err != nil {
				problems = append(problems, lintkit.SectionError(where, fmt.Sprintf("match '%s' is not a valid regex: %v", match, err)))
			}
			entry.match = re
		}
	}

	if _, problem := requiredString(fields, "reason"); problem != "" {
		problems = append(problems, lintkit.SectionError(where, problem))
	}
	if len(problems) > 0 {
		return nil, problems
	}
	return entry, nil
}

// requiredString returns the non-blank string under key, or the problem with
// it. A key given with no value is null, which is the wrong type rather than
// absent, so a half-written entry is reported as such.
func requiredString(fields map[string]any, key string) (string, string) {
	value, present := fields[key]
	if !present {
		return "", key + " is missing or empty"
	}
	text, ok := value.(string)
	if !ok {
		return "", key + " must be a string"
	}
	if strings.TrimSpace(text) == "" {
		return "", key + " is missing or empty"
	}
	return text, ""
}

func unknownKeys(where string, fields map[string]any, allowed ...string) []lintkit.Finding {
	var unknown []string
	for key := range fields {
		if !slices.Contains(allowed, key) {
			unknown = append(unknown, key)
		}
	}
	slices.Sort(unknown)
	problems := make([]lintkit.Finding, 0, len(unknown))
	for _, key := range unknown {
		problems = append(problems, lintkit.SectionError(where, fmt.Sprintf("unknown key '%s'", key)))
	}
	return problems
}

// validGlob reports whether every segment of pattern is a valid path.Match
// pattern; `**` is always valid.
func validGlob(pattern string) bool {
	for segment := range strings.SplitSeq(pattern, "/") {
		if _, err := path.Match(segment, ""); err != nil {
			return false
		}
	}
	return true
}

// Exempts reports whether a reference on line, in the file at the
// repo-relative path, is exempt, and marks every exemption that covers it as
// used.
func (e *Exemptions) Exempts(file, line string) bool {
	exempt := false
	for _, entry := range e.entries {
		if !globMatch(entry.pattern, strings.Split(file, "/")) {
			continue
		}
		if entry.match != nil && !entry.match.MatchString(line) {
			continue
		}
		entry.used = true
		exempt = true
	}
	return exempt
}

// Stale returns a finding for every exemption that has suppressed nothing.
func (e *Exemptions) Stale() []lintkit.Finding {
	var stale []lintkit.Finding
	for _, entry := range e.entries {
		if entry.used {
			continue
		}
		name := entry.path
		if entry.match != nil {
			name += " match " + entry.match.String()
		}
		stale = append(stale, lintkit.Finding{Path: configFile, Message: "stale exemption: " + name + " suppresses nothing"})
	}
	return stale
}

// globMatch matches a git pathspec glob, split into segments, against a path:
// `*`, `?` and classes stay within a segment, and a `**` segment matches any
// number of segments, none included. As with a git pathspec, a pattern that
// matches a leading directory of the path matches every file under it.
func globMatch(pattern, segments []string) bool {
	if len(pattern) == 0 {
		return true
	}
	if pattern[0] == "**" {
		for skip := range len(segments) + 1 {
			if globMatch(pattern[1:], segments[skip:]) {
				return true
			}
		}
		return false
	}
	if len(segments) == 0 {
		return false
	}
	if ok, _ := path.Match(pattern[0], segments[0]); !ok { //nolint:errcheck // the pattern was validated when the section was parsed
		return false
	}
	return globMatch(pattern[1:], segments[1:])
}
