package patchcov_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
	"github.com/rvenutolo/repo-toolbelt/internal/patchcov"
)

// sentinel sits in a comment of the fixture source. A finding names files,
// lines and counts, never source text.
const sentinel = "markerSentinelValue"

// section names the fixture reports.
const section = `{"reports":[{"format":"go","path":"cov/go.out"},{"format":"cobertura","path":"cov/c.xml"}]}`

// aGo has three blocks: lines 4-5 (run), the if body 5-7 (never run) and
// the final return on 8 (run). An if body's block starts on the if line, so
// line 5 belongs to two blocks.
const aGo = `package m

// Pick is a fixture. ` + sentinel + `
func Pick(x int) int {
	if x < 0 {
		return 1
	}
	return 2
}
`

const goProfile = `mode: set
example.com/m/a.go:4.22,5.11 1 1
example.com/m/a.go:5.11,7.3 1 0
example.com/m/a.go:8.2,8.10 1 1
`

// tSh is scored by the Cobertura report: line 2 run, 3 and 4 never run.
const tSh = "#!/usr/bin/env bash\necho one\necho two\necho three\n"

// newFixture commits a module holding aGo, b.go (no blocks) and lib/t.sh,
// writes the reports untracked, and returns the root and the base commit.
func newFixture(t *testing.T) (string, string) {
	t.Helper()
	root, base := newRepo(t, map[string]string{
		"go.mod":   "module example.com/m\n\ngo 1.22\n",
		"a.go":     aGo,
		"b.go":     "package m\n\n// B is a fixture.\nconst B = 1\n",
		"lib/t.sh": tSh,
	})
	writeFile(t, filepath.Join(root, "cov", "go.out"), goProfile)
	writeFile(t, filepath.Join(root, "cov", "c.xml"), `<coverage><sources><source>`+root+`</source></sources>`+
		`<packages><package name="p"><classes><class filename="lib/t.sh"><lines>`+
		`<line number="2" hits="3"/><line number="3" hits="0"/><line number="4" hits="0"/>`+
		`</lines></class></classes></package></packages></coverage>`)
	return root, base
}

// setLine replaces line n (1-based) of the file at root/name.
func setLine(t *testing.T, root, name string, n int, text string) {
	t.Helper()
	path := filepath.Join(root, name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	lines[n-1] = text
	writeFile(t, path, strings.Join(lines, "\n"))
}

func check(t *testing.T, root, sectionJSON, base string) ([]lintkit.Finding, error) {
	t.Helper()
	return patchcov.Check(t.Context(), lintkit.Context{Root: root, Section: []byte(sectionJSON), Hook: "ci", Base: base})
}

func diffFindings(t *testing.T, want, got []lintkit.Finding) {
	t.Helper()
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(lintkit.Finding{})); diff != "" {
		t.Errorf("Check mismatch (-want +got):\n%s", diff)
	}
	for _, f := range got {
		if strings.Contains(f.Message, sentinel) {
			t.Errorf("finding %q quotes source text", f.Message)
		}
	}
}

var unrunBlock = lintkit.Finding{Path: "a.go", Line: 5, EndLine: 7, Message: "changed block not covered (1 statement)"}

