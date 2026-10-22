package pins_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
	"github.com/rvenutolo/repo-toolbelt/internal/pins"
)

const (
	digestA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sha     = "0123456789abcdef0123456789abcdef01234567"
	imgA    = "ghcr.io/rvenutolo/repo-toolbelt:v1.2.3@" + digestA
	imgB    = "ghcr.io/rvenutolo/repo-toolbelt:v1.2.4@" + digestB
	zeroSHA = "0000000000000000000000000000000000000000"
	zeroImg = "ghcr.io/rvenutolo/repo-toolbelt:v0.0.0@sha256:0000000000000000000000000000000000000000000000000000000000000000"
)

func write(t *testing.T, root, name, content string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, files map[string]string) []lintkit.Finding {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		write(t, root, name, content)
	}
	got, err := pins.Check(t.Context(), lintkit.Context{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func finding(path string, line int, msg string) lintkit.Finding {
	return lintkit.Finding{Path: path, Line: line, Message: msg}
}

func TestCheck(t *testing.T) {
	t.Parallel()
	hook := "#!/usr/bin/env bash\ndocker run " + imgA + " run\n"
	tests := []struct {
		name  string
		files map[string]string
		want  []lintkit.Finding
	}{
		{"pinned image in a hook passes", map[string]string{".githooks/pre-commit": hook}, nil},
		{"placeholder pair passes", map[string]string{
			".github/workflows/lint.yml": "jobs:\n  lint:\n    uses: rvenutolo/repo-toolbelt/.github/workflows/lint.yml@" + zeroSHA + " # v0.0.0\n    with:\n      image: " + zeroImg + "\n",
		}, nil},
		{"matching image and uses pass", map[string]string{
			".github/workflows/lint.yml": "    uses: rvenutolo/repo-toolbelt/.github/workflows/lint.yml@" + sha + " # v1.2.3\n    with:\n      image: " + imgA + "\n",
			".githooks/pre-push":         hook,
		}, nil},
		{"quoted uses passes", map[string]string{
			".github/workflows/lint.yml": "    - uses: 'rvenutolo/repo-toolbelt/.github/workflows/lint.yml@" + sha + "' # v1.2.3\n      image: " + imgA + "\n",
		}, nil},
		{
			"image without a tag",
			map[string]string{".githooks/pre-commit": "docker run ghcr.io/rvenutolo/repo-toolbelt run\n"},
			[]lintkit.Finding{finding(".githooks/pre-commit", 1, "image not pinned by tag and digest")},
		},
		{
			"image with a tag but no digest",
			map[string]string{".githooks/pre-commit": "x\ndocker run ghcr.io/rvenutolo/repo-toolbelt:v1.2.3 run\n"},
			[]lintkit.Finding{finding(".githooks/pre-commit", 2, "image not pinned by tag and digest")},
		},
		{
			"image with a short digest",
			map[string]string{".githooks/pre-commit": "docker run ghcr.io/rvenutolo/repo-toolbelt:v1.2.3@sha256:abc run\n"},
			[]lintkit.Finding{finding(".githooks/pre-commit", 1, "image not pinned by tag and digest")},
		},
		{
			"digest without a tag",
			map[string]string{".githooks/pre-commit": "docker run ghcr.io/rvenutolo/repo-toolbelt@" + digestA + " run\n"},
			[]lintkit.Finding{finding(".githooks/pre-commit", 1, "image not pinned by tag and digest")},
		},
		{"a different image with a longer name is ignored", map[string]string{
			".githooks/pre-commit": "docker run ghcr.io/rvenutolo/repo-toolbelt-other:v1 run\n" + hook,
		}, nil},
		{"differing references name both, one finding per differing reference", map[string]string{
			".githooks/pre-commit":    "docker run " + imgA + "\n",
			".githooks/pre-push":      "docker run " + imgB + "\n",
			".github/workflows/a.yml": "image: " + imgB + "\nimage: " + imgA + "\n",
		}, []lintkit.Finding{
			finding(".githooks/pre-push", 1, "image reference "+imgB+" differs from "+imgA),
			finding(".github/workflows/a.yml", 1, "image reference "+imgB+" differs from "+imgA),
		}},
		{"uses without a version comment", map[string]string{
			".github/workflows/lint.yml": "    uses: rvenutolo/repo-toolbelt/.github/workflows/lint.yml@" + sha + "\n",
			".githooks/pre-commit":       hook,
		}, []lintkit.Finding{finding(".github/workflows/lint.yml", 1, "uses: has no # vX.Y.Z version comment")}},
		{"uses with a malformed comment", map[string]string{
			".github/workflows/lint.yml": "    uses: rvenutolo/repo-toolbelt/.github/workflows/lint.yml@" + sha + " # latest\n",
			".githooks/pre-commit":       hook,
		}, []lintkit.Finding{finding(".github/workflows/lint.yml", 1, "uses: has no # vX.Y.Z version comment")}},
		{"uses version differs from the image tag", map[string]string{
			".github/workflows/lint.yml": "    uses: rvenutolo/repo-toolbelt/.github/workflows/lint.yml@" + sha + " # v1.2.4\n",
			".githooks/pre-commit":       hook,
		}, []lintkit.Finding{finding(".github/workflows/lint.yml", 1, "uses: version v1.2.4 differs from image tag v1.2.3")}},
		{"uses by tag", map[string]string{
			".github/workflows/lint.yml": "    uses: rvenutolo/repo-toolbelt/.github/workflows/lint.yml@v1.2.3\n",
			".githooks/pre-commit":       hook,
		}, []lintkit.Finding{finding(".github/workflows/lint.yml", 1, "uses: not pinned by a full commit SHA")}},
		{"uses by branch", map[string]string{
			".github/workflows/lint.yml": "    uses: rvenutolo/repo-toolbelt/.github/workflows/lint.yml@main # v1.2.3\n",
			".githooks/pre-commit":       hook,
		}, []lintkit.Finding{finding(".github/workflows/lint.yml", 1, "uses: not pinned by a full commit SHA")}},
		{"uses alone with a version comment passes", map[string]string{
			".github/workflows/lint.yml": "    uses: rvenutolo/repo-toolbelt/.github/workflows/lint.yml@" + sha + " # v1.2.3\n",
		}, nil},
		{"another repo's uses is ignored", map[string]string{
			".github/workflows/lint.yml": "    uses: actions/checkout@v4\n    uses: rvenutolo/other/.github/workflows/x.yml@main\n" + "image: " + imgA + "\n",
		}, nil},
		{
			"no references",
			map[string]string{".github/workflows/lint.yml": "name: lint\n"},
			[]lintkit.Finding{{Message: "no repo-toolbelt pin found"}},
		},
		{"no files at all", map[string]string{}, []lintkit.Finding{{Message: "no repo-toolbelt pin found"}}},
		{"a file outside the scanned directories is ignored", map[string]string{"README.md": "image: " + imgB + "\n", ".githooks/pre-commit": hook}, nil},
		{"a subdirectory is not scanned", map[string]string{".githooks/sub/x": "docker run ghcr.io/rvenutolo/repo-toolbelt run\n", ".githooks/pre-commit": hook}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, run(t, tt.files), cmp.AllowUnexported(lintkit.Finding{})); diff != "" {
				t.Errorf("findings (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCheckIgnoresSymlinks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, ".githooks/pre-commit", "docker run "+imgA+"\n")
	write(t, root, "outside", "docker run ghcr.io/rvenutolo/repo-toolbelt run\n")
	if err := os.Symlink("../outside", filepath.Join(root, ".githooks", "link")); err != nil {
		t.Fatal(err)
	}
	got, err := pins.Check(t.Context(), lintkit.Context{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("findings = %v, want none", got)
	}
}

func TestCheckSetupFailures(t *testing.T) {
	t.Parallel()
	t.Run("root missing", func(t *testing.T) {
		t.Parallel()
		if _, err := pins.Check(t.Context(), lintkit.Context{Root: filepath.Join(t.TempDir(), "absent")}); err == nil {
			t.Error("want an error")
		}
	})
	t.Run("scanned path is a file", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		write(t, root, ".githooks", "not a directory\n")
		if _, err := pins.Check(t.Context(), lintkit.Context{Root: root}); err == nil {
			t.Error("want an error")
		}
	})
	t.Run("unreadable file", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		write(t, root, ".githooks/pre-commit", "x\n")
		if err := os.Chmod(filepath.Join(root, ".githooks", "pre-commit"), 0); err != nil {
			t.Fatal(err)
		}
		if os.Getuid() == 0 {
			t.Skip("root reads any file")
		}
		if _, err := pins.Check(t.Context(), lintkit.Context{Root: root}); err == nil {
			t.Error("want an error")
		}
	})
	t.Run("unlistable directory", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		write(t, root, ".githooks/pre-commit", "x\n")
		if err := os.Chmod(filepath.Join(root, ".githooks"), 0o300); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(filepath.Join(root, ".githooks"), 0o755); err != nil {
				t.Error(err)
			}
		})
		if os.Getuid() == 0 {
			t.Skip("root lists any directory")
		}
		if _, err := pins.Check(t.Context(), lintkit.Context{Root: root}); err == nil {
			t.Error("want an error")
		}
	})
}
