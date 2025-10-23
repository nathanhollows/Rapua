package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestVersionMatchesChangelog(t *testing.T) {
	// Read the changelog file
	changelogPath := "../../docs/changelog.md"
	content, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatalf("failed to read changelog: %v", err)
	}

	// Parse the first version heading (## X.Y.Z)
	re := regexp.MustCompile(`(?m)^## (\d+\.\d+\.\d+)`)
	matches := re.FindStringSubmatch(string(content))
	if len(matches) < 2 {
		t.Fatal("could not find version in changelog")
	}
	changelogVersion := matches[1]

	// Compare with version constant (remove 'v' prefix if present)
	codeVersion := strings.TrimPrefix(version, "v")

	if codeVersion != changelogVersion {
		t.Errorf("version mismatch: code has %q but changelog has %q", version, changelogVersion)
	}
}
