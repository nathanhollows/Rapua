package templates

import (
	"fmt"
	"html/template"
	"os"
	"sync"
	"time"

	"github.com/nathanhollows/Rapua/v7/internal/render"
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
	md, err := render.MarkdownToHTMLFull(s)
	if err != nil {
		//nolint:gosec // Error message from goldmark, not user input
		return template.HTML(err.Error())
	}

	return md
}
