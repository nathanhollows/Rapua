package helpers_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v5/helpers"
)

func TestSanitizeHTML(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  []byte
	}{
		{
			name: "Valid HTML",
			input: []byte(
				"Hello <STYLE>.XSS{background-image:url('javascript:alert('XSS')');}</STYLE><A CLASS=XSS></A>World",
			),
			want: []byte("Hello World"),
		},
		{
			name:  "Invalid HTML",
			input: []byte("<script>alert('Hello, World!')</script>"),
			want:  []byte(""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := helpers.SanitizeHTML(tt.input)
			if string(got) != string(tt.want) {
				t.Errorf("SanitizeHTML() = %v, want %v", string(got), string(tt.want))
			}
		})
	}
}
