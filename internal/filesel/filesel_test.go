package filesel_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/rvenutolo/repo-toolbelt/internal/filesel"
	"github.com/rvenutolo/repo-toolbelt/internal/manifest"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newRepo returns a git repository whose commit tracks files, a map of
// repo-relative path to content.
func newRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	repo := t.TempDir()
	gitIn(t, repo, "init", "--quiet", "--initial-branch=main")
	for name, content := range files {
		writeFile(t, filepath.Join(repo, name), content)
	}
	commitAll(t, repo)
	return repo
}

func commitAll(t *testing.T, repo string) {
	t.Helper()
	gitIn(t, repo, "add", "--all")
	gitIn(t, repo, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "--quiet", "--message=m")
}

// formatters are the fixture formatters: shell owns shell files and bash
// scripts, perl owns perl files and the extensionless file "claimed".
var formatters = []manifest.Formatter{
	{
		Name: "shell", Language: "bash", Speed: "quick", Description: "shell",
		Include:  []string{"*.sh", ".envrc", ".envrc.*"},
		Shebangs: []string{"bash"},
	},
	{
		Name: "perl", Language: "bash", Speed: "quick", Description: "perl",
		Include:  []string{"*.pl", "claimed"},
		Shebangs: []string{"perl"},
	},
}

func selectFiles(t *testing.T, repo string) map[string][]string {
	t.Helper()
	got, err := filesel.Select(t.Context(), repo, formatters)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	return got
}

// assertSelected compares got with want, where a formatter want omits has no
// files.
func assertSelected(t *testing.T, got, want map[string][]string) {
	t.Helper()
	full := make(map[string][]string, len(formatters))
	for _, f := range formatters {
		full[f.Name] = want[f.Name]
	}
	if diff := cmp.Diff(full, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("Select (-want +got):\n%s", diff)
	}
}

func TestSelect_includeAndShebang(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, map[string]string{
		"a.sh":                    "echo a\n",
		"dir/b.sh":                "echo b\n",
		"c.pl":                    "print 1;\n",
		".envrc":                  "export A=1\n",
		".envrc.local":            "export B=2\n",
		"readme.txt":              "text\n",
		"bin/direct":              "#!/bin/bash\necho\n",
		"bin/viaenv":              "#!/usr/bin/env bash\necho\n",
		"bin/viaenv-s":            "#!/usr/bin/env -S bash -e\necho\n",
		"bin/viaenv-assign":       "#!/usr/bin/env FOO=1 bash\necho\n",
		"bin/spaced":              "#! /usr/local/bin/bash -eu\necho\n",
		"bin/nonewline":           "#!/bin/bash",
		"bin/perlscript":          "#!/usr/bin/perl\n",
		"bin/sh":                  "#!/bin/sh\necho\n",
		"bin/bashful":             "#!/usr/bin/env bashful\n",
		"bin/envonly":             "#!/usr/bin/env\n",
		"bin/emptyshebang":        "#!\n",
		"bin/plain":               "echo no shebang\n",
		"bin/second-line":         "\n#!/bin/bash\n",
		"bin/claimed":             "#!/bin/bash\n",
		"claimed":                 "#!/bin/bash\n",
		"bin/has.ext":             "#!/bin/bash\n",
		"bin/.hidden":             "#!/bin/bash\n",
		"bin/.hidden.local":       "#!/bin/bash\n",
		"notes.txt":               "#!/bin/bash\n",
		"vendored.sh":             "echo\n",
		"myvendor/x.sh":           "echo\n",
		"bin/shebang-after-space": " #!/bin/bash\n",
	})

	got := selectFiles(t, repo)

	assertSelected(t, got, map[string][]string{
		"shell": {
			".envrc", ".envrc.local", "a.sh", "bin/.hidden", "bin/direct", "bin/nonewline", "bin/spaced",
			"bin/viaenv", "bin/viaenv-assign", "bin/viaenv-s", "dir/b.sh", "myvendor/x.sh", "vendored.sh",
		},
		"perl": {"bin/claimed", "bin/perlscript", "c.pl", "claimed"},
	})
}

