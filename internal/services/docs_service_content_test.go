package services_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v5/internal/services"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// TestDocs_RequireYAMLFrontmatter that ensures all .md files
// have proper YAML frontmatter. Files that fail this test will fail all others.
func TestDocs_RequireYAMLFrontmatter(t *testing.T) {
	dir := "../../docs"

	var missingYAMLFiles []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-Markdown files
		if info.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		// Get relative path for reporting
		relativePath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		// Read file content
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("failed to read file %s: %v", relativePath, err)
			return nil
		}

		content := string(data)

		// Check for YAML frontmatter (must start with --- and have at least one more ---)
		if !strings.HasPrefix(content, "---\n") {
			missingYAMLFiles = append(missingYAMLFiles, relativePath)
			return nil
		}

		// Split content to check for proper frontmatter structure
		parts := strings.SplitN(content, "---", 3)
		if len(parts) < 3 {
			missingYAMLFiles = append(missingYAMLFiles, relativePath)
			return nil
		}

		// Check that frontmatter section is not empty
		frontmatter := strings.TrimSpace(parts[1])
		if frontmatter == "" {
			missingYAMLFiles = append(missingYAMLFiles, relativePath)
			return nil
		}

		return nil
	})

	if err != nil {
		t.Fatalf("failed to walk docs directory: %v", err)
	}

	if len(missingYAMLFiles) > 0 {
		t.Fatalf(
			"%d files are missing proper YAML frontmatter. Docs service cannot be initialized.\nFiles missing YAML frontmatter:\n  - %s\n\nAll .md files must have YAML frontmatter starting with '---' and containing at least title and order fields.",
			len(missingYAMLFiles),
			strings.Join(missingYAMLFiles, "\n  - "),
		)
	}
}

// TestDocs_YAMLStructureValid validates that YAML frontmatter contains required fields.
func TestDocs_YAMLStructureValid(t *testing.T) {
	dir := "../../docs"
	docsService, err := services.NewDocsService(dir)
	if err != nil {
		t.Fatalf("failed to create DocsService (this may be due to missing YAML frontmatter): %v", err)
	}

	var walkPages func(pages []*services.DocPage)
	walkPages = func(pages []*services.DocPage) {
		for _, page := range pages {
			if len(page.Children) > 0 {
				walkPages(page.Children)
			}

			// Skip non-.md files (directories, etc.)
			if !strings.HasSuffix(page.Path, ".md") {
				continue
			}

			// Validate required fields
			if page.Title == "" {
				t.Errorf("page %s is missing 'title' field in YAML frontmatter", page.Path)
			}

			// Order can be 0, so we just check it was parsed (will be 0 if not set)
			// This is acceptable as 0 is a valid order value
		}
	}
	walkPages(docsService.Pages)
}

// Markdown to AST.
func testDocs_MarkdownToAST(t *testing.T, markdown string) ast.Node {
	t.Helper()

	// Goldmark
	gm := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
	)

	// Parse markdown
	md := text.NewReader([]byte(markdown))
	var buf bytes.Buffer
	if err := gm.Convert([]byte(markdown), &buf); err != nil {
		t.Fatalf("failed to convert markdown: %v", err)
	}

	// Get AST
	node := gm.Parser().Parse(md)
	return node
}

// Make sure that all internal links are valid and point to an existing page.
func TestDocs_LinksResolve(t *testing.T) {
	dir := "../../docs"
	docsService, err := services.NewDocsService(dir)
	if err != nil {
		t.Fatalf("failed to create DocsService: %v", err)
	}

	var walkPages func(pages []*services.DocPage)
	walkPages = func(pages []*services.DocPage) {
		for _, page := range pages {
			if len(page.Children) > 0 {
				walkPages(page.Children)
			}
			nodes := testDocs_MarkdownToAST(t, page.Content)
			err := ast.Walk(nodes, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
				if !entering || n.Kind() != ast.KindLink {
					return ast.WalkContinue, nil
				}

				link := n.(*ast.Link)
				dest := (string)(link.Destination)

				// Only check internal links
				if !strings.HasPrefix(dest, "/docs/") {
					return ast.WalkContinue, nil
				}

				// Trim any anchor links
				// var anchor string
				if i := strings.Index(dest, "#"); i != -1 {
					// anchor = dest[i:]
					dest = dest[:i]
				}

				// Check if this is a redirect
				if redirectTo, ok := docsService.Redirects[dest]; ok {
					// Verify the redirect target exists
					_, err := docsService.GetPage(redirectTo)
					if err != nil {
						t.Errorf("redirect for (%s -> %s) points to non-existent page in /docs/%s",
							dest, redirectTo, page.Path)
					}
					return ast.WalkContinue, nil
				}

				// Complain if the link doesn't resolve to a doc page
				_, err := docsService.GetPage(dest)
				if err != nil {
					t.Errorf("invalid link (%s) in /docs/%s", dest, page.Path)
				}

				// TODO: Check for anchor links
				return ast.WalkContinue, nil
			})
			if err != nil {
				t.Fatalf("failed to walk AST: %v", err)
			}
		}
	}
	walkPages(docsService.Pages)
}

