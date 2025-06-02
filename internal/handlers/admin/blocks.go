package handlers

import (
	"net/http"

	"github.com/go-chi/chi"
	templates "github.com/nathanhollows/Rapua/v3/internal/templates/blocks"
)

// BlockEdit shows the form to edit a block.
func (h *AdminHandler) BlockEdit(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	location := chi.URLParam(r, "location")
	if !h.GameManagerService.ValidateLocationID(user, location) {
		h.handleError(w, r, "BlockEdit: invalid location", "Could not find block", "location", location)
		return
	}

	blockID := chi.URLParam(r, "blockID")

	block, err := h.BlockService.GetByBlockID(r.Context(), blockID)
	if err != nil {
		h.handleError(w, r, "BlockEdit: getting block", "Could not find block", "error", err)
		return
	}

	if block.GetLocationID() != location {
		h.handleError(w, r, "BlockEdit: block does not belong to location", "Could not find block", "blockID", blockID, "location", location)
		return
	}

	err = templates.RenderAdminEdit(user.CurrentInstance.Settings, block).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("BlockEdit: rendering template", "error", err)
	}
}

// BlockEditPost updates the block.
func (h *AdminHandler) BlockEditPost(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	location := chi.URLParam(r, "location")
	if !h.GameManagerService.ValidateLocationID(user, location) {
		h.handleError(w, r, "BlockEditPost: invalid location", "Could not update block. Invalid location", "location", location)
		return
	}

	blockID := chi.URLParam(r, "blockID")

	block, err := h.BlockService.GetByBlockID(r.Context(), blockID)
	if err != nil {
		h.handleError(w, r, "BlockEditPost: getting block", "Could not update block", "error", err)
		return
	}

	if block.GetLocationID() != location {
		h.handleError(w, r, "BlockEditPost: block does not belong to location", "Could not update block", "blockID", blockID, "location", location)
		return
	}

	err = r.ParseForm()
	if err != nil {
		h.handleError(w, r, "BlockEditPost: parsing form", "Could not update block", "error", err)
		return
	}

	data := make(map[string][]string)
	for key, value := range r.Form {
		data[key] = value
	}

	_, err = h.BlockService.UpdateBlock(r.Context(), block, data)
	if err != nil {
		h.handleError(w, r, "BlockEditPost: updating block", "Could not update block", "error", err)
		return
	}

	h.handleSuccess(w, r, "Block updated")
}

// Show the form to edit the navigation settings.
func (h *AdminHandler) BlockNewPost(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	blockType := chi.URLParam(r, "type")

	locationID := chi.URLParam(r, "location")
	if !h.GameManagerService.ValidateLocationID(user, locationID) {
		h.handleError(w, r, "BlockNewPost: invalid location", "Could not create block. Invalid location", "location", locationID)
		return
	}

	location, err := h.LocationService.GetByID(r.Context(), locationID)
	if err != nil {
		h.handleError(w, r, "BlockNewPost: finding location", "Could not create block", "error", err)
		return
	}

	block, err := h.BlockService.NewBlock(r.Context(), location.ID, blockType)
	if err != nil {
		h.handleError(w, r, "BlockNewPost: creating block", "Could not create block", "error", err)
		return
	}

	err = templates.RenderAdminBlock(user.CurrentInstance.Settings, block, true).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("BlockNewPost: rendering template", "error", err)
	}
}

// BlockDelete deletes a block.
func (h *AdminHandler) BlockDelete(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	location := chi.URLParam(r, "location")
	blockID := chi.URLParam(r, "blockID")

	if location == "" || blockID == "" {
		h.handleError(w, r, "BlockDelete: missing parameters", "Could not delete block. Missing parameters")
		return
	}

	// Check if the user has access to the location
	if !h.GameManagerService.ValidateLocationID(user, location) {
		h.handleError(w, r, "BlockDelete: invalid location", "Could not delete block. Invalid location", "location", location)
		return
	}

	block, err := h.BlockService.GetByBlockID(r.Context(), blockID)
	if err != nil {
		h.handleError(w, r, "BlockDelete: getting block", "Could not delete block", "error", err)
		return
	}

	// Sanity check, but should never happen
	if block.GetLocationID() != location {
		h.handleError(w, r, "BlockDelete: block does not belong to location", "Could not delete block", "blockID", blockID, "location", location)
		return
	}

	err = h.BlockService.DeleteBlock(r.Context(), block.GetID())
	if err != nil {
		h.handleError(w, r, "BlockDelete: deleting block", "Could not delete block", "error", err)
		return
	}

	h.handleSuccess(w, r, "Block deleted")
}

// ReorderBlocks reorders the blocks.
func (h *AdminHandler) ReorderBlocks(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	location := chi.URLParam(r, "location")
	if !h.GameManagerService.ValidateLocationID(user, location) {
		h.handleError(w, r, "ReorderBlocks: invalid location", "Could not reorder blocks. Invalid location", "location", location)
		return
	}

	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "ReorderBlocks: parsing form", "Could not reorder blocks", "error", err)
		return
	}

	blockOrder := r.Form["block_id"]
	err = h.BlockService.ReorderBlocks(r.Context(), location, blockOrder)
	if err != nil {
		h.handleError(w, r, "ReorderBlocks: reordering blocks", "Could not reorder blocks", "error", err)
		return
	}

	h.handleSuccess(w, r, "Blocks reordered")
}
