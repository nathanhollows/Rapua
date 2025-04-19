package services_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v3/services"
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
		t.Errorf("expected 1 root page, got %d", len(docsService.Pages))
	}

	setupPage := docsService.Pages[1]
	if len(setupPage.Children) != 2 {
		t.Fatalf("expected 2 children in setup page, got %d", len(setupPage.Children))
	}

	// Verify child page titles
	titles := []string{setupPage.Children[0].Title, setupPage.Children[1].Title}
	sort.Strings(titles)
	if strings.Join(titles, ",") != "Details,Setup" {
		t.Errorf("expected children titles 'Details,Setup', got '%s'", strings.Join(titles, ","))
	}
}
