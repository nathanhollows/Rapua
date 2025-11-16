package templates

import (
	"fmt"
	"os"
	"strconv"
	"sync"
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

func floatToString(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
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
