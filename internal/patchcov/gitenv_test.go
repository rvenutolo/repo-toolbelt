package patchcov_test

import (
	"testing"

	"github.com/rvenutolo/repo-toolbelt/internal/testutil"
)

// TestMain hides the host's git setup from the code under test, which runs
// git with the process environment, as well as from the fixture commands.
func TestMain(m *testing.M) {
	if err := testutil.IsolateGit(); err != nil {
		panic(err)
	}
	m.Run()
}
