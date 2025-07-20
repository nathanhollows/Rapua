package admin

import (
	"encoding/json"
	"net/http"

	"github.com/nathanhollows/Rapua/v4/helpers"
	templates "github.com/nathanhollows/Rapua/v4/internal/templates/admin"
)

// PreviewMarkdown takes markdown from a form and renders it for htmx.
func (h *AdminHandler) PreviewMarkdown(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var m map[string]string
	err := decoder.Decode(&m)
	if err != nil {
		h.handleError(w, r, "markdown preview: decoding JSON", "Error converting markdown", "error", err)
		return
	}

	md, err := helpers.MarkdownToHTML(m["markdown"])
	if err != nil {
		h.handleError(w, r, "markdown preview: converting string to markdown", "Error converting markdown", "error", err)
		return
	}

	err = templates.MarkdownPreview(md).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("markdown preview: rendering template", "error", err)
	}
}
