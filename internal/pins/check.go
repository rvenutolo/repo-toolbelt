// Package pins implements the check-toolbelt-pins lint: every repo-toolbelt
// image reference and shared-workflow reference in a repo's workflows and git
// hooks is pinned, and they all name the same version.
package pins

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
)

var (
	// imageRef matches the image name and whatever token characters follow it.
	imageRef = regexp.MustCompile(`ghcr\.io/rvenutolo/repo-toolbelt([^\s'"]*)`)
	// pinnedSuffix is what follows the image name in a pinned reference.
	pinnedSuffix = regexp.MustCompile(`^:([^@\s]+)@sha256:[0-9a-f]{64}$`)
	// usesRef matches a shared-workflow or action reference: the ref, then an
	// optional trailing comment.
	usesRef = regexp.MustCompile(`\buses:\s*['"]?rvenutolo/repo-toolbelt(?:/[^@\s'"]*)?@([^\s'"#]+)['"]?(?:\s+#\s*(\S*))?`)
	sha40   = regexp.MustCompile(`^[0-9a-f]{40}$`)
	version = regexp.MustCompile(`^v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$`)
)

// image is a pinned image reference seen in a file.
type image struct {
	ref, tag string
}

// use is a shared-workflow reference by SHA with a version comment.
type use struct {
	path    string
	line    int
	version string
}

// Check reads the files under .github/workflows and .githooks as they are on
// disk and reports every pin that is not tag-plus-digest or SHA-plus-version,
// every image reference that differs from the first, and every shared-workflow
// version that differs from the image tag. A repo with no reference at all is
// a finding.
func Check(_ context.Context, lc lintkit.Context) ([]lintkit.Finding, error) {
	root, err := os.OpenRoot(lc.Root)
	if err != nil {
		return nil, fmt.Errorf("open repository root: %w", err)
	}
	defer root.Close()
	var files []string
	// The repo-relative directories whose regular files are scanned.
	for _, dir := range []string{".github/workflows", ".githooks"} {
		names, err := regularFiles(root, dir)
		if err != nil {
			return nil, err
		}
		files = append(files, names...)
	}
	slices.Sort(files)

	var (
		findings []lintkit.Finding
		first    *image
		uses     []use
		seen     bool
	)
	for _, file := range files {
		content, err := root.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}
		for i, text := range strings.Split(string(content), "\n") {
			line := i + 1
			for _, m := range imageRef.FindAllStringSubmatch(text, -1) {
				suffix := m[1]
				if suffix != "" && suffix[0] != ':' && suffix[0] != '@' {
					continue // a different image whose name starts with this one
				}
				seen = true
				pm := pinnedSuffix.FindStringSubmatch(suffix)
				if pm == nil {
					findings = append(findings, lintkit.Finding{Path: file, Line: line, Message: "image not pinned by tag and digest"})
					continue
				}
				img := image{ref: m[0], tag: pm[1]}
				switch {
				case first == nil:
					first = &img
				case img.ref != first.ref:
					findings = append(findings, lintkit.Finding{
						Path: file, Line: line,
						Message: fmt.Sprintf("image reference %s differs from %s", img.ref, first.ref),
					})
				}
			}
			for _, m := range usesRef.FindAllStringSubmatch(text, -1) {
				seen = true
				switch {
				case !sha40.MatchString(m[1]):
					findings = append(findings, lintkit.Finding{Path: file, Line: line, Message: "uses: not pinned by a full commit SHA"})
				case !version.MatchString(m[2]):
					findings = append(findings, lintkit.Finding{Path: file, Line: line, Message: "uses: has no # vX.Y.Z version comment"})
				default:
					uses = append(uses, use{path: file, line: line, version: m[2]})
				}
			}
		}
	}
	if !seen {
		return []lintkit.Finding{{Message: "no repo-toolbelt pin found"}}, nil
	}
	if first != nil {
		for _, u := range uses {
			if u.version != first.tag {
				findings = append(findings, lintkit.Finding{
					Path: u.path, Line: u.line,
					Message: fmt.Sprintf("uses: version %s differs from image tag %s", u.version, first.tag),
				})
			}
		}
	}
	return findings, nil
}

// regularFiles lists the regular files directly in dir, as repo-relative
// paths. A missing directory has none.
func regularFiles(root *os.Root, dir string) ([]string, error) {
	d, err := root.Open(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dir, err)
	}
	defer d.Close()
	entries, err := d.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if e.Type().IsRegular() {
			files = append(files, path.Join(dir, e.Name()))
		}
	}
	return files, nil
}