func TestCheck(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		change func(t *testing.T, root string)
		want   []lintkit.Finding
	}{
		{name: "an empty diff", change: func(*testing.T, string) {}},
		{
			name: "a change to covered code",
			change: func(t *testing.T, root string) {
				t.Helper()
				setLine(t, root, "a.go", 8, "\treturn 3")
				setLine(t, root, "lib/t.sh", 2, "echo uno")
			},
		},
		{
			name: "an uncommitted change to a block never run",
			change: func(t *testing.T, root string) {
				t.Helper()
				setLine(t, root, "a.go", 6, "\t\treturn 9")
			},
			want: []lintkit.Finding{unrunBlock},
		},
		{
			name: "a committed change to a block never run",
			change: func(t *testing.T, root string) {
				t.Helper()
				setLine(t, root, "a.go", 6, "\t\treturn 9")
				git(t, root, "commit", "--quiet", "--all", "--message", "next")
			},
			want: []lintkit.Finding{unrunBlock},
		},
		{
			name: "a changed block that is exempt with a reason",
			change: func(t *testing.T, root string) {
				t.Helper()
				setLine(t, root, "a.go", 5, "\tif x < 0 { // coverage-exempt: negative input is rejected upstream")
				setLine(t, root, "a.go", 6, "\t\treturn 9")
			},
		},
		{
			name: "a whole-line marker inside the block exempts it",
			change: func(t *testing.T, root string) {
				t.Helper()
				setLine(t, root, "a.go", 7, "\t} // coverage-exempt: negative input is rejected upstream")
				setLine(t, root, "a.go", 6, "\t\treturn 9")
			},
		},
		{
			name: "a marker with no reason exempts nothing",
			change: func(t *testing.T, root string) {
				t.Helper()
				setLine(t, root, "a.go", 5, "\tif x < 0 { // coverage-exempt:")
				setLine(t, root, "a.go", 6, "\t\treturn 9")
			},
			want: []lintkit.Finding{
				{Path: "a.go", Line: 5, Message: "coverage-exempt marker gives no reason"},
				unrunBlock,
			},
		},
		{
			name: "markers are judged in unchanged files and outside every block",
			change: func(t *testing.T, root string) {
				t.Helper()
				setLine(t, root, "a.go", 2, "// coverage-exempt: not code")
				git(t, root, "commit", "--quiet", "--all", "--message", "next")
				writeFile(t, filepath.Join(root, "b_test.go"), "package m\n\n// coverage-exempt: tests are never profiled\n")
				git(t, root, "add", "b_test.go")
				git(t, root, "commit", "--quiet", "--message", "test")
				// Untracked files are not judged.
				writeFile(t, filepath.Join(root, "c.go"), "package m\n\n// coverage-exempt: untracked\n")
			},
			want: []lintkit.Finding{
				{Path: "a.go", Line: 2, Message: "coverage-exempt marker is not inside any measured block"},
				{Path: "b_test.go", Line: 3, Message: "coverage-exempt marker is not inside any measured block"},
			},
		},
		{
			name: "a marker on covered code is stale",
			change: func(t *testing.T, root string) {
				t.Helper()
				setLine(t, root, "a.go", 8, "\treturn 2 // coverage-exempt: no longer needed")
			},
			want: []lintkit.Finding{{Path: "a.go", Line: 8, Message: "coverage-exempt marker is stale: the code it marks is covered"}},
		},
		{
			name: "a changed Cobertura line never run",
			change: func(t *testing.T, root string) {
				t.Helper()
				setLine(t, root, "lib/t.sh", 3, "echo dos")
				setLine(t, root, "lib/t.sh", 4, "echo tres")
			},
			want: []lintkit.Finding{
				{Path: "lib/t.sh", Line: 3, Message: "changed line not covered"},
				{Path: "lib/t.sh", Line: 4, Message: "changed line not covered"},
			},
		},
		{
			name: "Cobertura markers: exempt, stale, without a reason, and outside every line",
			change: func(t *testing.T, root string) {
				t.Helper()
				setLine(t, root, "lib/t.sh", 1, "#!/usr/bin/env bash # coverage-exempt: a shebang is never scored")
				setLine(t, root, "lib/t.sh", 2, "echo one # coverage-exempt: stale")
				setLine(t, root, "lib/t.sh", 3, "echo dos # coverage-exempt: exempt")
				setLine(t, root, "lib/t.sh", 4, "echo tres # coverage-exempt:")
			},
			want: []lintkit.Finding{
				{Path: "lib/t.sh", Line: 1, Message: "coverage-exempt marker is not inside any measured block"},
				{Path: "lib/t.sh", Line: 2, Message: "coverage-exempt marker is stale: the code it marks is covered"},
				{Path: "lib/t.sh", Line: 4, Message: "coverage-exempt marker gives no reason"},
				{Path: "lib/t.sh", Line: 4, Message: "changed line not covered"},
			},
		},
		{
			name: "a # marker in a file no Cobertura report scores is not a marker",
			change: func(t *testing.T, root string) {
				t.Helper()
				writeFile(t, filepath.Join(root, "doc.md"), "# coverage-exempt: documented\n")
				git(t, root, "add", "doc.md")
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, base := newFixture(t)
			tc.change(t, root)
			got, err := check(t, root, section, base)
			if err != nil {
				t.Fatalf("Check error = %v", err)
			}
			diffFindings(t, tc.want, got)
		})
	}
}

func TestCheck_blockStatementCount(t *testing.T) {
	t.Parallel()
	root, base := newFixture(t)
	writeFile(t, filepath.Join(root, "cov", "go.out"), "mode: set\nexample.com/m/a.go:4.22,8.10 3 0\n")
	setLine(t, root, "a.go", 8, "\treturn 3")
	got, err := check(t, root, section, base)
	if err != nil {
		t.Fatalf("Check error = %v", err)
	}
	diffFindings(t, []lintkit.Finding{{Path: "a.go", Line: 4, EndLine: 8, Message: "changed block not covered (3 statements)"}}, got)
}

