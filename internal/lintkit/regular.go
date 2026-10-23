package lintkit

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

// ReadRegular reads the tracked file at the slash-separated path file under
// root. It reports false, with no error, for a file missing from the working
// tree (its directory may now be a file) and for anything but a regular file:
// git records a symlink's target as its content, and the file it points to is
// read under its own name if it is tracked. Any other failure to stat or read
// the file is an error naming it.
func ReadRegular(root *os.Root, file string) ([]byte, bool, error) {
	info, err := root.Lstat(file)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("stat %s: %w", file, err)
	}
	if !info.Mode().IsRegular() {
		return nil, false, nil
	}
	content, err := root.ReadFile(file)
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", file, err)
	}
	return content, true, nil
}
