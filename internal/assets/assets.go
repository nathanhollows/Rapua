// Package assets gives templates a way to link static files that cannot be
// served stale.
package assets

import (
	"fmt"
	"os"
	"strings"
)

// URL stamps a static file with the time it last changed, read on each render,
// so a rebuilt or redeployed file is fetched again. A browser that kept an older
// copy under its own caching rules would otherwise run it against newer markup.
// A file that cannot be found is still linked: a missing stamp must not take the
// script or stylesheet away.
func URL(path string) string {
	stat, err := os.Stat(strings.TrimPrefix(path, "/"))
	if err != nil {
		return path
	}
	return fmt.Sprintf("%s?v=%d", path, stat.ModTime().Unix())
}
