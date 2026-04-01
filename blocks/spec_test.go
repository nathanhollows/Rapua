package blocks_test

import (
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nathanhollows/Rapua/v7/blocks"
)

// updateGolden regenerates docs/developer/block-spec.md when passed to go test:
//
//	go test ./blocks/... -run TestBlockSpecGolden -update
var updateGolden = flag.Bool("update", false, "regenerate the block-spec.md golden file")

// goldenPath returns the absolute path to docs/developer/block-spec.md,
// resolved relative to this test file's location so it works regardless of
// the working directory the test runner uses.
func goldenPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// blocks/spec_test.go → repo root → docs/developer/block-spec.md
	root := filepath.Join(filepath.Dir(file), "..")
	return filepath.Join(root, "docs", "developer", "block-spec.md")
}

// TestBlockSpecGolden verifies that docs/developer/block-spec.md matches
// the current output of GenerateSpec().
//
// To regenerate the golden file after changing block specs:
//
//	go test ./blocks/... -run TestBlockSpecGolden -update
func TestBlockSpecGolden(t *testing.T) {
	got := blocks.GenerateSpec()
	path := goldenPath(t)

	if *updateGolden {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		t.Logf("updated %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file %s: %v\n\nRun with -update to generate it.", path, err)
	}

	if got != string(want) {
		t.Errorf(
			"docs/developer/block-spec.md is out of date.\n"+
				"Run: go test ./blocks/... -run TestBlockSpecGolden -update\n\n"+
				"First diff at byte offset: %d",
			firstDiffOffset(got, string(want)),
		)
	}
}

func firstDiffOffset(a, b string) int {
	max := len(a)
	if len(b) < max {
		max = len(b)
	}
	for i := 0; i < max; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return max
}
