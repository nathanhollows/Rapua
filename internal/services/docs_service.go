package services

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// DocPage represents a single documentation page.
type DocPage struct {
	Title    string
	Order    int
	Path     string
	Content  string
	URL      string
	Headings []Heading
	Children []*DocPage
}

// Heading represents a section heading within a doc page.
type Heading struct {
	Level int
	Text  string
	ID    string
}

// RedirectEntry defines a redirection from one URL to another
type RedirectEntry struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

// DocsService handles loading and providing documentation content.
type DocsService struct {
	DocsDir      string
	Pages        []*DocPage
	KnownPages   map[string]bool
	MissingPages map[string]bool
	Redirects    map[string]string
}

// NewDocsService creates a new instance of DocsService.
func NewDocsService(docsDir string) (*DocsService, error) {
	service := &DocsService{
		DocsDir:      docsDir,
		KnownPages:   make(map[string]bool),
		MissingPages: make(map[string]bool),
		Redirects:    make(map[string]string),
	}

	// Load known pages from YAML if it exists
	if err := service.loadKnownPages(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Load existing missing pages if it exists
	if err := service.loadMissingPages(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Load redirects
	if err := service.loadRedirects(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Load docs
	if err := service.loadDocs(); err != nil {
		return nil, err
	}

	// Track pages and write update files
	if err := service.trackPages(); err != nil {
		return nil, err
	}

	return service, nil
}

// loadKnownPages loads the list of known pages from docs/.known_pages.yaml
func (ds *DocsService) loadKnownPages() error {
	knownPagesPath := filepath.Join(ds.DocsDir, ".known_pages.yaml")

	data, err := os.ReadFile(knownPagesPath)
	if err != nil {
		return err
	}

	var pages []string
	if err := yaml.Unmarshal(data, &pages); err != nil {
		return err
	}

	for _, page := range pages {
		ds.KnownPages[page] = true
	}

	return nil
}

// loadMissingPages loads the existing missing pages from docs/missing_pages.yaml
func (ds *DocsService) loadMissingPages() error {
	missingPagesPath := filepath.Join(ds.DocsDir, "missing_pages.yaml")

	data, err := os.ReadFile(missingPagesPath)
	if err != nil {
		return err
	}

	var pages []string
	if err := yaml.Unmarshal(data, &pages); err != nil {
		return err
	}

	for _, page := range pages {
		ds.MissingPages[page] = true
	}

	return nil
}

// loadRedirects loads the redirect configuration from docs/redirect_pages.yaml
func (ds *DocsService) loadRedirects() error {
	redirectPath := filepath.Join(ds.DocsDir, "redirect_pages.yaml")

	data, err := os.ReadFile(redirectPath)
	if err != nil {
		return err
	}

	var redirects []RedirectEntry
	if err := yaml.Unmarshal(data, &redirects); err != nil {
		return err
	}

	for _, redirect := range redirects {
		ds.Redirects[redirect.From] = redirect.To
	}

	return nil
}

// trackPages compares current pages with known pages and updates tracking files
func (ds *DocsService) trackPages() error {
	// Build list of current pages
	currentPages := make(map[string]bool)
	collectPageURLs(ds.Pages, currentPages)

	// Find newly missing pages (in known but not in current and not already tracked as missing)
	for page := range ds.KnownPages {
		if !currentPages[page] && !ds.MissingPages[page] {
			// Add to missing pages if it's not in redirects
			if _, exists := ds.Redirects[page]; !exists {
				ds.MissingPages[page] = true
			}
		}
	}

	// Remove from missing pages if they are now in current pages or in redirects
	for page := range ds.MissingPages {
		if currentPages[page] || ds.Redirects[page] != "" {
			delete(ds.MissingPages, page)
		}
	}

	// Write missing pages to YAML file only if there are missing pages
	missingPath := filepath.Join(ds.DocsDir, "missing_pages.yaml")

	if len(ds.MissingPages) > 0 {
		var missingPagesList []string
		for page := range ds.MissingPages {
			missingPagesList = append(missingPagesList, page)
		}
		sort.Strings(missingPagesList)

		missingData, err := yaml.Marshal(missingPagesList)
		if err != nil {
			return err
		}

		if err := os.WriteFile(missingPath, missingData, 0644); err != nil {
			return err
		}
	} else {
		// Remove the missing_pages.yaml file if it exists but there are no missing pages
		_ = os.Remove(missingPath) // Ignore error if file doesn't exist
	}

	// Update known pages file with all current pages
	// Keep existing known pages and add new ones
	for page := range currentPages {
		ds.KnownPages[page] = true
	}

	var allKnownPages []string
	for page := range ds.KnownPages {
		allKnownPages = append(allKnownPages, page)
	}
	sort.Strings(allKnownPages)

	knownData, err := yaml.Marshal(allKnownPages)
	if err != nil {
		return err
	}

	knownPath := filepath.Join(ds.DocsDir, ".known_pages.yaml")
	if err := os.WriteFile(knownPath, knownData, 0644); err != nil {
		return err
	}

	return nil
}

// collectPageURLs extracts URLs from the page tree
func collectPageURLs(pages []*DocPage, result map[string]bool) {
	for _, page := range pages {
		result[page.URL] = true
		if len(page.Children) > 0 {
			collectPageURLs(page.Children, result)
		}
	}
}

// loadDocs loads and parses all Markdown files in the DocsDir.
func (ds *DocsService) loadDocs() error {
	var pages []*DocPage

	err := filepath.Walk(ds.DocsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-Markdown files
		if info.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		// Get the relative path for URL generation
		relativePath, err := filepath.Rel(ds.DocsDir, path)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(relativePath) // Ensure consistent path separators

		// Read file content
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Split front matter and content
		parts := strings.SplitN(string(data), "---", 3)
		if len(parts) < 3 {
			return nil // Skip files without proper front matter
		}

		// Parse YAML front matter
		var meta struct {
			Title string `yaml:"title"`
			Order int    `yaml:"order"`
		}
		if err := yaml.Unmarshal([]byte(parts[1]), &meta); err != nil {
			return err
		}

		// Extract headings for ToC
		headings := extractHeadings(parts[2])

		// Create DocPage
		page := &DocPage{
			Title:    meta.Title,
			Order:    meta.Order,
			Path:     relativePath,
			URL:      "/docs/" + strings.TrimSuffix(relativePath, ".md"),
			Content:  parts[2],
			Headings: headings,
		}

		pages = append(pages, page)
		return nil
	})

	if err != nil {
		return err
	}

	// Build the page hierarchy
	ds.Pages = buildHierarchy(pages)
	return nil
}

// extractHeadings extracts headings from Markdown content.
func extractHeadings(content string) []Heading {
	var headings []Heading
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			level := len(strings.SplitN(line, " ", 2)[0])
			text := strings.TrimSpace(strings.TrimLeft(line, "# "))
			id := strings.ReplaceAll(strings.ToLower(text), " ", "-")
			reg := regexp.MustCompile(`[^A-z0-9-]`)
			id = reg.ReplaceAllString(id, "")
			headings = append(headings, Heading{
				Level: level,
				Text:  text,
				ID:    id,
			})
		}
	}
	return headings
}

