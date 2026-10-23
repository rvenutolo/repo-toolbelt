package lintkit_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/rvenutolo/repo-toolbelt/internal/lintkit"
)

// openRoot returns dir opened as an os.Root, closed when the test ends.
func openRoot(t *testing.T, dir string) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return root
}

func TestReadRegular(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		file   string
		setup  func(t *testing.T, dir string)
		want   string
		wantOK bool
	}{
		{
			name: "regular file", file: "a.txt", want: "content\n", wantOK: true,
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeTestFile(t, filepath.Join(dir, "a.txt"), "content\n")
			},
		},
		{name: "missing file", file: "gone.txt", setup: func(*testing.T, string) {}},
		{
			name: "directory now a file", file: "sub/a.txt",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeTestFile(t, filepath.Join(dir, "sub"), "a file where the directory was\n")
			},
		},
		{
			name: "symlink", file: "link.txt",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeTestFile(t, filepath.Join(dir, "a.txt"), "content\n")
				if err := os.Symlink("a.txt", filepath.Join(dir, "link.txt")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "fifo", file: "pipe",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				if err := syscall.Mkfifo(filepath.Join(dir, "pipe"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "directory", file: "sub",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				if err := os.Mkdir(filepath.Join(dir, "sub"), 0o700); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			tc.setup(t, dir)
			root := openRoot(t, dir)
			got, ok, err := lintkit.ReadRegular(root, tc.file)
			if err != nil {
				t.Fatalf("ReadRegular(%q) error = %v, want nil", tc.file, err)
			}
			if ok != tc.wantOK || string(got) != tc.want {
				t.Errorf("ReadRegular(%q) = %q, %t; want %q, %t", tc.file, got, ok, tc.want, tc.wantOK)
			}
			// A limit past the end reads what ReadRegular reads.
			got, ok, err = lintkit.ReadRegularPrefix(root, tc.file, 1<<10)
			if err != nil {
				t.Fatalf("ReadRegularPrefix(%q) error = %v, want nil", tc.file, err)
			}
			if ok != tc.wantOK || string(got) != tc.want {
				t.Errorf("ReadRegularPrefix(%q) = %q, %t; want %q, %t", tc.file, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestReadRegular_unreadable(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads files whatever their mode")
	}
	tests := []struct {
		name       string
		file       string
		unreadable string // made mode 0o000, relative to the root
		want       string // the error's prefix
	}{
		{name: "file with no permissions", file: "a.txt", unreadable: "a.txt", want: "read a.txt: "},
		{name: "file in a directory with no permissions", file: "sub/a.txt", unreadable: "sub", want: "stat sub/a.txt: "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			writeTestFile(t, filepath.Join(dir, tc.file), "content\n")
			target := filepath.Join(dir, tc.unreadable)
			if err := os.Chmod(target, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(target, 0o700) }) //nolint:errcheck // lets TempDir clean up; a failure only leaves the dir
			root := openRoot(t, dir)
			readers := map[string]func() ([]byte, bool, error){
				"ReadRegular":       func() ([]byte, bool, error) { return lintkit.ReadRegular(root, tc.file) },
				"ReadRegularPrefix": func() ([]byte, bool, error) { return lintkit.ReadRegularPrefix(root, tc.file, 4) },
			}
			for name, read := range readers {
				got, ok, err := read()
				if err == nil {
					t.Fatalf("%s(%q) = %q, %t, nil; want an error", name, tc.file, got, ok)
				}
				if !errors.Is(err, fs.ErrPermission) || !strings.HasPrefix(err.Error(), tc.want) {
					t.Errorf("%s(%q) error = %v; want a permission error starting %q", name, tc.file, err, tc.want)
				}
				if got != nil || ok {
					t.Errorf("%s(%q) = %q, %t with its error; want nil, false", name, tc.file, got, ok)
				}
			}
		})
	}
}

func TestReadRegularPrefix_stopsAtTheLimit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "a.txt"), "content\n")
	got, ok, err := lintkit.ReadRegularPrefix(openRoot(t, dir), "a.txt", 4)
	if err != nil || !ok || string(got) != "cont" {
		t.Errorf("ReadRegularPrefix = %q, %t, %v; want %q, true, nil", got, ok, err, "cont")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
