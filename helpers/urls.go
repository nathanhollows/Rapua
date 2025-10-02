package helpers

import (
	"net/url"
	"os"
)

// URL constructs a URL specific to the application.
func URL(patterns ...string) string {
	const (
		pathIdx     = 0
		queryIdx    = 1
		fragmentIdx = 2
	)

	u := &url.URL{}
	if site, ok := os.LookupEnv("SITE_URL"); ok {
		u, _ = url.Parse(site)
	} else {
		u.Path = "/"
	}
	if len(patterns) > pathIdx {
		u.Path += patterns[pathIdx]
	}
	if len(patterns) > queryIdx {
		u.RawQuery = patterns[queryIdx]
	}
	if len(patterns) > fragmentIdx {
		u.Fragment = patterns[fragmentIdx]
	}
	return u.String()
}
