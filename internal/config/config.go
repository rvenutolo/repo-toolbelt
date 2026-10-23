// Package config loads and validates the file level of a consumer's
// .repo-toolbelt.yml. The file holds one top-level key per lint, each holding
// that lint's section. The loader owns what is true of the whole file; each
// lint owns the schema of its own section.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

// FileName is the name of the config file, at the repo root.
const FileName = ".repo-toolbelt.yml"

//nolint:gochecknoglobals // immutable lookup table
var escaper = strings.NewReplacer(
	"\\", `\\`, "\"", `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`,
)

const (
	nullJSON     = "null"
	topLevel     = "top level"
	tagFixSuffix = "; quote the value or drop the tag"
)

// Config is the validated content of a config file. The zero value is the
// config of a repo with no file: every section is absent.
type Config struct {
	sections map[string][]byte
}

// Section returns the named lint's section as JSON. It returns []byte("null")
// when the file has no such section, the section holds null, or the file had
// problems that prevented reading it.
func (c Config) Section(lint string) []byte {
	if section, ok := c.sections[lint]; ok {
		return slices.Clone(section)
	}
	return []byte(nullJSON)
}

// problem is one file-level finding, positioned for ordering.
type problem struct {
	offset int
	text   string
}

// Load reads repoRoot's config file and validates its file level: it parses,
// holds one document, uses no merge key, alias or non-core tag, has a map at
// the top, repeats no key anywhere and names only the known lints. A missing
// file is the same as an empty one. The problems come back in file order, each
// without the file name prefix; the returned Config is empty when there are
// any. The error reports a failure to read the file, which is not a problem
// with its content.
func Load(repoRoot string, known []string) (Config, []string, error) {
	path := filepath.Join(repoRoot, FileName)

	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil, nil
	}
	if err != nil {
		return Config{}, nil, fmt.Errorf("stat %s: %w", FileName, err)
	}

	data, err := os.ReadFile(path) //nolint:gosec // the path is the fixed config file name under the caller's repo root
	if err != nil {
		// A dangling symlink is not "no file": it is a file that cannot be parsed.
		if info.Mode()&fs.ModeSymlink != 0 && errors.Is(err, fs.ErrNotExist) {
			return Config{}, []string{"not valid YAML: " + err.Error()}, nil
		}
		return Config{}, nil, fmt.Errorf("read %s: %w", FileName, err)
	}

	cfg, problems := parse(data, known)
	return cfg, problems, nil
}

func parse(data []byte, known []string) (Config, []string) {
	file, err := parser.ParseBytes(data, 0, parser.AllowDuplicateMapKey())
	if err != nil {
		return Config{}, []string{"not valid YAML: " + err.Error()}
	}
	switch len(file.Docs) {
	case 0:
		return Config{}, nil
	case 1:
	default:
		return Config{}, []string{fmt.Sprintf("holds %d YAML documents; expected one", len(file.Docs))}
	}
	body := file.Docs[0].Body

	// Checked before anything else reads the file, and fatal: every later read
	// would see merged, aliased or untagged content instead of what was written.
	var unsupported collector
	unsupported.walk(body, nil)
	if problems := unsupported.sorted(); len(problems) > 0 {
		return Config{}, problems
	}

	entries, isMap := mapEntries(body)
	if !isMap {
		if isNull(body) {
			return Config{}, nil
		}
		return Config{}, []string{"top level must be a map"}
	}

	var findings collector
	findings.duplicateKeys(body, nil)
	for _, entry := range entries {
		name := keyText(entry.Key)
		if !slices.Contains(known, name) {
			findings.add(entry.Key, fmt.Sprintf("unknown lint '%s'; known: %s", escape(name), strings.Join(known, ", ")))
		}
	}
	if problems := findings.sorted(); len(problems) > 0 {
		return Config{}, problems
	}

	sections := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		section, err := toJSON(entry.Value)
		if err != nil {
			findings.add(entry.Key, fmt.Sprintf("%s: cannot be read as JSON: %v", escape(keyText(entry.Key)), err))
			continue
		}
		sections[keyText(entry.Key)] = section
	}
	if problems := findings.sorted(); len(problems) > 0 {
		return Config{}, problems
	}
	return Config{sections: sections}, nil
}

// toJSON renders a section's node as JSON; a null or absent node is "null".
func toJSON(node ast.Node) ([]byte, error) {
	node = unwrap(node)
	if isNull(node) {
		return []byte(nullJSON), nil
	}
	var value any
	if err := yaml.NodeToValue(node, &value); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	out, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	return out, nil
}

// collector gathers positioned problems.
type collector struct {
	problems []problem
}

func (c *collector) add(at ast.Node, text string) {
	offset := 0
	if at != nil {
		if tk := at.GetToken(); tk != nil && tk.Position != nil {
			offset = tk.Position.Offset
		}
	}
	c.problems = append(c.problems, problem{offset: offset, text: text})
}

