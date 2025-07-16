package handlers

import (
	"net/http"

	"github.com/go-chi/chi"
	templates "github.com/nathanhollows/Rapua/v3/internal/templates/blocks"
)

// BlockEdit shows the form to edit a block.
func (h *AdminHandler) BlockEdit(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	blockID := chi.URLParam(r, "blockID")

	access, err := h.accessService.CanAdminAccessBlock(r.Context(), user.ID, blockID)
	if err != nil {
		h.handleError(w, r, "BlockEditPost: checking access", "Could not update block", "error", err)
		return
	}
	if !access {
		h.handleError(w, r, "BlockEditPost: access denied", "Could not update block. Access denied", "blockID", blockID)
		return
	}

	block, err := h.blockService.GetByBlockID(r.Context(), blockID)
	if err != nil {
		h.handleError(w, r, "BlockEdit: getting block", "Could not find block", "error", err)
		return
	}

	err = templates.RenderAdminEdit(user.CurrentInstance.Settings, block).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("BlockEdit: rendering template", "error", err)
	}
}

// BlockEditPost updates the block.
func (h *AdminHandler) BlockEditPost(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	blockID := chi.URLParam(r, "blockID")

	access, err := h.accessService.CanAdminAccessBlock(r.Context(), user.ID, blockID)
	if err != nil {
		h.handleError(w, r, "BlockEditPost: checking access", "Could not update block", "error", err)
		return
	}
	if !access {
		h.handleError(w, r, "BlockEditPost: access denied", "Could not update block. Access denied", "blockID", blockID)
		return
	}

	block, err := h.blockService.GetByBlockID(r.Context(), blockID)
	if err != nil {
		h.handleError(w, r, "BlockEditPost: getting block", "Could not update block", "error", err)
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

	_, err = h.blockService.UpdateBlock(r.Context(), block, data)
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
	access, err := h.accessService.CanAdminAccessLocation(r.Context(), user.ID, locationID)
	if err != nil {
		h.handleError(w, r, "BlockNewPost: checking access", "Could not create block", "error", err)
		return
	}
	if !access {
		h.handleError(w, r, "BlockNewPost: access denied", "Could not create block. Access denied", "location", locationID)
		return
	}

	location, err := h.locationService.GetByID(r.Context(), locationID)
	if err != nil {
		h.handleError(w, r, "BlockNewPost: finding location", "Could not create block", "error", err)
		return
	}

	block, err := h.blockService.NewBlock(r.Context(), location.ID, blockType)
	if err != nil {
		h.handleError(w, r, "BlockNewPost: creating block", "Could not create block", "error", err)
		return
	}

	err = templates.RenderAdminBlock(user.CurrentInstance.Settings, block, true).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("BlockNewPost: rendering template", "error", err)
	}
}

// BlockDelete deletes a block.
func (h *AdminHandler) BlockDelete(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	blockID := chi.URLParam(r, "blockID")

	access, err := h.accessService.CanAdminAccessBlock(r.Context(), user.ID, blockID)
	if err != nil {
		h.handleError(w, r, "BlockDelete: checking access", "Could not delete block", "error", err)
		return
	}
	if !access {
		h.handleError(w, r, "BlockDelete: access denied", "Could not delete block. Access denied", "blockID", blockID)
		return
	}

	block, err := h.blockService.GetByBlockID(r.Context(), blockID)
	if err != nil {
		h.handleError(w, r, "BlockDelete: getting block", "Could not delete block", "error", err)
		return
	}

	err = h.deleteService.DeleteBlock(r.Context(), block.GetID())
	if err != nil {
		h.handleError(w, r, "BlockDelete: deleting block", "Could not delete block", "error", err)
		return
	}

	h.handleSuccess(w, r, "Block deleted")
}

// ReorderBlocks reorders the blocks.
func (h *AdminHandler) ReorderBlocks(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "ReorderBlocks: parsing form", "Could not reorder blocks", "error", err)
		return
	}

	blockOrder := r.Form["block_id"]

	for _, blockID := range blockOrder {
		access, err := h.accessService.CanAdminAccessBlock(r.Context(), user.ID, blockID)
		if err != nil {
			h.handleError(w, r, "ReorderBlocks: checking access", "Could not reorder blocks", "error", err)
			return
		}
		if !access {
			h.handleError(w, r, "ReorderBlocks: access denied", "Could not reorder blocks. Access denied", "blockID", blockID)
			return
		}
	}

	err = h.blockService.ReorderBlocks(r.Context(), blockOrder)
	if err != nil {
		h.handleError(w, r, "ReorderBlocks: reordering blocks", "Could not reorder blocks", "error", err)
		return
	}

	h.handleSuccess(w, r, "Blocks reordered")
}
