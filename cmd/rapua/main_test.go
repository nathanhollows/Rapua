package main

import (
	"os"
	"regexp"
	"strconv"
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

func TestModuleVersionMatchesChangelog(t *testing.T) {
	// Read the changelog file
	changelogPath := "../../docs/changelog.md"
	changelogContent, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatalf("failed to read changelog: %v", err)
	}

	// Parse the first version heading (## X.Y.Z)
	changelogRe := regexp.MustCompile(`(?m)^## (\d+)\.(\d+)\.(\d+)`)
	changelogMatches := changelogRe.FindStringSubmatch(string(changelogContent))
	if len(changelogMatches) < 4 {
		t.Fatal("could not find version in changelog")
	}

	changelogMajor, _ := strconv.Atoi(changelogMatches[1])
	changelogVersion := changelogMatches[1] + "." + changelogMatches[2] + "." + changelogMatches[3]

	// Read go.mod file
	goModPath := "../../go.mod"
	goModContent, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("failed to read go.mod: %v", err)
	}

	// Parse module version (e.g., github.com/nathanhollows/Rapua/v5)
	moduleRe := regexp.MustCompile(`(?m)^module .+/v(\d+)`)
	moduleMatches := moduleRe.FindStringSubmatch(string(goModContent))

	var moduleMajor int
	if len(moduleMatches) >= 2 {
		moduleMajor, _ = strconv.Atoi(moduleMatches[1])
	} else {
		// No /vX suffix means v0 or v1
		moduleMajor = 1
	}

	// For Go modules, major version >= 2 must match the /vX suffix
	if changelogMajor >= 2 && changelogMajor != moduleMajor {
		t.Errorf("module version mismatch: changelog has %s (major: %d) but go.mod has /v%d\n"+
			"Update go.mod module path to end with /v%d",
			changelogVersion, changelogMajor, moduleMajor, changelogMajor)
	}
}