// Make sure the body is not empty.
func TestDocs_BodyNotEmpty(t *testing.T) {
	dir := "../../docs"
	docsService, err := services.NewDocsService(dir)
	if err != nil {
		t.Fatalf("failed to create DocsService: %v", err)
	}

	var walkPages func(pages []*services.DocPage)
	walkPages = func(pages []*services.DocPage) {
		for _, page := range pages {
			if len(page.Children) > 0 {
				walkPages(page.Children)
			}
			if !strings.HasSuffix(page.Path, ".md") {
				continue
			}
			if strings.TrimSpace(page.Content) == "" {
				t.Errorf("empty body in /docs/%s", page.Path)
			}
		}
	}
	walkPages(docsService.Pages)
}

// Make sure headers use title case (first letter capitalized).
func TestDocs_HeadersTitleCase(t *testing.T) {
	dir := "../../docs"
	docsService, err := services.NewDocsService(dir)
	if err != nil {
		t.Fatalf("failed to create DocsService: %v", err)
	}

	var walkPages func(pages []*services.DocPage)
	walkPages = func(pages []*services.DocPage) {
		for _, page := range pages {
			if len(page.Children) > 0 {
				walkPages(page.Children)
			}

			for _, heading := range page.Headings {
				// Only check if the first word starts with a capital letter
				words := strings.Split(heading.Text, " ")
				if len(words) == 0 {
					continue
				}

				firstWord := words[0]
				if len(firstWord) == 0 || !isAlpha(firstWord[0]) {
					continue
				}

				// Check if the first letter of the heading is capitalized
				if !strings.HasPrefix(firstWord, strings.ToUpper(firstWord[:1])) {
					t.Errorf("heading '%s' doesn't start with a capital letter in /docs/%s", heading.Text, page.Path)
				}
			}
		}
	}
	walkPages(docsService.Pages)
}

// Check if a character is alphabetic.
func isAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// Make sure no pages have the same order number.
func TestDocs_OrderNumbersUnique(t *testing.T) {
	dir := "../../docs"
	docsService, err := services.NewDocsService(dir)
	if err != nil {
		t.Fatalf("failed to create DocsService: %v", err)
	}

	var walkPages func(pages []*services.DocPage)
	walkPages = func(pages []*services.DocPage) {
		orderset := make(map[int]string)
		for _, page := range pages {
			if len(page.Children) > 0 {
				walkPages(page.Children)
			}
			// Index pages order denote where the folder is placed in the sidebar.
			// However, the index page itself should always be at the top.
			if strings.HasSuffix(page.Path, "index.md") {
				page.Order = -1
			}
			if orderset[page.Order] != "" {
				t.Errorf(
					"duplicate order number %d in /docs/%s and /docs/%s",
					page.Order,
					orderset[page.Order],
					page.Path,
				)
			}
			orderset[page.Order] = page.Path
		}
	}
	walkPages(docsService.Pages)
}

// Make sure no pages have the same title within the same level.
func TestDocs_TitlesUnique(t *testing.T) {
	dir := "../../docs"
	docsService, err := services.NewDocsService(dir)
	if err != nil {
		t.Fatalf("failed to create DocsService: %v", err)
	}

	var walkPages func(pages []*services.DocPage)
	walkPages = func(pages []*services.DocPage) {
		titleset := make(map[string]string)
		for _, page := range pages {
			if len(page.Children) > 0 {
				walkPages(page.Children)
			}
			if titleset[page.Title] != "" {
				t.Errorf("duplicate title %s in /docs/%s and /docs/%s", page.Title, titleset[page.Title], page.Path)
			}
			titleset[page.Title] = page.Path
		}
	}
	walkPages(docsService.Pages)
}

// Test to make sure there are no missing pages reported by the docs service.
func TestDocs_NoMissingPages(t *testing.T) {
	dir := "../../docs"
	docsService, err := services.NewDocsService(dir)
	if err != nil {
		t.Fatalf("failed to create DocsService: %v", err)
	}

	if len(docsService.MissingPages) > 0 {
		var missingPagesList []string
		for page := range docsService.MissingPages {
			missingPagesList = append(missingPagesList, page)
		}
		t.Errorf("found %d missing pages: %v", len(docsService.MissingPages), missingPagesList)
	}
}

// Test to make sure all redirects point to valid pages.
func TestDocs_RedirectsValid(t *testing.T) {
	dir := "../../docs"
	docsService, err := services.NewDocsService(dir)
	if err != nil {
		t.Fatalf("failed to create DocsService: %v", err)
	}

	for from, to := range docsService.Redirects {
		// Verify the target exists
		_, err := docsService.GetPage(to)
		if err != nil {
			// Skip errors that are themselves redirects
			redirectError := &services.RedirectError{}
			if errors.As(err, &redirectError) {
				continue
			}
			t.Errorf("redirect from %s points to non-existent page %s", from, to)
		}
	}
}

// Test to make sure redirects don't create loops.
func TestDocs_NoRedirectLoops(t *testing.T) {
	dir := "../../docs"
	docsService, err := services.NewDocsService(dir)
	if err != nil {
		t.Fatalf("failed to create DocsService: %v", err)
	}

	for from := range docsService.Redirects {
		visited := make(map[string]bool)
		current := from

		for {
			if visited[current] {
				t.Errorf("redirect loop detected starting from %s", from)
				break
			}

			visited[current] = true
			redirectTo, ok := docsService.Redirects[current]
			if !ok {
				// No redirect, we've reached the end
				break
			}

			current = redirectTo
		}
	}
}
