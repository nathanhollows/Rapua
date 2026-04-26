package render

import (
	"bytes"
	"html/template"
	"log/slog"
	"regexp"
	"sync"

	"github.com/microcosm-cc/bluemonday"
	enclave "github.com/quail-ink/goldmark-enclave"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

//nolint:gochecknoglobals // Lazily initialized HTML sanitization policy
var (
	policyOnce sync.Once
	policy     *bluemonday.Policy
)

func getPolicy() *bluemonday.Policy {
	policyOnce.Do(func() {
		policy = bluemonday.
			UGCPolicy().
			AddTargetBlankToFullyQualifiedLinks(true).
			// Allow iframe with any class attribute
			AllowAttrs("class").OnElements("iframe").
			AllowAttrs("src", "width", "height", "allow", "allowfullscreen", "frameborder").
			OnElements("iframe").
			// Allow input with type "checkbox", remove disabled attribute
			AllowAttrs("type").Matching(regexp.MustCompile(`\bcheckbox\b`)).OnElements("input").
			// Allow table elements (goldmark Table extension output)
			AllowElements("table", "thead", "tbody", "tfoot", "tr").
			AllowAttrs("align").OnElements("th", "td").
			AllowElements("th", "td").
			AllowURLSchemes("http", "https", "mailto", "tel", "sms")
	})
	return policy
}

// SanitizeHTML sanitizes HTML using the shared bluemonday policy.
func SanitizeHTML(input []byte) []byte {
	return getPolicy().SanitizeBytes(input)
}

// MarkdownToHTML converts markdown to sanitized HTML.
// Uses a restricted goldmark configuration suitable for block content.
func MarkdownToHTML(s string, logger *slog.Logger) (template.HTML, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.Strikethrough,
			extension.Linkify,
			extension.Typographer,
			extension.Table,
			enclave.New(
				&enclave.Config{},
			),
		),
		goldmark.WithParserOptions(),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
		),
	)

	var buf bytes.Buffer
	if err := md.Convert([]byte(s), &buf); err != nil {
		if logger != nil {
			logger.Error("converting markdown to HTML", "err", err)
		}
		return template.HTML("Error rendering markdown to HTML"), err
	}

	// #nosec G203 - SanitizeHTML uses bluemonday to sanitize, safe from XSS
	return template.HTML(SanitizeHTML(buf.Bytes())), nil
}

// MarkdownToHTMLFull converts markdown to sanitized HTML using the full GFM
// feature set. Suitable for richer page content such as quest descriptions.
// Enables GitHub Flavored Markdown, auto-heading IDs, and passes raw HTML
// through goldmark before sanitizing with bluemonday.
func MarkdownToHTMLFull(s string) (template.HTML, error) {
	md := goldmark.New(
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithExtensions(
			extension.GFM,
			extension.Strikethrough,
			extension.Linkify,
			extension.Typographer,
			enclave.New(
				&enclave.Config{},
			),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithUnsafe(),
		),
	)

	var buf bytes.Buffer
	if err := md.Convert([]byte(s), &buf); err != nil {
		//nolint:sloglint // Template helper without access to request context
		slog.Error("converting markdown to HTML", "err", err)
		return template.HTML("Error rendering markdown to HTML"), err
	}

	//nolint:gosec // HTML is sanitized with bluemonday policy
	return template.HTML(SanitizeHTML(buf.Bytes())), nil
}
