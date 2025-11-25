package players

import (
	"encoding/json"
	"net/http"

	"github.com/nathanhollows/Rapua/v6/internal/services"
)

const maxUploadSize = 25 << 20 // 25MB

func (h *PlayerHandler) UploadImage(w http.ResponseWriter, r *http.Request) {
	// Set the maximum request body size
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	// The parameter to ParseMultipartForm is maxMemory (how much to keep in RAM before spilling to disk)
	err := r.ParseMultipartForm(maxUploadSize)
	if err != nil {
		h.handleError(w, r, "UploadImage", "File too large", "error", err)
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		h.handleError(w, r, "UploadImage", "Failed to get file", "error", err)
		return
	}
	defer file.Close()

	// Get team from context
	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		h.handleError(w, r, "UploadImage", "Failed to get team", "error", err)
		return
	}

	metadata := services.UploadMetadata{
		InstanceID: team.InstanceID,
		TeamID:     team.Code,
		BlockID:    r.Form.Get("block_id"),
		LocationID: r.Form.Get("location_id"),
	}

	media, err := h.uploadService.UploadFile(r.Context(), file, fileHeader, metadata)
	if err != nil {
		h.handleError(w, r, "UploadImage", "Error uploading file", "error", err)
		return
	}

	// Return JSON response with the upload URL
	response := map[string]string{
		"url": media.OriginalURL,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.handleError(w, r, "UploadImage", "Failed to encode response", "error", err)
	}
}
