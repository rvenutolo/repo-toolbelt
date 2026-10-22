package coverage

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// class is one Cobertura <class>: a file and the lines scored in it.
type class struct {
	filename string
	lines    []Block // Path unset until the filename is resolved
}

// ReadCobertura reads a Cobertura XML report, as kcov and most other
// languages' tools write it. Each line it scores becomes a block of one
// statement. An absolute filename is made relative to root; a relative one is
// joined to the report's <source>, the first that holds the file when there
// are several, and a relative <source> is itself relative to root. <package>
// names are ignored. A line scored in more than one class is covered when any
// covers it. A file outside root is an *OutsideError.
func ReadCobertura(root string, r io.Reader) (Report, error) {
	sources, classes, err := decodeCobertura(r)
	if err != nil {
		return Report{}, err
	}
	root = filepath.Clean(root)
	blocks := map[blockKey]*Block{}
	for _, c := range classes {
		path, err := resolve(root, sources, c.filename)
		if err != nil {
			return Report{}, err
		}
		for _, l := range c.lines {
			l.Path = path
			key := blockKey{path: path, startLine: l.StartLine, endLine: l.EndLine}
			if seen, ok := blocks[key]; ok {
				seen.Covered = seen.Covered || l.Covered
				continue
			}
			blocks[key] = &l
		}
	}
	return sortedReport(blocks), nil
}

// decodeCobertura streams the document, keeping only the sources and the
// lines directly under each class: the lines under a class's methods repeat
// them.
func decodeCobertura(r io.Reader) ([]string, []class, error) {
	dec := xml.NewDecoder(r)
	var (
		stack   []string
		sources []string
		classes []class
		text    strings.Builder
		rooted  bool
	)
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("not valid XML: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			name := t.Name.Local
			if len(stack) == 0 && name != "coverage" {
				return nil, nil, fmt.Errorf("root element is <%s>, want <coverage>", name)
			}
			rooted = true
			stack = append(stack, name)
			switch {
			case name == "source":
				text.Reset()
			case name == "class":
				filename := attr(t, "filename")
				if filename == "" {
					return nil, nil, errors.New("a <class> has no filename")
				}
				classes = append(classes, class{filename: filename})
			case name == "line" && under(stack, "class", "lines"):
				block, err := parseLine(t, classes[len(classes)-1].filename)
				if err != nil {
					return nil, nil, err
				}
				classes[len(classes)-1].lines = append(classes[len(classes)-1].lines, block)
			}
		case xml.CharData:
			if len(stack) > 0 && stack[len(stack)-1] == "source" {
				text.Write(t)
			}
		case xml.EndElement:
			if t.Name.Local == "source" {
				sources = append(sources, strings.TrimSpace(text.String()))
			}
			stack = stack[:len(stack)-1]
		}
	}
	if !rooted {
		return nil, nil, errors.New("empty document, want a <coverage> report")
	}
	return sources, classes, nil
}

// under reports whether the element on top of stack sits directly in
// parent, which sits directly in grandparent.
func under(stack []string, grandparent, parent string) bool {
	n := len(stack)
	return n >= 3 && stack[n-3] == grandparent && stack[n-2] == parent
}

func attr(el xml.StartElement, name string) string {
	for _, a := range el.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func parseLine(el xml.StartElement, filename string) (Block, error) {
	number, err := strconv.Atoi(attr(el, "number"))
	if err != nil || number < 1 {
		return Block{}, fmt.Errorf("%s: line number %q is not a positive number", filename, attr(el, "number"))
	}
	hits, err := strconv.ParseUint(attr(el, "hits"), 10, 64)
	if err != nil {
		return Block{}, fmt.Errorf("%s: line %d: hits %q is not a non-negative number", filename, number, attr(el, "hits"))
	}
	return Block{StartLine: number, EndLine: number, Statements: 1, Covered: hits > 0}, nil
}

// resolve makes a report's filename relative to root.
func resolve(root string, sources []string, filename string) (string, error) {
	abs := filename
	if !filepath.IsAbs(filename) {
		abs = filepath.Join(sourceFor(root, sources, filename), filename)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return "", &OutsideError{Name: filename}
	}
	return filepath.ToSlash(rel), nil
}

// sourceFor picks the absolute source directory a relative filename is
// joined to: the first source holding the file, else the first source, else
// root.
func sourceFor(root string, sources []string, filename string) string {
	dirs := make([]string, 0, len(sources))
	for _, s := range sources {
		if !filepath.IsAbs(s) {
			s = filepath.Join(root, s)
		}
		dirs = append(dirs, s)
	}
	if len(dirs) == 0 {
		return root
	}
	if len(dirs) > 1 {
		for _, d := range dirs {
			if _, err := os.Stat(filepath.Join(d, filename)); err == nil {
				return d
			}
		}
	}
	return dirs[0]
}
