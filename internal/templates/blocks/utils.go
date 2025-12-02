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
		return template.HTML(err.Error())
	}
	return template.HTML(md)
}

func icon(icon string, attrs templ.Attributes) templ.Component {
	return templ.Raw(lucide.Icon(icon, attrs))
}
