package coverage

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
)

// DefaultBaseline is the baseline file when the section names none.
const DefaultBaseline = ".coverage-baseline.txt"

// Section is the config section the coverage lints share.
type Section struct {
	// Reports are the coverage reports, in the order listed.
	Reports []ReportSpec
	// Baseline is the repo-relative baseline file.
	Baseline string
}

// ReportSpec is one entry of the section's reports.
type ReportSpec struct {
	// Format is FormatGo or FormatCobertura.
	Format string
	// Path is the report, relative to the repo root.
	Path string
}

// ParseSection validates a coverage lint's config section, given as JSON:
// `reports`, a non-empty list of `{format, path}`, and an optional
// `baseline`. Every problem is returned as a section error, and then the
// Section is zero. The JSON null, no section at all, is a problem: the lint
// cannot know where the reports are. An error means the section is not JSON.
func ParseSection(data []byte) (Section, []lintkit.Finding, error) {
	var section any
	if err := json.Unmarshal(data, &section); err != nil {
		return Section{}, nil, fmt.Errorf("parse config section: %w", err)
	}
	if section == nil {
		return Section{}, []lintkit.Finding{lintkit.SectionError("", "reports is missing or empty")}, nil
	}
	fields, ok := section.(map[string]any)
	if !ok {
		return Section{}, []lintkit.Finding{lintkit.SectionError("", "must be a map")}, nil
	}
	problems := unknownKeys("", fields, "reports", "baseline")
	reports, reportProblems := parseReports(fields["reports"])
	problems = append(problems, reportProblems...)

	baseline := DefaultBaseline
	if value, present := fields["baseline"]; present {
		text, problem := pathField(value, "baseline")
		switch {
		case problem != "":
			problems = append(problems, lintkit.SectionError("", problem))
		case slices.ContainsFunc(reports, func(r ReportSpec) bool { return r.Path == text }):
			problems = append(problems, lintkit.SectionError("", fmt.Sprintf("baseline '%s' is also a report", text)))
		default:
			baseline = text
		}
	}
	if len(problems) > 0 {
		return Section{}, problems, nil
	}
	return Section{Reports: reports, Baseline: baseline}, nil, nil
}

func parseReports(value any) ([]ReportSpec, []lintkit.Finding) {
	if value == nil {
		return nil, []lintkit.Finding{lintkit.SectionError("", "reports is missing or empty")}
	}
	items, ok := value.([]any)
	if !ok {
		return nil, []lintkit.Finding{lintkit.SectionError("", "reports must be a list")}
	}
	if len(items) == 0 {
		return nil, []lintkit.Finding{lintkit.SectionError("", "reports is missing or empty")}
	}
	var (
		reports  []ReportSpec
		problems []lintkit.Finding
	)
	for i, item := range items {
		where := fmt.Sprintf("reports[%d]", i)
		entry, ok := item.(map[string]any)
		if !ok {
			problems = append(problems, lintkit.SectionError("", where+" must be a map"))
			continue
		}
		entryProblems := unknownKeys(where, entry, "format", "path")
		format, problem := stringField(entry["format"], "format")
		switch {
		case problem != "":
			entryProblems = append(entryProblems, lintkit.SectionError(where, problem))
		case format != FormatGo && format != FormatCobertura:
			entryProblems = append(entryProblems, lintkit.SectionError(where, fmt.Sprintf("format '%s' is not go or cobertura", format)))
		}
		path, problem := pathField(entry["path"], "path")
		switch {
		case problem != "":
			entryProblems = append(entryProblems, lintkit.SectionError(where, problem))
		case slices.ContainsFunc(reports, func(r ReportSpec) bool { return r.Path == path }):
			entryProblems = append(entryProblems, lintkit.SectionError(where, fmt.Sprintf("path '%s' is already listed", path)))
		}
		if len(entryProblems) > 0 {
			problems = append(problems, entryProblems...)
			continue
		}
		reports = append(reports, ReportSpec{Format: format, Path: path})
	}
	return reports, problems
}

// stringField returns the non-blank string value, or the problem with it.
func stringField(value any, key string) (string, string) {
	if value == nil {
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

// pathField is stringField for a path that must stay inside the repo.
func pathField(value any, key string) (string, string) {
	text, problem := stringField(value, key)
	if problem != "" {
		return "", problem
	}
	if !filepath.IsLocal(text) {
		return "", fmt.Sprintf("%s '%s' must be relative to the repo root and inside it", key, text)
	}
	return filepath.ToSlash(filepath.Clean(text)), ""
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
