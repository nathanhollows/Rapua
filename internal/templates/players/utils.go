package templates

import (
	"fmt"
	"os"
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
