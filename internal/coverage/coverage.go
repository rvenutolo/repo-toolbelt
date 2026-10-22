// Package coverage reads the coverage reports a consumer's own tests write, Go
// cover profiles and Cobertura XML, into one shape: blocks of statements in
// repo-relative files, each covered or not. It also holds the coverage
// baseline file and the config section the coverage lints share.
package coverage

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// ErrNotFound is the report file not existing.
var ErrNotFound = errors.New("coverage report not found")

// Report formats, as the config section names them.
const (
	FormatGo        = "go"
	FormatCobertura = "cobertura"
)

// Block is a run of statements a report scores as one: a Go cover block, or
// a single Cobertura line holding one statement.
type Block struct {
	// Path is the file, relative to the repo root, with forward slashes.
	Path string
	// StartLine and EndLine are 1-based and inclusive.
	StartLine, EndLine int
	// StartCol and EndCol are the Go profile's columns; 0 for Cobertura.
	StartCol, EndCol int
	// Statements is how many statements the block holds.
	Statements int
	// Covered reports whether a test ran the block.
	Covered bool
}

// Report is the blocks of one or more reports, sorted by path then position,
// with no two blocks at the same position.
type Report struct {
	Blocks []Block
}

// OutsideError is a report naming a file that is not in the repository: a
// Go import path outside the module, or a Cobertura file outside the root.
type OutsideError struct {
	// Name is the file as the report names it.
	Name string
}

func (e *OutsideError) Error() string {
	return "names " + e.Name + ", which is outside the repository"
}

// ReadFile reads the report at the repo-relative path spec names, in spec's
// format. A missing report is an error wrapping ErrNotFound; one naming a
// file outside the repository is an *OutsideError. Symlinks are followed, as
// kcov links its report directory by absolute path, but only to a file inside
// the repository: only the repository is mounted where the lints run.
func ReadFile(root string, spec ReportSpec) (Report, error) {
	var read func(root string, r io.Reader) (Report, error)
	switch spec.Format {
	case FormatGo:
		read = ReadGo
	case FormatCobertura:
		read = ReadCobertura
	default:
		return Report{}, fmt.Errorf("unknown report format %q", spec.Format)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return Report{}, fmt.Errorf("resolve repository root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(realRoot, spec.Path))
	if errors.Is(err, fs.ErrNotExist) {
		return Report{}, fmt.Errorf("%s: %w", spec.Path, ErrNotFound)
	}
	if err != nil {
		return Report{}, fmt.Errorf("resolve %s: %w", spec.Path, err)
	}
	if rel, err := filepath.Rel(realRoot, resolved); err != nil || !filepath.IsLocal(rel) {
		return Report{}, fmt.Errorf("%s resolves to %s, outside the repository", spec.Path, resolved)
	}
	f, err := os.Open(resolved)
	if err != nil {
		return Report{}, fmt.Errorf("open %s: %w", spec.Path, err)
	}
	defer f.Close()
	report, err := read(root, f)
	if err != nil {
		return Report{}, fmt.Errorf("read %s: %w", spec.Path, err)
	}
	return report, nil
}