func TestSelect_everyFormatterIsAKey(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, map[string]string{"readme.txt": "x\n"})

	got := selectFiles(t, repo)

	if len(got) != len(formatters) {
		t.Fatalf("keys = %v, want one per formatter", got)
	}
	for _, f := range formatters {
		if files, ok := got[f.Name]; !ok || len(files) != 0 {
			t.Errorf("got[%q] = %v, %v; want present and empty", f.Name, files, ok)
		}
	}
}

func TestSelect_resultIsSorted(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, map[string]string{"b.sh": "", "a/z.sh": "", "a.sh": "", "B.sh": "", "a/b.sh": ""})

	got := selectFiles(t, repo)

	assertSelected(t, got, map[string][]string{"shell": {"B.sh", "a.sh", "a/b.sh", "a/z.sh", "b.sh"}})
}

func TestSelect_skipsUntrackedDeletedSymlinkAndSubmodule(t *testing.T) {
	t.Parallel()
	sub := newRepo(t, map[string]string{"inner.sh": "echo\n"})
	repo := newRepo(t, map[string]string{"keep.sh": "echo\n", "gone.sh": "echo\n", "outside.sh": "echo\n"})
	// A symlink is tracked as its target text; formatting it would rewrite
	// the target.
	if err := os.Symlink("keep.sh", filepath.Join(repo, "link.sh")); err != nil {
		t.Fatal(err)
	}
	gitIn(t, repo, "add", "link.sh")
	gitIn(t, repo, "-c", "protocol.file.allow=always", "submodule", "add", "--quiet", sub, "lib.sh")
	commitAll(t, repo)
	writeFile(t, filepath.Join(repo, "untracked.sh"), "echo\n")
	if err := os.Remove(filepath.Join(repo, "gone.sh")); err != nil {
		t.Fatal(err)
	}

	got := selectFiles(t, repo)

	assertSelected(t, got, map[string][]string{"shell": {"keep.sh", "outside.sh"}})
}

func TestSelect_skipsADirectoryReplacingATrackedFile(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, map[string]string{"a.sh": "echo\n", "keep.sh": "echo\n"})
	if err := os.Remove(filepath.Join(repo, "a.sh")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, "a.sh", "inner"), "x\n")

	got := selectFiles(t, repo)

	assertSelected(t, got, map[string][]string{"shell": {"keep.sh"}})
}

func TestSelect_skipsAFileWhoseParentBecameAFile(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, map[string]string{"d/a.sh": "echo\n", "keep.sh": "echo\n"})
	if err := os.RemoveAll(filepath.Join(repo, "d")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, "d"), "now a file\n")

	got := selectFiles(t, repo)

	assertSelected(t, got, map[string][]string{"shell": {"keep.sh"}})
}

func TestSelect_vendoredFilesAreSkipped(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, map[string]string{
		"vendor/a.sh":          "echo\n",
		"a/vendor/b.sh":        "echo\n",
		"a/vendor/deep/c.sh":   "echo\n",
		"vendor/bin/run":       "#!/bin/bash\n",
		"vendored.sh":          "echo\n",
		"myvendor/x.sh":        "echo\n",
		"vendor.sh":            "echo\n",
		"x/vendor":             "#!/bin/bash\n",
		".gitattributes":       "third_party/** linguist-vendored\nbare/** linguist-vendored=true\nkept/** linguist-vendored=false\nunset/** -linguist-vendored\n",
		"third_party/x.sh":     "echo\n",
		"third_party/bin/tool": "#!/bin/bash\n",
		"bare/x.sh":            "echo\n",
		"kept/x.sh":            "echo\n",
		"unset/x.sh":           "echo\n",
		"plain/x.sh":           "echo\n",
	})

	got := selectFiles(t, repo)

	assertSelected(t, got, map[string][]string{
		"shell": {"kept/x.sh", "myvendor/x.sh", "plain/x.sh", "unset/x.sh", "vendor.sh", "vendored.sh", "x/vendor"},
	})
}

