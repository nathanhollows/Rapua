package blocks

import (
	"html/template"

	"github.com/nathanhollows/Rapua/v4/helpers"
)

func stringToMarkdown(s string) template.HTML {
	md, err := helpers.MarkdownToHTML(s, nil)
	if err != nil {
		return template.HTML(err.Error())
	}
	return template.HTML(md)
}
