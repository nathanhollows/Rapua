package templates

import (
	"bytes"
	"html/template"
	"log/slog"
	"time"

	enclave "github.com/quail-ink/goldmark-enclave"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"go.abhg.dev/goldmark/anchor"
)

func currYear() string {
	return time.Now().Format("2006")
}

func stringToMarkdown(s string) template.HTML {
	md, err := markdownToHTML(s)
	if err != nil {
		return template.HTML(err.Error())
	}
	return template.HTML(md)
}

// MarkdownToHTML converts a string to markdown.
func markdownToHTML(s string) (template.HTML, error) {
	md := goldmark.New(
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithExtensions(
			extension.GFM,
			extension.Strikethrough,
			extension.Linkify,
			extension.Typographer,
			&anchor.Extender{
				Texter:   anchor.Text("#"),
				Position: anchor.Before,
			},
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
		slog.Error("converting markdown to HTML", "err", err)
		return template.HTML("Error rendering markdown to HTML"), err
	}

	return template.HTML(buf.String()), nil
}
