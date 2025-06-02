package helpers

import (
	"net/url"
	"os"
)

// URL constructs a URL specific to the application.
func URL(patterns ...string) string {
	u := &url.URL{}
	if site, ok := os.LookupEnv("SITE_URL"); ok {
		u, _ = url.Parse(site)
	} else {
		u.Path = "/"
	}
	if len(patterns) > 0 {
		u.Path += patterns[0]
	}
	if len(patterns) > 1 {
		u.RawQuery = patterns[1]
	}
	if len(patterns) > 2 {
		u.Fragment = patterns[2]
	}
	return u.String()
}
