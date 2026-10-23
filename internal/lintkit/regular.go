package lintkit

import (
	"errors"
	"fmt"
	"io"
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
	ok, err := isRegular(root, file)
	if !ok || err != nil {
		return nil, false, err
	}
	content, err := root.ReadFile(file)
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", file, err)
	}
	return content, true, nil
}

// ReadRegularPrefix is ReadRegular reading at most the first limit bytes, for
// a caller that needs only the start of a file that may be large. A limit
// that is not positive is an error: it would read nothing, and an empty file
// would look like any other.
func ReadRegularPrefix(root *os.Root, file string, limit int64) ([]byte, bool, error) {
	if limit <= 0 {
		return nil, false, fmt.Errorf("read %s: prefix limit %d is not positive", file, limit)
	}
	ok, err := isRegular(root, file)
	if !ok || err != nil {
		return nil, false, err
	}
	var content []byte
	f, err := root.Open(file)
	if err == nil {
		defer f.Close()
		content, err = io.ReadAll(io.LimitReader(f, limit))
	}
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", file, err)
	}
	return content, true, nil
}

// isRegular reports whether file is a regular file in the working tree; see
// ReadRegular for what is not.
func isRegular(root *os.Root, file string) (bool, error) {
	info, err := root.Lstat(file)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", file, err)
	}
	return info.Mode().IsRegular(), nil
}
