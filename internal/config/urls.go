package config

import (
	"net/url"
	"os"
)

// SiteURL constructs an absolute URL for the application.
// Reads the base from SITE_URL env var; falls back to "/" if unset.
// patterns: [path[, rawQuery[, fragment]]]
func SiteURL(patterns ...string) string {
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
	if urlStr == "" {
		return false
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Relative URLs (no host) are considered local
	if u.Host == "" {
		return true
	}

	siteURL := os.Getenv("SITE_URL")
	if siteURL == "" {
		return u.Host == ""
	}

	serverURL, err := url.Parse(siteURL)
	if err != nil {
		return u.Host == ""
	}

	return u.Host == serverURL.Host
}
