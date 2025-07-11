package services_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nathanhollows/Rapua/v3/internal/services"
	"gopkg.in/yaml.v3"
)

// Helper function to create temporary markdown files for testing.
func createTempMarkdownFile(t *testing.T, dir, name, content string) string {
	filePath := filepath.Join(dir, name)

	// Ensure directory exists if creating a file in a subdirectory
	dirPath := filepath.Dir(filePath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("failed to create directory for temp markdown file: %v", err)
	}

	err := os.WriteFile(filePath, []byte(content), 0600)
	if err != nil {
		t.Fatalf("failed to create temp markdown file: %v", err)
	}
	return filePath
}

func TestNewDocsService(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "docs_service_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test markdown files
	createTempMarkdownFile(t, tempDir, "index.md", "---\ntitle: Home\norder: 1\n---\n# Home Page\nWelcome to the documentation.")
	createTempMarkdownFile(t, tempDir, "getting-started.md", "---\ntitle: Getting Started\norder: 2\n---\n# Getting Started\nHow to get started.")

	docsService, err := services.NewDocsService(tempDir)
	if err != nil {
		t.Fatalf("failed to create DocsService: %v", err)
	}

	// Verify the title of the root page (index.md)
	if docsService.Pages[0].Title != "Home" {
		t.Errorf("expected title 'Home', got '%s'", docsService.Pages[0].Title)
	}
}

func TestDocsService_GetPage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "docs_service_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test markdown files
	createTempMarkdownFile(t, tempDir, "index.md", "---\ntitle: Home\norder: 1\n---\n# Home Page\nWelcome to the documentation.")
	createTempMarkdownFile(t, tempDir, "getting-started.md", "---\ntitle: Getting Started\norder: 2\n---\n# Getting Started\nHow to get started.")
	createTempMarkdownFile(t, tempDir, "setup/index.md", "---\ntitle: Setup\norder: 1\n---\n# Setup Page\nInstructions for setup.")

	docsService, err := services.NewDocsService(tempDir)
	if err != nil {
		t.Fatalf("failed to create DocsService: %v", err)
	}

	// Test retrieving the root page
	page, err := docsService.GetPage("/docs/")
	if err != nil {
		t.Fatalf("failed to get root page: %v", err)
	}
	if page.Title != "Home" {
		t.Errorf("expected title 'Home', got '%s'", page.Title)
	}

	// Test retrieving a specific page
	page, err = docsService.GetPage("/docs/getting-started")
	if err != nil {
		t.Fatalf("failed to get 'getting-started' page: %v", err)
	}
	if page.Title != "Getting Started" {
		t.Errorf("expected title 'Getting Started', got '%s'", page.Title)
	}

	// Test retrieving a nested page
	page, err = docsService.GetPage("/docs/setup/")
	if err != nil {
		t.Fatalf("failed to get 'setup' page: %v", err)
	}
	if page.Title != "Setup" {
		t.Errorf("expected title 'Setup', got '%s'", page.Title)
	}
}

func TestDocsService_BuildHierarchy(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "docs_service_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test markdown files
	createTempMarkdownFile(t, tempDir, "index.md", "---\ntitle: Home\norder: 1\n---\n# Home Page\nWelcome to the documentation.")
	createTempMarkdownFile(t, tempDir, "setup/index.md", "---\ntitle: Setup\norder: 1\n---\n# Setup\nSetup instructions.")
	createTempMarkdownFile(t, tempDir, "setup/details.md", "---\ntitle: Details\norder: 2\n---\n# Details\nDetailed setup information.")

	docsService, err := services.NewDocsService(tempDir)
	if err != nil {
		t.Fatalf("failed to create DocsService: %v", err)
	}

	if len(docsService.Pages) != 2 {
		t.Errorf("expected 2 root pages, got %d", len(docsService.Pages))
	}

	setupPage := docsService.Pages[1]
	if len(setupPage.Children) != 2 {
		t.Fatalf("expected 2 children in setup page, got %d", len(setupPage.Children))
	}

	// Find and verify the Details page among the children
	var detailsPage *services.DocPage
	for _, child := range setupPage.Children {
		if child.Title == "Details" {
			detailsPage = child
			break
		}
	}

	if detailsPage == nil {
		t.Fatalf("expected to find a child page with title 'Details'")
	}
}

