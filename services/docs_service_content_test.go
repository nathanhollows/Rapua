package services_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v3/services"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

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
	dir := "../docs"
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
				if n.Kind() == ast.KindLink {
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

					// Complain if the link doesn't resolve to a doc page
					_, err := docsService.GetPage(dest)
					if err != nil {
						t.Errorf("invalid link (%s) in /docs/%s", dest, page.Path)
					}

					// TODO: Check for anchor links
				}
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
	dir := "../docs"
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

// Make sure headers use title case.
func TestDocs_HeadersTitleCase(t *testing.T) {
}

// Make sure no pages have the same order number.
func TestDocs_OrderNumbersUnique(t *testing.T) {
	dir := "../docs"
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
				t.Errorf("duplicate order number %d in /docs/%s and /docs/%s", page.Order, orderset[page.Order], page.Path)
			}
			orderset[page.Order] = page.Path
		}
	}
	walkPages(docsService.Pages)
}

// Make sure no pages have the same title within the same level.
func TestDocs_TitlesUnique(t *testing.T) {
	dir := "../docs"
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
