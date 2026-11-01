package templates

import (
	"fmt"
	"html/template"
	"os"
	"time"

	"github.com/a-h/templ"
	lucide "github.com/kaugesaar/lucide-go"
	"github.com/nathanhollows/Rapua/v8/internal/render"
)

func icon(name string, attrs templ.Attributes) templ.Component {
	return templ.Raw(lucide.Icon(name, attrs))
}

// getCSSVersion is read per render so a rebuilt stylesheet busts the year-long
// cache without a restart.
func getCSSVersion() string {
	stat, err := os.Stat("static/css/tailwind.css")
	if err != nil {
		return ""
	}
	return fmt.Sprintf("?v=%d", stat.ModTime().Unix())
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
