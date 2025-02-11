package blocks

import (
	"html/template"

	"github.com/nathanhollows/Rapua/v3/helpers"
)

func stringToMarkdown(s string) template.HTML {
	md, err := helpers.MarkdownToHTML(s)
	if err != nil {
		return template.HTML(err.Error())
	}
	return template.HTML(md)
}
