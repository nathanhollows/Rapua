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
