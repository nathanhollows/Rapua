package blocks

import (
	"html/template"

	"github.com/a-h/templ"
	"github.com/kaugesaar/lucide-go"
	"github.com/nathanhollows/Rapua/v6/helpers"
)

func stringToMarkdown(s string) template.HTML {
	md, err := helpers.MarkdownToHTML(s, nil)
	if err != nil {
		//nolint:gosec // Error message from goldmark, not user input
		return template.HTML(err.Error())
	}
	//nolint:gosec // HTML is sanitized in helpers.MarkdownToHTML
	return md
}

func icon(icon string, attrs templ.Attributes) templ.Component {
	return templ.Raw(lucide.Icon(icon, attrs))
}