// buildHierarchy organizes pages into a tree based on their paths.
func buildHierarchy(pages []*DocPage) []*DocPage {
	root := make(map[string]*DocPage)

	for _, page := range pages {
		parts := strings.Split(page.Path, "/")
		addToTree(root, parts, page, 0)
	}

	// Convert map to slice and sort
	var rootPages []*DocPage
	for _, page := range root {
		rootPages = append(rootPages, page)
	}

	sortPages(rootPages)
	return rootPages
}

func addToTree(node map[string]*DocPage, parts []string, page *DocPage, depth int) {
	if depth >= len(parts) {
		return
	}
	key := parts[depth]
	if existing, ok := node[key]; ok {
		// Existing node, proceed to next depth
		if depth == len(parts)-1 {
			// Leaf node
			existing.Title = page.Title
			existing.Order = page.Order
			existing.Content = page.Content
			existing.Headings = page.Headings
			existing.URL = page.URL
		} else {
			// Check if page is index.md for this directory
			if filepath.Base(page.Path) == "index.md" && depth == len(parts)-2 {
				// Update directory node with index.md's Title, Order, Content, etc.
				existing.Title = page.Title
				existing.Order = page.Order
				existing.Content = page.Content
				existing.Headings = page.Headings
				existing.URL = page.URL
			}
			if existing.Children == nil {
				existing.Children = []*DocPage{}
			}
			childMap := make(map[string]*DocPage)
			for _, child := range existing.Children {
				childMap[filepath.Base(child.Path)] = child
			}
			addToTree(childMap, parts, page, depth+1)
			existing.Children = mapToSlice(childMap)
			sortPages(existing.Children)
		}
	} else {
		// Create a new node
		newPage := &DocPage{
			Title: key,
			Path:  strings.Join(parts[:depth+1], "/"),
			URL:   "/docs/" + strings.Join(parts[:depth+1], "/"),
			Order: 9999, // Default order for directories without specified order
		}
		if depth == len(parts)-1 {
			// Leaf node
			newPage.Title = page.Title
			newPage.Order = page.Order
			newPage.Content = page.Content
			newPage.Headings = page.Headings
			newPage.URL = page.URL
		} else {
			// Check if page is index.md for this directory
			if filepath.Base(page.Path) == "index.md" && depth == len(parts)-2 {
				// Update directory node with index.md's Title, Order, Content, etc.
				newPage.Title = page.Title
				newPage.Order = page.Order
				newPage.Content = page.Content
				newPage.Headings = page.Headings
				newPage.URL = page.URL
			}
			childMap := make(map[string]*DocPage)
			addToTree(childMap, parts, page, depth+1)
			newPage.Children = mapToSlice(childMap)
			sortPages(newPage.Children)
		}
		node[key] = newPage
	}
}

