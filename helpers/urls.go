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

// IsLocalURL checks if a given URL belongs to the current server.
// Returns true for relative URLs and URLs matching the SITE_URL host.
func IsLocalURL(urlStr string) bool {
	// Empty URLs are not local
	if urlStr == "" {
		return false
	}

	// Parse the input URL
	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Relative URLs (no host) are considered local
	if u.Host == "" {
		return true
	}

	// Get the server's URL from environment
	siteURL := os.Getenv("SITE_URL")
	if siteURL == "" {
		// If no SITE_URL is set, only relative URLs are considered local
		return u.Host == ""
	}

	// Parse the server URL
	serverURL, err := url.Parse(siteURL)
	if err != nil {
		return u.Host == ""
	}

	// Compare hosts (case-insensitive)
	return u.Host == serverURL.Host
}
