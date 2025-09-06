package templates

import (
	"fmt"
	"html/template"
	"os"
	"sync"

	"github.com/nathanhollows/Rapua/v4/helpers"
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

func stringToMarkdown(s string) template.HTML {
	md, err := helpers.MarkdownToHTML(s)
	if err != nil {
		return template.HTML(err.Error())
	}
	return template.HTML(md)
}
