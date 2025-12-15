package helpers_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v6/helpers"
)

func TestURL(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		want     string
		site     string
	}{
		{
			name:     "No Patterns",
			patterns: []string{},
			want:     "http://example.com",
			site:     "http://example.com",
		},
		{
			name:     "Path Pattern",
			patterns: []string{"/path"},
			want:     "http://example.com/path",
			site:     "http://example.com",
		},
		{
			name:     "Query Pattern",
			patterns: []string{"/path", "key=value"},
			want:     "http://example.com/path?key=value",
			site:     "http://example.com",
		},
		{
			name:     "Fragment Pattern",
			patterns: []string{"/path", "key=value", "fragment"},
			want:     "http://example.com/path?key=value#fragment",
			site:     "http://example.com",
		},
		{
			name:     "No Site",
			patterns: []string{},
			want:     "",
			site:     "",
		},
		{
			name:     "No Site - Path Pattern",
			patterns: []string{"/path"},
			want:     "/path",
			site:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the site URL
			t.Setenv("SITE_URL", tt.site)
			got := helpers.URL(tt.patterns...)
			if got != tt.want {
				t.Errorf("URL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsLocalURL(t *testing.T) {
	tests := []struct {
		name     string
		siteURL  string
		inputURL string
		want     bool
	}{
		{
			name:     "Relative URL is local",
			siteURL:  "http://localhost:8090",
			inputURL: "/static/uploads/image.jpg",
			want:     true,
		},
		{
			name:     "Absolute URL with matching host and port",
			siteURL:  "http://localhost:8090",
			inputURL: "http://localhost:8090/static/uploads/image.jpg",
			want:     true,
		},
		{
			name:     "Absolute URL with different port is not local",
			siteURL:  "http://localhost:8090",
			inputURL: "http://localhost:3000/static/uploads/image.jpg",
			want:     false,
		},
		{
			name:     "External URL is not local",
			siteURL:  "http://localhost:8090",
			inputURL: "https://example.com/image.jpg",
			want:     false,
		},
		{
			name:     "Production URL matches",
			siteURL:  "https://rapua.com",
			inputURL: "https://rapua.com/static/uploads/image.jpg",
			want:     true,
		},
		{
			name:     "Production URL with subdomain does not match",
			siteURL:  "https://rapua.com",
			inputURL: "https://cdn.rapua.com/static/uploads/image.jpg",
			want:     false,
		},
		{
			name:     "Empty URL is not local",
			siteURL:  "http://localhost:8090",
			inputURL: "",
			want:     false,
		},
		{
			name:     "Relative URL with no SITE_URL set",
			siteURL:  "",
			inputURL: "/static/uploads/image.jpg",
			want:     true,
		},
		{
			name:     "Absolute URL with no SITE_URL set is not local",
			siteURL:  "",
			inputURL: "http://example.com/image.jpg",
			want:     false,
		},
		{
			name:     "Invalid URL is not local",
			siteURL:  "http://localhost:8090",
			inputURL: "://invalid-url",
			want:     false,
		},
		{
			name:     "URL with query params is local",
			siteURL:  "http://localhost:8090",
			inputURL: "/static/uploads/image.jpg?size=small",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SITE_URL", tt.siteURL)
			got := helpers.IsLocalURL(tt.inputURL)
			if got != tt.want {
				t.Errorf("IsLocalURL(%q) with SITE_URL=%q = %v, want %v",
					tt.inputURL, tt.siteURL, got, tt.want)
			}
		})
	}
}
