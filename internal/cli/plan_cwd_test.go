package cli_test

import (
	"os"
	"testing"
)

// plan runs with no checkout mounted, so it must not read the working
// directory. A working directory that no longer exists makes any read of it
// fail.
func TestRun_planNeverReadsWorkingDirectory(t *testing.T) {
	// Not parallel: the working directory is process-wide.
	gone := t.TempDir()
	t.Chdir(gone)
	if err := os.Remove(gone); err != nil {
		t.Fatalf("remove working directory: %v", err)
	}
	got := run(t, nil, "plan", "a", "b")
	if got.code != 0 || got.stdout != "{\"groups\":[[\"a\"],[\"b\"]]}\n" {
		t.Errorf("got %+v, want exit 0 and the plan", got)
	}
}