func TestSelect_namesAreSelectedIntact(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, map[string]string{
		"with space/a b.sh": "echo\n",
		"new\nline.sh":      "echo\n",
		"tab\there.sh":      "echo\n",
		"quote\"d.sh":       "echo\n",
		"üñí.sh":            "echo\n",
		"-dash.sh":          "echo\n",
		"star*.sh":          "echo\n",
		"bin/with space":    "#!/bin/bash\n",
	})

	got := selectFiles(t, repo)

	assertSelected(t, got, map[string][]string{
		"shell": {"-dash.sh", "bin/with space", "new\nline.sh", "quote\"d.sh", "star*.sh", "tab\there.sh", "with space/a b.sh", "üñí.sh"},
	})
}

func TestSelect_shebangPrefixCases(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, map[string]string{
		"bin/fits":    "#!/bin/bash\n" + strings.Repeat("x", 500),
		"bin/exact":   "#!/usr/bin/env bash\n",
		"bin/long":    "#!/bin/" + strings.Repeat("a", 300) + "/bash\n",
		"bin/longbin": "#!" + strings.Repeat("\x00", 400),
		"bin/binary":  "\x7fELF\x00\x01\x02\x03",
		"bin/empty":   "",
		"keep.sh":     "echo\n",
	})

	got := selectFiles(t, repo)

	assertSelected(t, got, map[string][]string{"shell": {"bin/exact", "bin/fits", "keep.sh"}})
}

func TestSelect_shebangLineCutByThePrefixIsSkippedEvenWhenItNamesBashEarly(t *testing.T) {
	t.Parallel()
	// The interpreter sits in the first bytes, but the prefix cuts the line,
	// so the whole line is unknown.
	repo := newRepo(t, map[string]string{"bin/long": "#!/bin/bash " + strings.Repeat("a", 400) + "\n"})

	got := selectFiles(t, repo)

	assertSelected(t, got, nil)
}

func TestSelect_unreadableCandidate(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads any file")
	}
	repo := newRepo(t, map[string]string{"bin/secret": "#!/bin/bash\n", "keep.sh": "echo\n"})
	if err := os.Chmod(filepath.Join(repo, "bin", "secret"), 0); err != nil {
		t.Fatal(err)
	}

	_, err := filesel.Select(t.Context(), repo, formatters)

	if err == nil || !strings.Contains(err.Error(), "bin/secret") {
		t.Errorf("Select error = %v, want one naming bin/secret", err)
	}
}

func TestSelect_unstattableFile(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root stats any file")
	}
	repo := newRepo(t, map[string]string{"locked/a.sh": "echo\n", "keep.sh": "echo\n"})
	locked := filepath.Join(repo, "locked")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) }) //nolint:errcheck // restoring access so the TempDir cleanup can remove it

	_, err := filesel.Select(t.Context(), repo, formatters)

	if err == nil || !strings.Contains(err.Error(), "locked/a.sh") {
		t.Errorf("Select error = %v, want one naming locked/a.sh", err)
	}
}

func TestSelect_neverFollowsASymlinkOutOfTheRoot(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "evil.sh"), "echo\n")
	writeFile(t, filepath.Join(outside, "evil"), "#!/bin/bash\n")
	repo := newRepo(t, map[string]string{"keep.sh": "echo\n"})
	links := map[string]string{
		"dirlink":    outside,
		"scriptlink": filepath.Join(outside, "evil.sh"),
		"evillink":   filepath.Join(outside, "evil"),
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(repo, name)); err != nil {
			t.Fatal(err)
		}
	}
	commitAll(t, repo)

	got := selectFiles(t, repo)

	assertSelected(t, got, map[string][]string{"shell": {"keep.sh"}})
}