// sorted returns the problems' text in file order.
func (c *collector) sorted() []string {
	slices.SortStableFunc(c.problems, func(a, b problem) int { return a.offset - b.offset })
	out := make([]string, len(c.problems))
	for i, p := range c.problems {
		out[i] = p.text
	}
	return out
}

// walk reports every construct a JSON conversion would silently rewrite:
// merge keys, aliases and tags other than the core ones.
func (c *collector) walk(node ast.Node, path []string) {
	switch n := node.(type) {
	case *ast.TagNode:
		if tag := n.Start.Value; !coreTag(tag) {
			c.add(n, fmt.Sprintf("YAML tag '%s' is not supported (in %s)%s", escape(tag), where(path), tagFixSuffix))
		}
		c.walk(n.Value, path)
	case *ast.AnchorNode:
		c.walk(n.Value, path)
	case *ast.AliasNode:
		c.add(n, fmt.Sprintf("YAML alias '*%s' is not supported (in %s)", escape(aliasName(n)), where(path)))
	case *ast.SequenceNode:
		for i, value := range n.Values {
			c.walk(value, append(path, fmt.Sprintf("[%d]", i)))
		}
	default:
		entries, isMap := mapEntries(node)
		if !isMap {
			return
		}
		for _, entry := range entries {
			c.walkKey(entry.Key, path)
			c.walk(entry.Value, append(path, "."+escape(keyText(entry.Key))))
		}
	}
}

// walkKey reports a merge key, or an alias or tag in a key, against the map
// holding it.
func (c *collector) walkKey(key ast.MapKeyNode, path []string) {
	if key.IsMergeKey() {
		c.add(key, fmt.Sprintf("merge key '<<' is not supported (in %s)", where(path)))
		return
	}
	var node ast.Node = key
	if explicit, ok := key.(*ast.MappingKeyNode); ok {
		node = explicit.Value
	}
	c.walk(node, path)
}

// duplicateKeys reports every key a map repeats, anywhere in the file. A
// duplicate is invisible to every later read, which keeps only the last.
func (c *collector) duplicateKeys(node ast.Node, path []string) {
	node = unwrap(node)
	if seq, ok := node.(*ast.SequenceNode); ok {
		for i, value := range seq.Values {
			c.duplicateKeys(value, append(path, fmt.Sprintf("[%d]", i)))
		}
		return
	}
	entries, isMap := mapEntries(node)
	if !isMap {
		return
	}
	seen := make(map[string]bool, len(entries))
	reported := make(map[string]bool)
	for _, entry := range entries {
		name := keyText(entry.Key)
		if seen[name] && !reported[name] {
			reported[name] = true
			c.add(entry.Key, fmt.Sprintf("duplicate key '%s' in %s", escape(name), where(path)))
		}
		seen[name] = true
		c.duplicateKeys(entry.Value, append(path, "."+escape(name)))
	}
}

// mapEntries returns a mapping node's entries.
func mapEntries(node ast.Node) ([]*ast.MappingValueNode, bool) {
	if mapping, ok := unwrap(node).(*ast.MappingNode); ok {
		return mapping.Values, true
	}
	return nil, false
}

// unwrap strips anchors, tags and the `?` of an explicit key, which carry no
// content of their own.
func unwrap(node ast.Node) ast.Node {
	for {
		switch n := node.(type) {
		case *ast.AnchorNode:
			node = n.Value
		case *ast.TagNode:
			node = n.Value
		case *ast.MappingKeyNode:
			node = n.Value
		default:
			return node
		}
	}
}

func isNull(node ast.Node) bool {
	switch unwrap(node).(type) {
	case nil, *ast.NullNode:
		return true
	default:
		return false
	}
}

// keyText is a key as the lints see it: its decoded text.
func keyText(key ast.MapKeyNode) string {
	switch n := unwrap(key).(type) {
	case ast.ScalarNode:
		return fmt.Sprint(n.GetValue())
	default:
		return strings.TrimSpace(n.String())
	}
}

func aliasName(alias *ast.AliasNode) string {
	return strings.TrimSpace(alias.Value.String())
}

func coreTag(tag string) bool {
	switch tag {
	case "!!map", "!!seq", "!!str", "!!null", "!!int", "!!bool", "!!float":
		return true
	default:
		return false
	}
}

// where renders a path as `section.key[index]`, or `top level` for the root.
func where(path []string) string {
	joined := strings.TrimPrefix(strings.Join(path, ""), ".")
	if joined == "" {
		return topLevel
	}
	return joined
}

// escape prints a value on one line, as JSON string escaping without the
// quotes, so `a<newline>b` prints as `a\nb`. Keys are user data: a raw newline
// would make one message look like two, and a raw carriage return would
// garble the terminal line.
func escape(text string) string {
	return escaper.Replace(text)
}