func TestTrackPages(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "docs_service_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test markdown files
	createTempMarkdownFile(t, tempDir, "index.md", "---\ntitle: Home\norder: 1\n---\n# Home Page\nWelcome to the documentation.")
	createTempMarkdownFile(t, tempDir, "getting-started.md", "---\ntitle: Getting Started\norder: 2\n---\n# Getting Started\nHow to get started.")

	// Create a pre-existing known_pages.yaml with additional pages
	knownPages := []string{
		"/docs/index",
		"/docs/getting-started",
		"/docs/old-page", // This will be treated as missing
	}
	knownPagesData, err := yaml.Marshal(knownPages)
	if err != nil {
		t.Fatalf("failed to marshal known pages: %v", err)
	}
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, ".known_pages.yaml"), knownPagesData, 0644); err != nil {
		t.Fatalf("failed to write known pages file: %v", err)
	}

	docsService, err := services.NewDocsService(tempDir)
	if err != nil {
		t.Fatalf("failed to create DocsService: %v", err)
	}

	// Verify KnownPages contains all pages
	if !docsService.KnownPages["/docs/index"] {
		t.Error("expected known page '/docs/index' to be present")
	}
	if !docsService.KnownPages["/docs/getting-started"] {
		t.Error("expected known page '/docs/getting-started' to be present")
	}
	if !docsService.KnownPages["/docs/old-page"] {
		t.Error("expected known page '/docs/old-page' to be present")
	}

	// Verify MissingPages contains old-page
	if !docsService.MissingPages["/docs/old-page"] {
		t.Error("expected missing page '/docs/old-page' to be present")
	}

	// Verify missing_pages.yaml was created
	missingPagesData, err := os.ReadFile(filepath.Join(tempDir, "missing_pages.yaml"))
	if err != nil {
		t.Fatalf("failed to read missing pages file: %v", err)
	}
	var missingPages []string
	if err := yaml.Unmarshal(missingPagesData, &missingPages); err != nil {
		t.Fatalf("failed to unmarshal missing pages: %v", err)
	}
	if len(missingPages) != 1 || missingPages[0] != "/docs/old-page" {
		t.Errorf("expected missing pages to contain only '/docs/old-page', got %v", missingPages)
	}
}

func TestRedirects(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "docs_service_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test markdown files
	createTempMarkdownFile(t, tempDir, "index.md", "---\ntitle: Home\norder: 1\n---\n# Home Page\nWelcome to the documentation.")
	createTempMarkdownFile(t, tempDir, "new-page.md", "---\ntitle: New Page\norder: 2\n---\n# New Page\nThis is a new page.")

	// Create a redirect_pages.yaml file
	redirects := []services.RedirectEntry{
		{From: "/docs/old-page", To: "/docs/new-page"},
	}
	redirectData, err := yaml.Marshal(redirects)
	if err != nil {
		t.Fatalf("failed to marshal redirects: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "redirect_pages.yaml"), redirectData, 0644); err != nil {
		t.Fatalf("failed to write redirects file: %v", err)
	}

	docsService, err := services.NewDocsService(tempDir)
	if err != nil {
		t.Fatalf("failed to create DocsService: %v", err)
	}

	// Verify redirects were loaded
	if docsService.Redirects["/docs/old-page"] != "/docs/new-page" {
		t.Errorf("expected redirect for '/docs/old-page' to be '/docs/new-page', got '%s'",
			docsService.Redirects["/docs/old-page"])
	}

	// Test GetPage with redirect
	_, err = docsService.GetPage("/docs/old-page")
	if err == nil {
		t.Fatal("expected GetPage to return an error for redirect")
	}

	// Verify it's a RedirectError
	redirectErr, ok := err.(*services.RedirectError)
	if !ok {
		t.Fatalf("expected RedirectError, got %T: %v", err, err)
	}
	if redirectErr.RedirectTo != "/docs/new-page" {
		t.Errorf("expected redirect to '/docs/new-page', got '%s'", redirectErr.RedirectTo)
	}
}

func TestExtractHeadings(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "docs_service_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a markdown file with multiple headings
	content := `---
title: Test Headings
order: 1
---
# Main Heading
Some content

## Second Level
More content

### Third Level
Even more content

## Another Second Level
Final content`

	createTempMarkdownFile(t, tempDir, "headings.md", content)

	docsService, err := services.NewDocsService(tempDir)
	if err != nil {
		t.Fatalf("failed to create DocsService: %v", err)
	}

	// Get the page
	page, err := docsService.GetPage("/docs/headings")
	if err != nil {
		t.Fatalf("failed to get page: %v", err)
	}

	// Verify headings were extracted correctly
	if len(page.Headings) != 4 {
		t.Fatalf("expected 4 headings, got %d", len(page.Headings))
	}

	expectedHeadings := []struct {
		Level int
		Text  string
		ID    string
	}{
		{1, "Main Heading", "main-heading"},
		{2, "Second Level", "second-level"},
		{3, "Third Level", "third-level"},
		{2, "Another Second Level", "another-second-level"},
	}

	for i, expected := range expectedHeadings {
		if page.Headings[i].Level != expected.Level {
			t.Errorf("heading %d: expected level %d, got %d", i, expected.Level, page.Headings[i].Level)
		}
		if page.Headings[i].Text != expected.Text {
			t.Errorf("heading %d: expected text '%s', got '%s'", i, expected.Text, page.Headings[i].Text)
		}
		if page.Headings[i].ID != expected.ID {
			t.Errorf("heading %d: expected ID '%s', got '%s'", i, expected.ID, page.Headings[i].ID)
		}
	}
}
