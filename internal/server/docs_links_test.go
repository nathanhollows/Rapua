package server

import (
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi"
	admin "github.com/nathanhollows/Rapua/v8/internal/handlers/admin"
	players "github.com/nathanhollows/Rapua/v8/internal/handlers/players"
	"github.com/nathanhollows/Rapua/v8/internal/handlers/public"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// TestDocs_AppLinksResolve checks docs links into the app against the router.
// services.TestDocs_LinksResolve skips anything outside /docs/, so links to
// retired routes went unnoticed.
func TestDocs_AppLinksResolve(t *testing.T) {
	router := testRouter()

	docsService, err := services.NewDocsService("../../docs")
	if err != nil {
		t.Fatalf("creating DocsService: %v", err)
	}

	type badLink struct{ page, dest string }
	var bad []badLink

	forEachDocLink(t, docsService.Pages, func(page *services.DocPage, dest string) {
		target, ok := appLinkTarget(dest)
		if !ok {
			return
		}
		if !routeExists(router, target) {
			bad = append(bad, badLink{page: page.Path, dest: dest})
		}
	})

	sort.Slice(bad, func(i, j int) bool {
		if bad[i].page != bad[j].page {
			return bad[i].page < bad[j].page
		}
		return bad[i].dest < bad[j].dest
	})
	for _, b := range bad {
		t.Errorf("docs/%s links to %q, which no route serves", b.page, b.dest)
	}
}

func appLinkTarget(dest string) (string, bool) {
	if dest == "" || !strings.HasPrefix(dest, "/") {
		return "", false
	}
	if strings.HasPrefix(dest, "/docs/") || dest == "/docs" {
		return "", false // services.TestDocs_LinksResolve covers these
	}
	if i := strings.IndexAny(dest, "#?"); i != -1 {
		dest = dest[:i]
	}
	if dest == "" {
		return "", false
	}
	return dest, true
}

// Matching via the router keeps path params, wildcards and mounts in step with
// the real thing.
func routeExists(router *chi.Mux, path string) bool {
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		if router.Match(chi.NewRouteContext(), method, path) {
			return true
		}
		// Docs link /admin/activity; the route is registered /admin/activity/.
		alt := strings.TrimSuffix(path, "/")
		if alt == path {
			alt = path + "/"
		}
		if alt != "" && router.Match(chi.NewRouteContext(), method, alt) {
			return true
		}
	}
	return false
}

func forEachDocLink(t *testing.T, pages []*services.DocPage, fn func(*services.DocPage, string)) {
	t.Helper()
	for _, page := range pages {
		if len(page.Children) > 0 {
			forEachDocLink(t, page.Children, fn)
		}
		root := docMarkdownToAST(t, page.Content)
		err := ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering || n.Kind() != ast.KindLink {
				return ast.WalkContinue, nil
			}
			fn(page, string(n.(*ast.Link).Destination))
			return ast.WalkContinue, nil
		})
		if err != nil {
			t.Fatalf("walking markdown for %s: %v", page.Path, err)
		}
	}
}

func docMarkdownToAST(t *testing.T, markdown string) ast.Node {
	t.Helper()
	gm := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)
	return gm.Parser().Parse(text.NewReader([]byte(markdown)))
}

// Zero-value handlers are safe: registering a route only stores the handler,
// and nothing here serves a request.
func testRouter() *chi.Mux {
	logger := slog.New(slog.DiscardHandler)
	return setupRouter(logger, &public.Handler{}, &players.PlayerHandler{}, &admin.Handler{})
}
