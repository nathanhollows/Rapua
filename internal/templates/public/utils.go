package templates

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/nathanhollows/Rapua/v4/helpers"
	enclave "github.com/quail-ink/goldmark-enclave"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

var cssVersion string
var cssVersionOnce sync.Once

// getCSSVersion returns the CSS version, ensuring it is only set once.
func getCSSVersion() string {
	cssVersionOnce.Do(func() {
		if stat, err := os.Stat("static/css/tailwind.css"); err == nil {
			cssVersion = fmt.Sprintf("?v=%d", stat.ModTime().Unix())
		} else {
			cssVersion = "&v=1"
		}
	})
	return cssVersion
}

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

	sanitizedMD := helpers.SanitizeHTML(buf.Bytes())

	return template.HTML(sanitizedMD), nil
}
