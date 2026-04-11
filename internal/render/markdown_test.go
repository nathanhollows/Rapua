package render_test

import (
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v7/internal/render"
)

func TestSanitizeHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "strips script tags",
			input: "<script>alert('xss')</script>",
			want:  "",
		},
		{
			name:  "strips inline event handlers",
			input: `Hello <STYLE>.XSS{background-image:url('javascript:alert("XSS")');}</STYLE><A CLASS=XSS></A>World`,
			want:  "Hello World",
		},
		{
			name:  "allows basic formatting",
			input: "<p><strong>bold</strong> and <em>italic</em></p>",
			want:  "<p><strong>bold</strong> and <em>italic</em></p>",
		},
		{
			name:  "allows table elements",
			input: "<table><thead><tr><th>A</th></tr></thead><tbody><tr><td>1</td></tr></tbody></table>",
			want:  "<table><thead><tr><th>A</th></tr></thead><tbody><tr><td>1</td></tr></tbody></table>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(render.SanitizeHTML([]byte(tt.input)))
			if got != tt.want {
				t.Errorf("SanitizeHTML() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMarkdownToHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
		absent   []string
	}{
		{
			name:     "bold and italic",
			input:    "**bold** and _italic_",
			contains: []string{"<strong>bold</strong>", "<em>italic</em>"},
		},
		{
			name:     "strikethrough",
			input:    "~~deleted~~",
			contains: []string{"<del>deleted</del>"},
		},
		{
			name:  "table",
			input: "| Name | Age |\n|------|-----|\n| Alice | 30 |\n| Bob | 25 |",
			contains: []string{
				"<table>",
				"<th>Name</th>",
				"<td>Alice</td>",
				"<td>25</td>",
				"</table>",
			},
		},
		{
			name:  "table with alignment",
			input: "| Left | Center | Right |\n|:-----|:------:|------:|\n| a | b | c |",
			contains: []string{
				"<table>",
				"<th",
				"</table>",
			},
		},
		{
			name:     "link",
			input:    "[Rapua](https://rapua.nz)",
			contains: []string{`href="https://rapua.nz"`, "Rapua"},
		},
		{
			name:   "strips script tags in markdown",
			input:  "Hello <script>alert(1)</script> world",
			absent: []string{"<script>"},
		},
		{
			name:   "no raw HTML passthrough",
			input:  "<div class=\"custom\">content</div>",
			absent: []string{"<div"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := render.MarkdownToHTML(tt.input, nil)
			if err != nil {
				t.Fatalf("MarkdownToHTML() error = %v", err)
			}
			html := string(got)
			for _, want := range tt.contains {
				if !strings.Contains(html, want) {
					t.Errorf("MarkdownToHTML() output missing %q\ngot: %s", want, html)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(html, absent) {
					t.Errorf("MarkdownToHTML() output should not contain %q\ngot: %s", absent, html)
				}
			}
		})
	}
}

func TestMarkdownToHTMLFull(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
		absent   []string
	}{
		{
			name:     "GFM table",
			input:    "| Col1 | Col2 |\n|------|------|\n| a    | b    |",
			contains: []string{"<table>", "<th>Col1</th>", "<td>a</td>", "</table>"},
		},
		{
			name:     "auto heading ID",
			input:    "## My Section",
			contains: []string{`id="my-section"`, "<h2"},
		},
		{
			name:     "task list",
			input:    "- [x] done\n- [ ] todo",
			contains: []string{`type="checkbox"`, "done", "todo"},
		},
		{
			name:     "strikethrough",
			input:    "~~removed~~",
			contains: []string{"<del>removed</del>"},
		},
		{
			name:   "strips script even with unsafe renderer",
			input:  "<script>alert(1)</script>",
			absent: []string{"<script>", "alert"},
		},
		{
			name:     "raw HTML passes through renderer then sanitized",
			input:    "<p class=\"custom\">text</p>",
			contains: []string{"text"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := render.MarkdownToHTMLFull(tt.input)
			if err != nil {
				t.Fatalf("MarkdownToHTMLFull() error = %v", err)
			}
			html := string(got)
			for _, want := range tt.contains {
				if !strings.Contains(html, want) {
					t.Errorf("MarkdownToHTMLFull() output missing %q\ngot: %s", want, html)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(html, absent) {
					t.Errorf("MarkdownToHTMLFull() output should not contain %q\ngot: %s", absent, html)
				}
			}
		})
	}
}