func mapToSlice(m map[string]*DocPage) []*DocPage {
	var slice []*DocPage
	for _, v := range m {
		slice = append(slice, v)
	}
	return slice
}

func sortPages(pages []*DocPage) {
	sort.SliceStable(pages, func(i, j int) bool {
		if strings.Contains(pages[i].Path, "index.md") {
			return true
		}
		if strings.Contains(pages[j].Path, "index.md") {
			return false
		}
		return pages[i].Order < pages[j].Order
	})
	for _, page := range pages {
		if len(page.Children) > 0 {
			sortPages(page.Children)
		}
	}
}

func (ds *DocsService) GetPage(urlPath string) (*DocPage, error) {
	// Check if this is a redirect
	if redirectTo, ok := ds.Redirects[urlPath]; ok {
		// Return special error that signals a redirect should happen
		return nil, &RedirectError{RedirectTo: redirectTo}
	}

	// Normalize the path
	trimmedPath := strings.TrimPrefix(urlPath, "/docs")
	trimmedPath = strings.TrimSuffix(trimmedPath, "/")

	if trimmedPath == "" {
		trimmedPath = "/index"
	}

	parts := strings.Split(trimmedPath, "/")[1:] // Skip the empty string at index 0

	var currentPages []*DocPage = ds.Pages
	var foundPage *DocPage

	for _, part := range parts {
		found := false
		for _, page := range currentPages {
			pageBaseName := strings.TrimSuffix(filepath.Base(page.Path), ".md")
			if pageBaseName == part {
				found = true
				foundPage = page
				currentPages = page.Children
				break
			}
		}
		if !found {
			return nil, os.ErrNotExist
		}
	}
	if foundPage != nil {
		return foundPage, nil
	}
	return nil, os.ErrNotExist
}

// RedirectError is a custom error type for signaling redirects
type RedirectError struct {
	RedirectTo string
}

func (e *RedirectError) Error() string {
	return "redirect to " + e.RedirectTo
}