func TestSelect_errors(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, map[string]string{"a.sh": "echo\n"})
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	tests := []struct {
		name string
		ctx  context.Context //nolint:containedctx // a table of inputs for one call
		root string
	}{
		{name: "canceled context", ctx: canceled, root: repo},
		{name: "missing root", ctx: t.Context(), root: filepath.Join(repo, "missing")},
		{name: "not a git repository", ctx: t.Context(), root: t.TempDir()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := filesel.Select(tc.ctx, tc.root, formatters)
			if err == nil {
				t.Errorf("Select = %v, want an error", got)
			}
		})
	}
}

func TestSelect_repositoryWithoutTrackedFiles(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	gitIn(t, repo, "init", "--quiet", "--initial-branch=main")

	got := selectFiles(t, repo)

	assertSelected(t, got, nil)
}

func TestSelect_skipsAFileBehindAParentSymlinkInsideTheRoot(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, map[string]string{
		"in/a.sh": "echo\n", "e/a.sh": "echo\n", "e/tool": "#!/bin/bash\n", "in/tool": "#!/bin/bash\n", "keep.sh": "echo\n",
	})
	// The tracked directory e is now a symlink to in, so e/a.sh reaches the
	// same physical file as in/a.sh.
	if err := os.RemoveAll(filepath.Join(repo, "e")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("in", filepath.Join(repo, "e")); err != nil {
		t.Fatal(err)
	}

	got := selectFiles(t, repo)

	assertSelected(t, got, map[string][]string{"shell": {"in/a.sh", "in/tool", "keep.sh"}})
}

func TestSelect_skipsAFileBehindAParentSymlinkOutsideTheRoot(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "a.sh"), "echo\n")
	writeFile(t, filepath.Join(outside, "tool"), "#!/bin/bash\n")
	repo := newRepo(t, map[string]string{"d/a.sh": "echo\n", "d/tool": "#!/bin/bash\n", "keep.sh": "echo\n"})
	if err := os.RemoveAll(filepath.Join(repo, "d")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(repo, "d")); err != nil {
		t.Fatal(err)
	}

	got := selectFiles(t, repo)

	assertSelected(t, got, map[string][]string{"shell": {"keep.sh"}})
}

func TestSelect_conflictedFileIsListedOnce(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, map[string]string{"a.sh": "echo base\n", "keep.sh": "echo\n"})
	gitIn(t, repo, "checkout", "--quiet", "-b", "other")
	writeFile(t, filepath.Join(repo, "a.sh"), "echo other\n")
	commitAll(t, repo)
	gitIn(t, repo, "checkout", "--quiet", "main")
	writeFile(t, filepath.Join(repo, "a.sh"), "echo main\n")
	commitAll(t, repo)
	// The merge stops on the conflict in a.sh, leaving it at three index stages.
	cmd := exec.CommandContext(t.Context(), "git", "-c", "user.name=t", "-c", "user.email=t@example.com", "merge", "--quiet", "other")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("git merge succeeded, want a conflict\n%s", out)
	}

	got := selectFiles(t, repo)

	assertSelected(t, got, map[string][]string{"shell": {"a.sh", "keep.sh"}})
}

func TestSelect_aFileTheIncludesOfTwoFormattersMatchIsAnError(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, map[string]string{"a.sh": "echo\n", "b.sh": "echo\n"})
	// Distinct globs, which the manifest accepts, that both match a.sh.
	overlapping := []manifest.Formatter{
		{Name: "zsh", Language: "bash", Speed: "quick", Description: "zsh", Include: []string{"a*.sh"}},
		{Name: "shell", Language: "bash", Speed: "quick", Description: "shell", Include: []string{"*.sh"}},
	}

	got, err := filesel.Select(t.Context(), repo, overlapping)
	if err == nil {
		t.Fatalf("Select = %v, want an error", got)
	}

	if want := "a.sh: matched by the include globs of more than one formatter: shell, zsh"; !strings.Contains(err.Error(), want) {
		t.Errorf("Select error = %q, want it to hold %q", err, want)
	}
}

