package templates

import (
	"html/template"
	"time"

	"github.com/a-h/templ"
	lucide "github.com/kaugesaar/lucide-go"
	"github.com/nathanhollows/Rapua/v8/internal/render"
)

func icon(name string, attrs templ.Attributes) templ.Component {
	return templ.Raw(lucide.Icon(name, attrs))
}

func currYear() string {
	return time.Now().Format("2006")
}

func stringToMarkdown(s string) template.HTML {
	md, err := render.MarkdownToHTMLFull(s)
	if err != nil {
		//nolint:gosec // Error message from goldmark, not user input
		return template.HTML(err.Error())
	}

	return md
}
