package coverage

import (
	"bufio"
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
)

// maxLineBytes bounds one line of a report or baseline, so a corrupt file
// cannot make the reader buffer without limit.
const maxLineBytes = 1 << 20

// blockKey is a Go block's position, which identifies it within a profile.
type blockKey struct {
	path                                 string
	startLine, startCol, endLine, endCol int
}

// ReadGo reads a Go cover profile (`go test -coverprofile`, in any mode).
// Files are named by import path; they are made repo-relative through the
// module path in root's go.mod, so root must be the module root. A block
// listed more than once, as when several test binaries cover one package, is
// covered when any of its listings is. A file outside the module is an
// *OutsideError; anything else wrong with the profile is an error naming the
// line.
func ReadGo(root string, r io.Reader) (Report, error) {
	module, err := modulePath(root)
	if err != nil {
		return Report{}, err
	}
	blocks := map[blockKey]*Block{}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(nil, maxLineBytes)
	mode := ""
	n := 0
	for scanner.Scan() {
		n++
		text := strings.TrimSuffix(scanner.Text(), "\r")
		if after, ok := strings.CutPrefix(text, "mode: "); ok {
			if err := checkMode(mode, after); err != nil {
				return Report{}, fmt.Errorf("line %d: %w", n, err)
			}
			mode = after
			continue
		}
		if mode == "" {
			return Report{}, fmt.Errorf("line %d: a profile starts with a mode line", n)
		}
		block, err := parseProfileLine(text, module)
		if err != nil {
			return Report{}, fmt.Errorf("line %d: %w", n, err)
		}
		key := blockKey{block.Path, block.StartLine, block.StartCol, block.EndLine, block.EndCol}
		seen, ok := blocks[key]
		if !ok {
			blocks[key] = &block
			continue
		}
		if seen.Statements != block.Statements {
			return Report{}, fmt.Errorf("line %d: block %s:%d.%d,%d.%d has %d statements here and %d before",
				n, block.Path, block.StartLine, block.StartCol, block.EndLine, block.EndCol, block.Statements, seen.Statements)
		}
		seen.Covered = seen.Covered || block.Covered
	}
	if err := scanner.Err(); err != nil {
		return Report{}, fmt.Errorf("line %d: %w", n+1, err)
	}
	if mode == "" {
		return Report{}, errors.New("empty profile: no mode line")
	}
	return sortedReport(blocks), nil
}

// checkMode accepts the first mode line, and any later one repeating it.
func checkMode(current, mode string) error {
	if mode != "set" && mode != "count" && mode != "atomic" {
		return fmt.Errorf("unknown mode %q, want set, count or atomic", mode)
	}
	if current != "" && current != mode {
		return fmt.Errorf("mode %q differs from the earlier mode %q", mode, current)
	}
	return nil
}

// parseProfileLine parses `<import path>:<l>.<c>,<l>.<c> <statements> <count>`.
func parseProfileLine(text, module string) (Block, error) {
	colon := strings.LastIndexByte(text, ':')
	if colon < 0 {
		return Block{}, errors.New("no ':' between the file and the position")
	}
	name, rest := text[:colon], text[colon+1:]
	fields := strings.Fields(rest)
	if len(fields) != 3 {
		return Block{}, errors.New("want <file>:<position> <statements> <count>")
	}
	start, end, ok := strings.Cut(fields[0], ",")
	if !ok {
		return Block{}, fmt.Errorf("position %q is not <line>.<col>,<line>.<col>", fields[0])
	}
	startLine, startCol, err := lineCol(start)
	if err != nil {
		return Block{}, err
	}
	endLine, endCol, err := lineCol(end)
	if err != nil {
		return Block{}, err
	}
	statements, err := strconv.Atoi(fields[1])
	if err != nil || statements < 0 {
		return Block{}, fmt.Errorf("statement count %q is not a non-negative number", fields[1])
	}
	count, err := strconv.ParseUint(fields[2], 10, 64)
	if err != nil {
		return Block{}, fmt.Errorf("hit count %q is not a non-negative number", fields[2])
	}
	path, ok := strings.CutPrefix(name, module+"/")
	if !ok || !filepath.IsLocal(path) {
		return Block{}, &OutsideError{Name: name}
	}
	return Block{
		Path:      path,
		StartLine: startLine, StartCol: startCol,
		EndLine: endLine, EndCol: endCol,
		Statements: statements,
		Covered:    count > 0,
	}, nil
}

func lineCol(text string) (int, int, error) {
	l, c, ok := strings.Cut(text, ".")
	if !ok {
		return 0, 0, fmt.Errorf("position %q is not <line>.<col>", text)
	}
	line, err := strconv.Atoi(l)
	if err != nil || line < 1 {
		return 0, 0, fmt.Errorf("line %q is not a positive number", l)
	}
	col, err := strconv.Atoi(c)
	if err != nil || col < 0 {
		return 0, 0, fmt.Errorf("column %q is not a non-negative number", c)
	}
	return line, col, nil
}

// modulePath reads the module path from root's go.mod.
func modulePath(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod")) //nolint:gosec // go.mod at the repo root the lint runs in
	if err != nil {
		return "", fmt.Errorf("read go.mod for the module path: %w", err)
	}
	module := modfile.ModulePath(data)
	if module == "" {
		return "", errors.New("go.mod has no module line")
	}
	return module, nil
}

// sortedReport returns the blocks sorted by path then position.
func sortedReport(blocks map[blockKey]*Block) Report {
	if len(blocks) == 0 {
		return Report{}
	}
	out := make([]Block, 0, len(blocks))
	for _, b := range blocks {
		out = append(out, *b)
	}
	slices.SortFunc(out, func(a, b Block) int {
		return cmp.Or(
			strings.Compare(a.Path, b.Path),
			cmp.Compare(a.StartLine, b.StartLine),
			cmp.Compare(a.StartCol, b.StartCol),
			cmp.Compare(a.EndLine, b.EndLine),
			cmp.Compare(a.EndCol, b.EndCol),
		)
	})
	return Report{Blocks: out}
}
