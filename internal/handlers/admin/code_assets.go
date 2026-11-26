package admin

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi"
)

const (
	pngExtension = "png"
	svgExtension = "svg"
	// A QR code tops out well before this; beyond it the encoder fails anyway.
	maxCodeLength = 2048
)

// CodeQRCode renders any code as a QR image, served at /admin/qr/{code}.{ext} so
// an <img> can embed it and an <a download> takes its filename from the path. The
// image encodes the code itself, because that is what a scan is compared against.
//
// Nothing is cached: a code is an arbitrary string rather than a stored resource,
// so writing one file per distinct request would let anyone signed in fill the
// disk. Encoding is cheap enough to redo.
func (h *Handler) CodeQRCode(w http.ResponseWriter, r *http.Request) {
	code, rawExtension := splitCodeExtension(chi.URLParam(r, "*"))

	var extension string
	switch rawExtension {
	case pngExtension:
		extension = pngExtension
	case svgExtension:
		extension = svgExtension
	default:
		http.Error(w, "Invalid extension provided", http.StatusNotFound)
		return
	}

	if code == "" {
		http.Error(w, "No code provided", http.StatusBadRequest)
		return
	}
	if len(code) > maxCodeLength {
		http.Error(w, "Code is too long to encode", http.StatusBadRequest)
		return
	}

	// No Content-Disposition: an attachment header would stop an <img> rendering
	// it. The download attribute on a link handles saving, and takes its filename
	// from the path.
	if extension == svgExtension {
		w.Header().Set("Content-Type", "image/svg+xml")
	} else {
		w.Header().Set("Content-Type", "image/png")
	}

	if err := h.assetGenerator.WriteQRCode(w, code, h.assetGenerator.WithQRFormat(extension)); err != nil {
		h.logger.ErrorContext(r.Context(), "CodeQRCode: writing QR code", "error", err)
		// The header is already sent, so the response can only be cut short.
		return
	}
}

// splitCodeExtension takes the extension off the end of the wildcard, splitting
// on the last dot so a code containing one survives intact.
func splitCodeExtension(rest string) (string, string) {
	if decoded, err := url.PathUnescape(rest); err == nil {
		rest = decoded
	}
	dot := strings.LastIndex(rest, ".")
	if dot < 0 {
		return strings.TrimSpace(rest), ""
	}
	return strings.TrimSpace(rest[:dot]), rest[dot+1:]
}