func TestSelect_noFormatters(t *testing.T) {
	t.Parallel()
	repo := newRepo(t, map[string]string{"a.sh": "echo\n"})

	got, err := filesel.Select(t.Context(), repo, nil)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("Select = %v, want an empty map", got)
	}
}

func TestClaimedByName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		file string
		want bool
	}{
		{name: "an include glob matches the base name", file: "dir/run.sh", want: true},
		{name: "another formatter's include is no claim", file: "dir/run.pl", want: false},
		{name: "a shebang alone is no claim by name", file: "bin/tool", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := filesel.ClaimedByName(formatters[0], tc.file); got != tc.want {
				t.Errorf("ClaimedByName(%s, %q) = %t, want %t", formatters[0].Name, tc.file, got, tc.want)
			}
		})
	}
}

func TestIsRegular(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "in", "a.sh"), "echo\n")
	writeFile(t, filepath.Join(root, "file"), "x\n")
	if err := os.Symlink("in", filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a.sh", filepath.Join(root, "in", "leaf.sh")); err != nil {
		t.Fatal(err)
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dir.Close() })
	tests := []struct {
		name string
		file string
		want bool
	}{
		{name: "a regular file", file: "in/a.sh", want: true},
		{name: "a missing file", file: "in/gone.sh", want: false},
		{name: "a file under a parent that is a file", file: "file/a.sh", want: false},
		{name: "a file under a parent that is an in-root symlink", file: "linked/a.sh", want: false},
		{name: "a symlink", file: "in/leaf.sh", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := filesel.IsRegular(dir, tc.file)
			if err != nil {
				t.Fatalf("IsRegular(%q) error = %v", tc.file, err)
			}
			if got != tc.want {
				t.Errorf("IsRegular(%q) = %t, want %t", tc.file, got, tc.want)
			}
		})
	}
}

func TestExclude(t *testing.T) {
	t.Parallel()
	files := []string{"a.sh", "test/fixtures/x.sh", "test/fixtures/deep/y.sh", "test/real.sh", "src/gen/a.sh", "src/a.sh"}
	tests := []struct {
		name  string
		globs []string
		want  []string
	}{
		{name: "no globs keeps all", globs: nil, want: files},
		{name: "directory glob", globs: []string{"test/fixtures"}, want: []string{"a.sh", "test/real.sh", "src/gen/a.sh", "src/a.sh"}},
		{name: "directory glob with double star", globs: []string{"test/fixtures/**"}, want: []string{"a.sh", "test/real.sh", "src/gen/a.sh", "src/a.sh"}},
		{name: "double star across segments", globs: []string{"**/gen/*.sh"}, want: []string{"a.sh", "test/fixtures/x.sh", "test/fixtures/deep/y.sh", "test/real.sh", "src/a.sh"}},
		{name: "single star stays within a segment", globs: []string{"test/*.sh"}, want: []string{"a.sh", "test/fixtures/x.sh", "test/fixtures/deep/y.sh", "src/gen/a.sh", "src/a.sh"}},
		{name: "several globs", globs: []string{"a.sh", "src"}, want: []string{"test/fixtures/x.sh", "test/fixtures/deep/y.sh", "test/real.sh"}},
		{name: "glob matching nothing", globs: []string{"nope/**"}, want: files},
		{name: "everything", globs: []string{"**"}, want: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := filesel.Exclude(files, tc.globs)
			if diff := cmp.Diff(tc.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("Exclude (-want +got):\n%s", diff)
			}
		})
	}
}

func TestExclude_doesNotModifyItsInput(t *testing.T) {
	t.Parallel()
	files := []string{"a.sh", "b.sh", "c.sh"}

	got := filesel.Exclude(files, []string{"a.sh"})

	if diff := cmp.Diff([]string{"a.sh", "b.sh", "c.sh"}, files); diff != "" {
		t.Errorf("input changed (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"b.sh", "c.sh"}, got); diff != "" {
		t.Errorf("Exclude (-want +got):\n%s", diff)
	}
}
