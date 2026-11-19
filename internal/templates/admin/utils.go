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
	stat, err := os.Stat("static/css/admin.css")
	if err != nil {
		return ""
	}
	return fmt.Sprintf("?v=%d", stat.ModTime().Unix())
}

func filter[T any](items []T, fn func(T) bool) []T {
	var filtered []T
	for _, item := range items {
		if fn(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func icon(icon string, attrs templ.Attributes) templ.Component {
	return templ.Raw(lucide.Icon(icon, attrs))
}