func TestCheck_emptyBlockIsNotJudged(t *testing.T) {
	t.Parallel()
	root, base := newFixture(t)
	writeFile(t, filepath.Join(root, "cov", "go.out"), goProfile+"example.com/m/a.go:9.1,9.1 0 0\n")
	setLine(t, root, "a.go", 9, "} // coverage-exempt: an empty block is not measured")
	got, err := check(t, root, section, base)
	if err != nil {
		t.Fatalf("Check error = %v", err)
	}
	diffFindings(t, []lintkit.Finding{{Path: "a.go", Line: 9, Message: "coverage-exempt marker is not inside any measured block"}}, got)
}

func TestCheck_trackedSymlinkIsNotScanned(t *testing.T) {
	t.Parallel()
	root, base := newFixture(t)
	writeFile(t, filepath.Join(root, "notes.txt"), "// coverage-exempt: not Go\n")
	if err := os.Symlink("notes.txt", filepath.Join(root, "link.go")); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "notes.txt", "link.go")
	got, err := check(t, root, section, base)
	if err != nil {
		t.Fatalf("Check error = %v", err)
	}
	diffFindings(t, nil, got)
}

func TestCheck_vendoredMarkersAreNotJudged(t *testing.T) {
	t.Parallel()
	root, base := newFixture(t)
	writeFile(t, filepath.Join(root, "vendor", "example.com", "lib", "lib.go"),
		"package lib\n\n// coverage-exempt: upstream code\nfunc F() {}\n")
	writeFile(t, filepath.Join(root, ".gitattributes"), "third_party/** linguist-vendored\n")
	writeFile(t, filepath.Join(root, "third_party", "x.go"), "package x\n\n// coverage-exempt: upstream code\nfunc G() {}\n")
	git(t, root, "add", ".gitattributes", "vendor", "third_party")
	got, err := check(t, root, section, base)
	if err != nil {
		t.Fatalf("Check error = %v", err)
	}
	diffFindings(t, nil, got)
}

func TestCheck_measuredVendoredCodeIsNotJudged(t *testing.T) {
	t.Parallel()
	root, base := newRepo(t, map[string]string{
		"go.mod":           "module example.com/m\n\ngo 1.22\n",
		".gitattributes":   "third_party/** linguist-vendored\n",
		"third_party/x.go": aGo,
	})
	writeFile(t, filepath.Join(root, "cov", "go.out"), "mode: set\n"+
		"example.com/m/third_party/x.go:4.22,5.11 1 1\n"+
		"example.com/m/third_party/x.go:5.11,7.3 1 0\n"+
		"example.com/m/third_party/x.go:8.2,8.10 1 1\n")
	setLine(t, root, "third_party/x.go", 6, "\t\treturn 3")
	got, err := check(t, root, `{"reports":[{"format":"go","path":"cov/go.out"}]}`, base)
	if err != nil {
		t.Fatalf("Check error = %v", err)
	}
	diffFindings(t, nil, got)
}

func TestCheck_trackedFileReplacedByAFIFOIsNotRead(t *testing.T) {
	t.Parallel()
	root, base := newFixture(t)
	pipe := filepath.Join(root, "pipe.go")
	writeFile(t, pipe, "package m\n\n// coverage-exempt: in a file now a FIFO\n")
	git(t, root, "add", "pipe.go")
	if err := os.Remove(pipe); err != nil {
		t.Fatal(err)
	}
	// Opening a FIFO for reading blocks until a writer appears, so reading
	// it would hang the lint.
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := check(t, root, section, base)
	if err != nil {
		t.Fatalf("Check error = %v", err)
	}
	diffFindings(t, nil, got)
}

func TestCheck_reportFindings(t *testing.T) {
	t.Parallel()
	t.Run("a missing report", func(t *testing.T) {
		t.Parallel()
		root, base := newFixture(t)
		if err := os.Remove(filepath.Join(root, "cov", "c.xml")); err != nil {
			t.Fatal(err)
		}
		got, err := check(t, root, section, base)
		if err != nil {
			t.Fatalf("Check error = %v", err)
		}
		diffFindings(t, []lintkit.Finding{{Path: "cov/c.xml", Message: "coverage report not found"}}, got)
	})
	t.Run("a report naming a file that does not exist", func(t *testing.T) {
		t.Parallel()
		root, base := newFixture(t)
		writeFile(t, filepath.Join(root, "cov", "go.out"), goProfile+"example.com/m/gone.go:1.1,2.2 1 1\n")
		got, err := check(t, root, section, base)
		if err != nil {
			t.Fatalf("Check error = %v", err)
		}
		diffFindings(t, []lintkit.Finding{{Path: "gone.go", Message: "a coverage report names this file, which does not exist; rerun the tests"}}, got)
	})
	t.Run("a section problem", func(t *testing.T) {
		t.Parallel()
		root, base := newFixture(t)
		got, err := check(t, root, "null", base)
		if err != nil {
			t.Fatalf("Check error = %v", err)
		}
		diffFindings(t, []lintkit.Finding{lintkit.SectionError("", "reports is missing or empty")}, got)
	})
}

