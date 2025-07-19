package admin

import (
	"encoding/json"
	"net/http"

	"github.com/nathanhollows/Rapua/v3/internal/services"
	templates "github.com/nathanhollows/Rapua/v3/internal/templates/blocks"
)

func (h *AdminHandler) UploadMedia(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(100 << 20) // 100MB max
	if err != nil {
		h.handleError(w, r, "UploadMedia", "File too large", "error", err)
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		h.handleError(w, r, "UploadMedia", "Failed to get file", "error", err)
		return
	}
	defer file.Close()

	metadata := services.UploadMetadata{}

	metadata.InstanceID = r.Form.Get("instance_id")
	metadata.TeamID = r.Form.Get("team_id")
	metadata.BlockID = r.Form.Get("block_id")
	metadata.LocationID = r.Form.Get("location_id")

	media, err := h.uploadService.UploadFile(r.Context(), file, fileHeader, metadata)
	if err != nil {
		h.handleError(w, r, "UploadFiles", "Error uploading files", "error", err)
		return
	}

	if r.Form.Get("context") == "image_block" {
		err := templates.ImageAdminUpload(*media).Render(r.Context(), w)
		if err != nil {
			h.handleError(w, r, "UploadMedia", "Failed to render template", "error", err)
		}
	}

	h.handleSuccess(w, r, "File uploaded")
}

func (h *AdminHandler) UploadsSearch(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "Couldn't search images", "Failed to parse form", "error", err)
		return
	}

	filters := map[string]string{}

	if !r.Form.Has("instanceID") {
		h.handleError(w, r, "Malformed request", "Missing instanceID", "error", nil)
	}

	filters["instance_id"] = r.Form.Get("instanceID")
	filters["type"] = r.Form.Get("type")
	filters["team_code"] = r.Form.Get("teamCode")
	filters["block_id"] = r.Form.Get("blockID")
	filters["location_id"] = r.Form.Get("locationID")

	// Remove empty filters
	for key, value := range filters {
		if value == "" {
			delete(filters, key)
		}
	}

	uploads, err := h.uploadService.Search(r.Context(), filters)
	if err != nil {
		h.handleError(w, r, "Couldn't search images", "Failed to search images", "error", err)
		return
	}

	err = json.NewEncoder(w).Encode(uploads)
	if err != nil {
		h.handleError(w, r, "Couldn't search images", "Failed to encode response", "error", err)
	}
}
