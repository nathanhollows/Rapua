package game_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestNoDepsOnProjectPackages ensures the game package imports no project-internal packages.
// game/ must remain a pure vocabulary leaf with zero project imports.
func TestNoDepsOnProjectPackages(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "github.com/nathanhollows/Rapua/v7/game")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -deps failed: %v", err)
	}

	const modulePrefix = "github.com/nathanhollows/Rapua/v7/"
	forbidden := []string{
		modulePrefix + "blocks",
		modulePrefix + "models",
		modulePrefix + "internal/",
		modulePrefix + "navigation",
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == modulePrefix+"game" {
			continue
		}
		for _, f := range forbidden {
			if strings.HasPrefix(line, f) {
				t.Errorf("game/ must not import %q (found %q)", f, line)
			}
		}
	}
}
