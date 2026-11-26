package blocks

import (
	"html/template"

	"github.com/a-h/templ"
	"github.com/kaugesaar/lucide-go"
	"github.com/nathanhollows/Rapua/v8/internal/render"
)

func stringToMarkdown(s string) template.HTML {
	md, err := render.MarkdownToHTML(s, nil)
	if err != nil {
		//nolint:gosec // Error message from goldmark, not user input
		return template.HTML(err.Error())
	}

	return md
}

func icon(icon string, attrs templ.Attributes) templ.Component {
	return templ.Raw(lucide.Icon(icon, attrs))
}
