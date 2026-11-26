package templates

import (
	"github.com/a-h/templ"
	"github.com/kaugesaar/lucide-go"
)

func icon(icon string, attrs templ.Attributes) templ.Component {
	return templ.Raw(lucide.Icon(icon, attrs))
}
