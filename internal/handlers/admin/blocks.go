package admin

import (
	"encoding/json"
	"maps"
	"net/http"
	"slices"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/blocks"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/blocks"
)

// BlockCreate creates a new block using query parameters.
func (h *Handler) BlockCreate(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "BlockCreate: parsing form", "Could not create block", "error", err)
		return
	}

	// Parse query parameters
	ownerID := r.FormValue("owner")
	contextParam := r.FormValue("context")
	blockType := r.FormValue("type")

	// Validate required parameters
	if ownerID == "" {
		h.handleError(w, r, "BlockCreate: missing owner parameter", "Owner parameter is required", "owner", ownerID)
		return
	}
	if contextParam == "" {
		h.handleError(
			w,
			r,
			"BlockCreate: missing context parameter",
			"Context parameter is required",
			"context",
			contextParam,
		)
		return
	}
	if blockType == "" {
		h.handleError(w, r, "BlockCreate: missing type parameter", "Type parameter is required", "type", blockType)
		return
	}

	// Validate context parameter
	validContexts := []blocks.BlockContext{
		blocks.ContextStart,
		blocks.ContextFinish,
		blocks.ContextObjectiveProof,
		blocks.ContextObjectiveReveal,
	}
	blockContext := blocks.BlockContext(contextParam)
	isValidContext := slices.Contains(validContexts, blocks.BlockContext(contextParam))
	if !isValidContext {
		h.handleError(
			w,
			r,
			"BlockCreate: invalid context parameter",
			"Invalid context parameter",
			"context",
			contextParam,
		)
		return
	}

	// Check access based on context type (instance for start/complete, objective otherwise)
	access, err := h.accessService.CanAdminAccessBlockOwner(r.Context(), user.ID, ownerID, blockContext)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		h.handleError(w, r, "BlockCreate: checking access", "Could not create block", "error", err)
		return
	}

	if !access {
		w.WriteHeader(http.StatusForbidden)
		h.handleError(w, r, "BlockCreate: access denied", "Could not create block. Access denied", "owner", ownerID)
		return
	}

	block, err := h.blockService.NewBlockWithOwnerAndContext(r.Context(), ownerID, blockContext, blockType)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		h.handleError(w, r, "BlockCreate: creating block", "Could not create block", "error", err)
		return
	}

	err = templates.RenderAdminBlock(user.CurrentQuest.Settings, block, true).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "BlockCreate: rendering template", "error", err)
	}
}

// BlockGet retrieves a single block by ID.
// GET /admin/blocks/{id}.
func (h *Handler) BlockGet(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	blockID := chi.URLParam(r, "id")
	if blockID == "" {
		h.handleError(w, r, "BlockGet: missing block ID", "Block ID is required", "id", blockID)
		return
	}

	access, err := h.accessService.CanAdminAccessBlock(r.Context(), user.ID, blockID)
	if err != nil {
		h.handleError(w, r, "BlockGet: checking access", "Could not retrieve block", "error", err)
		return
	}
	if !access {
		h.handleError(w, r, "BlockGet: access denied", "Could not retrieve block. Access denied", "blockID", blockID)
		return
	}

	block, err := h.blockService.GetByBlockID(r.Context(), blockID)
	if err != nil {
		h.handleError(w, r, "BlockGet: getting block", "Could not find block", "error", err)
		return
	}

	err = templates.RenderAdminEdit(user.CurrentQuest.Settings, block).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "BlockGet: rendering template", "error", err)
	}
}

// BlockUpdate updates a block.
// PUT /admin/blocks/{id}.
func (h *Handler) BlockUpdate(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	blockID := chi.URLParam(r, "id")
	if blockID == "" {
		h.handleError(w, r, "BlockUpdate: missing block ID", "Block ID is required", "id", blockID)
		return
	}

	access, err := h.accessService.CanAdminAccessBlock(r.Context(), user.ID, blockID)
	if err != nil {
		h.handleError(w, r, "BlockUpdate: checking access", "Could not update block", "error", err)
		return
	}
	if !access {
		h.handleError(w, r, "BlockUpdate: access denied", "Could not update block. Access denied", "blockID", blockID)
		return
	}

	block, err := h.blockService.GetByBlockID(r.Context(), blockID)
	if err != nil {
		h.handleError(w, r, "BlockUpdate: getting block", "Could not update block", "error", err)
		return
	}

	err = r.ParseForm()
	if err != nil {
		h.handleError(w, r, "BlockUpdate: parsing form", "Could not update block", "error", err)
		return
	}

	data := make(map[string][]string)
	maps.Copy(data, r.PostForm)

	_, err = h.blockService.UpdateBlock(r.Context(), block, data)
	if err != nil {
		h.handleError(w, r, "BlockUpdate: updating block", "Could not update block", "error", err)
		return
	}

	h.handleSuccess(w, r, "Block updated")
}

