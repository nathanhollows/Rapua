package templates

import (
	"fmt"
	"os"

	"github.com/a-h/templ"
	"github.com/kaugesaar/lucide-go"
)

// getCSSVersion is read per render so a rebuilt stylesheet busts the year-long
// cache without a restart.
func getCSSVersion() string {
	stat, err := os.Stat("static/css/tailwind.css")
	if err != nil {
		return ""
	}
	return fmt.Sprintf("?v=%d", stat.ModTime().Unix())
}

func icon(icon string, attrs templ.Attributes) templ.Component {
	return templ.Raw(lucide.Icon(icon, attrs))
}
