package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

// TestMain_versionPrintsTheVersion runs main in process; --version succeeds,
// so main returns without calling os.Exit. It swaps os.Args and os.Stdout, so
// it is not parallel.
func TestMain_versionPrintsTheVersion(t *testing.T) { //nolint:paralleltest // swaps os.Args and os.Stdout
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
	// The version is whatever the build stamped, or a fallback; never empty.
	got, ok := strings.CutPrefix(string(out), "toolbelt version ")
	if !ok || strings.TrimSpace(got) == "" || strings.Count(got, "\n") != 1 || !strings.HasSuffix(got, "\n") {
		t.Errorf("stdout = %q, want one line `toolbelt version <version>`", out)
	}
}