// BlockDelete deletes a block by ID.
// DELETE /admin/blocks/{id}.
func (h *Handler) BlockDelete(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	blockID := chi.URLParam(r, "id")
	if blockID == "" {
		h.handleError(w, r, "BlockDeleteRESTful: missing block ID", "Block ID is required", "id", blockID)
		return
	}

	access, err := h.accessService.CanAdminAccessBlock(r.Context(), user.ID, blockID)
	if err != nil {
		h.handleError(w, r, "BlockDeleteRESTful: checking access", "Could not delete block", "error", err)
		return
	}
	if !access {
		h.handleError(
			w,
			r,
			"BlockDeleteRESTful: access denied",
			"Could not delete block. Access denied",
			"blockID",
			blockID,
		)
		return
	}

	block, err := h.blockService.GetByBlockID(r.Context(), blockID)
	if err != nil {
		h.handleError(w, r, "BlockDeleteRESTful: getting block", "Could not delete block", "error", err)
		return
	}

	err = h.deleteService.DeleteBlock(r.Context(), block.GetID())
	if err != nil {
		h.handleError(w, r, "BlockDeleteRESTful: deleting block", "Could not delete block", "error", err)
		return
	}

	h.handleSuccess(w, r, "Block deleted")
}

// BlockList lists blocks with optional filtering.
// GET /admin/blocks?owner={uuid}&context={context}.
func (h *Handler) BlockList(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	// Parse query parameters
	ownerID := r.URL.Query().Get("owner")
	contextParam := r.URL.Query().Get("context")

	var foundBlocks blocks.Blocks
	var err error

	if ownerID == "" {
		h.handleError(w, r, "BlockList: missing owner parameter", "Owner parameter is required", "owner", ownerID)
		return
	}

	// Determine context for access check
	var blockContext blocks.BlockContext
	if contextParam != "" {
		blockContext = blocks.BlockContext(contextParam)
	} else {
		// Default to objective-proof for context-agnostic queries: the access
		// check only cares which owner-type branch it takes (quest vs.
		// objective), and an objective owner is now the common case.
		blockContext = blocks.ContextObjectiveProof
	}

	// Check access to the owner (instance for start/complete, objective otherwise)
	access, err := h.accessService.CanAdminAccessBlockOwner(r.Context(), user.ID, ownerID, blockContext)
	if err != nil {
		h.handleError(w, r, "BlockList: checking access", "Could not list blocks", "error", err)
		return
	}
	if !access {
		h.handleError(w, r, "BlockList: access denied", "Could not list blocks. Access denied", "owner", ownerID)
		return
	}

	if contextParam != "" {
		// List blocks for specific owner and context
		foundBlocks, err = h.blockService.FindByOwnerIDAndContext(r.Context(), ownerID, blockContext)
	} else {
		// List all blocks for owner (context agnostic)
		foundBlocks, err = h.blockService.FindByOwnerID(r.Context(), ownerID)
	}

	if err != nil {
		h.handleError(w, r, "BlockList: finding blocks", "Could not list blocks", "error", err)
		return
	}

	// Return JSON response for API
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(foundBlocks)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "BlockList: encoding JSON", "error", err)
		h.handleError(w, r, "BlockList: encoding response", "Could not encode response", "error", err)
	}
}

// BlockReorder reorders blocks.
// POST /admin/blocks/reorder.
func (h *Handler) BlockReorder(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "BlockReorder: parsing form", "Could not reorder blocks", "error", err)
		return
	}

	blockOrder := r.Form["block_id"]

	for _, blockID := range blockOrder {
		access, accessErr := h.accessService.CanAdminAccessBlock(r.Context(), user.ID, blockID)
		if accessErr != nil {
			h.handleError(w, r, "BlockReorder: checking access", "Could not reorder blocks", "error", accessErr)
			return
		}
		if !access {
			h.handleError(
				w,
				r,
				"BlockReorder: access denied",
				"Could not reorder blocks. Access denied",
				"blockID",
				blockID,
			)
			return
		}
	}

	err = h.blockService.ReorderBlocks(r.Context(), blockOrder)
	if err != nil {
		h.handleError(w, r, "BlockReorder: reordering blocks", "Could not reorder blocks", "error", err)
		return
	}

	h.handleSuccess(w, r, "Blocks reordered")
}
