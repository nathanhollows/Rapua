package helpers

import (
	"bytes"
	"html/template"
	"log/slog"

	enclave "github.com/quail-ink/goldmark-enclave"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

// MarkdownToHTML converts a string to markdown.
func MarkdownToHTML(s string) (template.HTML, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.Strikethrough,
			extension.Linkify,
			extension.Typographer,
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
		slog.Error("converting markdown to HTML", "err", err)
		return template.HTML("Error rendering markdown to HTML"), err
	}

	return template.HTML(SanitizeHTML(buf.Bytes())), nil
}
