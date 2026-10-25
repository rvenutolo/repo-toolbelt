// Package testutil holds what the test binaries of several packages share.
package testutil

import (
	"errors"
	"os"
)

// IsolateGit hides the host's global and system git config, and the global
// excludes and attributes files git reads under XDG_CONFIG_HOME whatever
// GIT_CONFIG_GLOBAL says, from every git command the process runs from now
// on: the fixture commands a test runs and the git the code under test runs.
// A test binary calls it from TestMain, before any test starts.
func IsolateGit() error {
	return errors.Join(
		os.Setenv("GIT_CONFIG_GLOBAL", os.DevNull),
		os.Setenv("GIT_CONFIG_NOSYSTEM", "1"),
		os.Setenv("XDG_CONFIG_HOME", "/nonexistent"),
	)
}
