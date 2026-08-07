package admin

import (
	"encoding/json"
	"net/http"

	"github.com/nathanhollows/Rapua/v8/internal/render"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
)

// PreviewMarkdown takes markdown from a form and renders it for htmx.
func (h *Handler) PreviewMarkdown(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var m map[string]string
	err := decoder.Decode(&m)
	if err != nil {
		h.handleError(w, r, "markdown preview: decoding JSON", "Error converting markdown", "error", err)
		return
	}

	md, err := render.MarkdownToHTML(m["markdown"], h.logger)
	if err != nil {
		h.handleError(
			w,
			r,
			"markdown preview: converting string to markdown",
			"Error converting markdown",
			"error",
			err,
		)
		return
	}

	err = templates.MarkdownPreview(md).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "markdown preview: rendering template", "error", err)
	}
}
