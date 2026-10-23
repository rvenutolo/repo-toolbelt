package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

// TestMain_versionPrintsTheVersion runs main in process; --version succeeds,
// so main returns without calling os.Exit. It swaps process state, so it is
// not parallel.
func TestMain_versionPrintsTheVersion(t *testing.T) { //nolint:paralleltest // swaps process state (os.Args, streams, working directory)
	version = "1.2.3-test"
	t.Cleanup(func() { version = "" })
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldArgs, oldOut := os.Args, os.Stdout
	os.Args, os.Stdout = []string{"toolbelt", "--version"}, w
	main()
	os.Args, os.Stdout = oldArgs, oldOut
	_ = w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "1.2.3-test") {
		t.Errorf("stdout = %q, want the version", out)
	}
}
