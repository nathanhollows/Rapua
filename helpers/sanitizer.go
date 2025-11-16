package helpers

import (
	"regexp"
	"sync"

	"github.com/microcosm-cc/bluemonday"
)

//nolint:gochecknoglobals // Lazily initialized HTML sanitization policy
var (
	policyOnce sync.Once
	policy     *bluemonday.Policy
)

func getPolicy() *bluemonday.Policy {
	policyOnce.Do(func() {
		policy = bluemonday.
			UGCPolicy().
			AddTargetBlankToFullyQualifiedLinks(true).
			// Allow iframe with any class attribute
			AllowAttrs("class").OnElements("iframe").
			AllowAttrs("src", "width", "height", "allow", "allowfullscreen", "frameborder").
			OnElements("iframe").
			// Allow input with type "checkbox", remove disabled attribute
			AllowAttrs("type").Matching(regexp.MustCompile(`\bcheckbox\b`)).OnElements("input").
			AllowURLSchemes("http", "https", "mailto", "tel", "sms")
	})
	return policy
}

func SanitizeHTML(input []byte) []byte {
	return getPolicy().SanitizeBytes(input)
}
