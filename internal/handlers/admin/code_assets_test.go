package admin //nolint:testpackage // splitCodeExtension is unexported

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitCodeExtension(t *testing.T) {
	cases := []struct {
		name    string
		rest    string
		code    string
		wantExt string
	}{
		{name: "plain code", rest: "ABCDE.png", code: "ABCDE", wantExt: "png"},
		{name: "svg", rest: "ABCDE.svg", code: "ABCDE", wantExt: "svg"},
		{name: "minted code with a dash", rest: "abcd-efgh.png", code: "abcd-efgh", wantExt: "png"},
		// Split on the last dot, so a code carrying one survives.
		{name: "code containing a dot", rest: "example.com.png", code: "example.com", wantExt: "png"},
		// A code with a slash arrives percent-encoded and comes back whole.
		{name: "encoded slash", rest: "a%2Fb.png", code: "a/b", wantExt: "png"},
		{name: "no extension", rest: "ABCDE", code: "ABCDE", wantExt: ""},
		{name: "nothing", rest: "", code: "", wantExt: ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, ext := splitCodeExtension(c.rest)
			assert.Equal(t, c.code, code)
			assert.Equal(t, c.wantExt, ext)
		})
	}
}

// A traversal attempt is just a code to encode now that nothing is written to
// disk, but it must still round-trip whole rather than being mangled.
func TestSplitCodeExtension_PathLikeCodes(t *testing.T) {
	code, ext := splitCodeExtension("..%2F..%2Fetc%2Fpasswd.png")
	assert.Equal(t, "../../etc/passwd", code)
	assert.Equal(t, pngExtension, ext)
}