func TestCheck_setupFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		base    string
		section string // "" for the fixture's section
		setup   func(t *testing.T, root string)
		want    string // part of the error
	}{
		{name: "an empty base", base: "", want: "TOOLBELT_BASE is empty"},
		{name: "a section that is not JSON", base: "HEAD", section: "{", want: "parse config section"},
		{name: "a base that is not a commit", base: "not-a-commit", want: "TOOLBELT_BASE not-a-commit is not a commit"},
		{name: "a base that names a tree", base: "HEAD^{tree}", want: "TOOLBELT_BASE HEAD^{tree} is not a commit"},
		{name: "a malformed Go profile", base: "HEAD", want: "read cov/go.out", setup: func(t *testing.T, root string) {
			t.Helper()
			writeFile(t, filepath.Join(root, "cov", "go.out"), "not a profile\n")
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, _ := newFixture(t)
			if tc.setup != nil {
				tc.setup(t, root)
			}
			sectionJSON := section
			if tc.section != "" {
				sectionJSON = tc.section
			}
			got, err := check(t, root, sectionJSON, tc.base)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Check = %v, %v; want an error containing %q", got, err, tc.want)
			}
		})
	}
}

func TestCheck_unreadableFiles(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads files whatever their mode")
	}
	tests := []struct {
		name string
		path string // made unreadable, relative to the root
		want string // part of the error
	}{
		{name: "an unreadable tracked Go file", path: "b.go", want: "read b.go"},
		{name: "an unsearchable directory holding a tracked Go file", path: "sub", want: "stat sub/c.go"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, base := newFixture(t)
			writeFile(t, filepath.Join(root, "sub", "c.go"), "package sub\n")
			git(t, root, "add", "sub/c.go")
			git(t, root, "commit", "--quiet", "--message", "sub")
			target := filepath.Join(root, tc.path)
			if err := os.Chmod(target, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(target, 0o755) }) //nolint:errcheck // lets TempDir clean up; a failure only leaves the dir
			if got, err := check(t, root, section, base); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Check = %v, %v; want an error containing %q", got, err, tc.want)
			}
		})
	}
}

func TestCheck_trackedFileUnderAFileIsSkipped(t *testing.T) {
	t.Parallel()
	root, base := newFixture(t)
	writeFile(t, filepath.Join(root, "sub", "c.go"), "package sub\n\n// coverage-exempt: gone with its directory\n")
	git(t, root, "add", "sub/c.go")
	git(t, root, "commit", "--quiet", "--message", "sub")
	if err := os.RemoveAll(filepath.Join(root, "sub")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "sub"), "now a file\n")
	got, err := check(t, root, section, base)
	if err != nil {
		t.Fatalf("Check error = %v", err)
	}
	diffFindings(t, nil, got)
}

func TestCheck_notARepository(t *testing.T) {
	t.Parallel()
	t.Run("a root that does not exist", func(t *testing.T) {
		t.Parallel()
		if got, err := check(t, filepath.Join(t.TempDir(), "missing"), section, "HEAD"); err == nil || !strings.Contains(err.Error(), "open repository root") {
			t.Errorf("Check = %v, %v; want an error opening the repository root", got, err)
		}
	})
	t.Run("a root that is not a git repository", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "go.mod"), "module example.com/m\n")
		writeFile(t, filepath.Join(root, "a.go"), aGo)
		writeFile(t, filepath.Join(root, "lib", "t.sh"), tSh)
		writeFile(t, filepath.Join(root, "cov", "go.out"), goProfile)
		writeFile(t, filepath.Join(root, "cov", "c.xml"), `<coverage><packages/></coverage>`)
		if got, err := check(t, root, section, "HEAD"); err == nil || !strings.Contains(err.Error(), "not a git repository") {
			t.Errorf("Check = %v, %v; want git's not-a-repository error", got, err)
		}
	})
}

// TestCheck_diffFails has no t.Parallel: it puts a fake git first on PATH,
// which runs the real one for everything but a diff.
func TestCheck_diffFails(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root, base := newFixture(t)
	setLine(t, root, "a.go", 6, "\t\treturn 9")
	bin := t.TempDir()
	writeFile(t, filepath.Join(bin, "git"), "#!/bin/sh\ncase \" $* \" in\n*' diff '*) exit 3 ;;\nesac\nexec '"+realGit+"' \"$@\"\n")
	if err := os.Chmod(filepath.Join(bin, "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if got, err := check(t, root, section, base); err == nil || !strings.Contains(err.Error(), "git diff") {
		t.Errorf("Check = %v, %v; want the git diff failure", got, err)
	}
}
