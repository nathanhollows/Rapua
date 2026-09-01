package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteGameSpec_CreatesFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "game-spec.md")
	n, err := writeGameSpec(out)
	if err != nil {
		t.Fatalf("writeGameSpec() error: %v", err)
	}
	if n == 0 {
		t.Error("writeGameSpec() reported 0 bytes written")
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if len(data) != n {
		t.Errorf("file size %d != reported %d", len(data), n)
	}
}

func TestWriteGameSpec_HasFrontmatter(t *testing.T) {
	out := filepath.Join(t.TempDir(), "game-spec.md")
	if _, err := writeGameSpec(out); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(out)
	s := string(content)

	if !strings.HasPrefix(s, "---\n") {
		t.Error("output does not start with YAML frontmatter (---)")
	}
	if !strings.Contains(s, "title: \"Game Spec\"") {
		t.Error("output missing title frontmatter")
	}
}

func TestWriteGameSpec_ContainsAuthConstraints(t *testing.T) {
	out := filepath.Join(t.TempDir(), "game-spec.md")
	if _, err := writeGameSpec(out); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(out)
	s := string(content)

	checks := []string{
		"SLUG_DUPLICATE",
		"BLOCK_ID_DUPLICATE",
		"NO_START_BUTTON",
		"BAND_MIN_EXCEEDS_MAX",
		"BAND_OUT_OF_RANGE",
		"DEPENDS_CYCLE",
		"POINTS_DISABLED",
	}
	for _, code := range checks {
		if !strings.Contains(s, code) {
			t.Errorf("output missing lint code %q", code)
		}
	}
}

func TestWriteGameSpec_ContainsValidJSON(t *testing.T) {
	out := filepath.Join(t.TempDir(), "game-spec.md")
	if _, err := writeGameSpec(out); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(out)
	s := string(content)

	// Extract JSON between ```json and ```
	start := strings.Index(s, "```json\n")
	end := strings.LastIndex(s, "\n```")
	if start < 0 || end < 0 || end <= start {
		t.Fatal("output missing ```json ... ``` block")
	}
	jsonStr := s[start+len("```json\n") : end]

	var v any
	if err := json.Unmarshal([]byte(jsonStr), &v); err != nil {
		t.Errorf("embedded JSON is invalid: %v", err)
	}
}

func TestWriteGameSpec_JSONHasExpectedTopLevelKeys(t *testing.T) {
	out := filepath.Join(t.TempDir(), "game-spec.md")
	if _, err := writeGameSpec(out); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(out)
	s := string(content)

	start := strings.Index(s, "```json\n")
	end := strings.LastIndex(s, "\n```")
	jsonStr := s[start+len("```json\n") : end]

	var spec map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &spec); err != nil {
		t.Fatalf("JSON parse error: %v", err)
	}

	for _, key := range []string{"version", "blocks", "contexts", "enums", "document"} {
		if _, ok := spec[key]; !ok {
			t.Errorf("spec JSON missing top-level key %q", key)
		}
	}
}

func TestWriteGameSpec_AllRegisteredBlocksPresent(t *testing.T) {
	out := filepath.Join(t.TempDir(), "game-spec.md")
	if _, err := writeGameSpec(out); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(out)
	s := string(content)

	// Every registered block type should appear in the output
	knownTypes := []string{
		"alert", "button", "checklist", "clue", "divider",
		"game_status", "header", "image", "password", "photo", "pincode",
		"quiz", "random_clue", "rating", "sorting", "start_button",
		"team_name", "text", "toggle_text", "youtube",
	}
	for _, bt := range knownTypes {
		if !strings.Contains(s, `"`+bt+`"`) {
			t.Errorf("output missing block type %q", bt)
		}
	}
}
