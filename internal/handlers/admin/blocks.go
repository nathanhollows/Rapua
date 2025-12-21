package admin

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v6/blocks"
	templates "github.com/nathanhollows/Rapua/v6/internal/templates/blocks"
)

// BlockEdit shows the form to edit a block (legacy method).
func (h *Handler) BlockEdit(w http.ResponseWriter, r *http.Request) {
	// Extract blockID from legacy URL structure
	blockID := chi.URLParam(r, "blockID")

	// Add deprecation header
	w.Header().Set("X-Deprecated", "Use GET /admin/blocks/{id} instead")

	// Update chi URL parameters to match new structure
	rctx := chi.RouteContext(r.Context())
	rctx.URLParams.Add("id", blockID)

	// Call the new handler directly
	h.BlockGet(w, r)
}

// BlockEditPost updates the block (legacy method).
func (h *Handler) BlockEditPost(w http.ResponseWriter, r *http.Request) {
	// Extract blockID from legacy URL structure
	blockID := chi.URLParam(r, "blockID")

	// Add deprecation header
	w.Header().Set("X-Deprecated", "Use PUT /admin/blocks/{id} instead")

	// Update chi URL parameters to match new structure
	rctx := chi.RouteContext(r.Context())
	rctx.URLParams.Add("id", blockID)

	// Call the new handler directly
	h.BlockUpdate(w, r)
}

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
		blocks.ContextLocationContent,
		blocks.ContextLocationClues,
		blocks.ContextTasks,
		blocks.ContextCheckpoint,
		blocks.ContextStart,
		blocks.ContextFinish,
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

	// Check access based on context type (instance for start/finish, location otherwise)
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

	// Set success status code
	w.WriteHeader(http.StatusCreated)
	err = templates.RenderAdminBlock(user.CurrentInstance.Settings, block, true).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("BlockCreate: rendering template", "error", err)
	}
}

// BlockNewWithOwnerAndContextPost creates a new block with owner and context (legacy path-based parameters).
func (h *Handler) BlockNewWithOwnerAndContextPost(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	ownerID := chi.URLParam(r, "owner")
	blockType := chi.URLParam(r, "type")
	contextParam := chi.URLParam(r, "context")

	// Parse context parameter to BlockContext
	blockContext := blocks.BlockContext(contextParam)

	// For now, assume if owner is a location, we can check location access
	// In the future, this could be expanded to handle different owner types
	access, err := h.accessService.CanAdminAccessLocation(r.Context(), user.ID, ownerID)
	if err != nil {
		h.handleError(w, r, "BlockNewWithOwnerAndContextPost: checking access", "Could not create block", "error", err)
		return
	}
	if !access {
		h.handleError(
			w,
			r,
			"BlockNewWithOwnerAndContextPost: access denied",
			"Could not create block. Access denied",
			"owner",
			ownerID,
		)
		return
	}

	block, err := h.blockService.NewBlockWithOwnerAndContext(r.Context(), ownerID, blockContext, blockType)
	if err != nil {
		h.handleError(w, r, "BlockNewWithOwnerAndContextPost: creating block", "Could not create block", "error", err)
		return
	}

	err = templates.RenderAdminBlock(user.CurrentInstance.Settings, block, true).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("BlockNewWithOwnerAndContextPost: rendering template", "error", err)
	}
}

// BlockNewPost creates a new block for a location (legacy method).
func (h *Handler) BlockNewPost(w http.ResponseWriter, r *http.Request) {
	// Extract parameters from legacy URL structure
	blockType := chi.URLParam(r, "type")
	locationID := chi.URLParam(r, "location")

	// Add deprecation header
	w.Header().Set("X-Deprecated", "Use POST /admin/blocks with query parameters instead")

	// Create new request with query parameters
	r.URL.RawQuery = fmt.Sprintf("owner=%s&context=location_content&type=%s", locationID, blockType)

	// Call the new handler directly instead of redirecting to preserve POST data
	h.BlockCreate(w, r)
}

// ReorderBlocks reorders the blocks (legacy method).
func (h *Handler) ReorderBlocks(w http.ResponseWriter, r *http.Request) {
	// Add deprecation header
	w.Header().Set("X-Deprecated", "Use POST /admin/blocks/reorder instead")

	// Call the new handler directly
	h.BlockReorder(w, r)
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

	err = templates.RenderAdminEdit(user.CurrentInstance.Settings, block).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("BlockGet: rendering template", "error", err)
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

// BlockInnerEditor returns the inner block editor for a task block.
// GET /admin/blocks/{id}/inner-editor.
func (h *Handler) BlockInnerEditor(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	blockID := chi.URLParam(r, "id")
	if blockID == "" {
		h.handleError(w, r, "BlockInnerEditor: missing block ID", "Block ID is required", "id", blockID)
		return
	}

	access, err := h.accessService.CanAdminAccessBlock(r.Context(), user.ID, blockID)
	if err != nil {
		h.handleError(w, r, "BlockInnerEditor: checking access", "Could not retrieve block", "error", err)
		return
	}
	if !access {
		h.handleError(w, r, "BlockInnerEditor: access denied", "Access denied", "blockID", blockID)
		return
	}

	block, err := h.blockService.GetByBlockID(r.Context(), blockID)
	if err != nil {
		h.handleError(w, r, "BlockInnerEditor: getting block", "Could not find block", "error", err)
		return
	}

	// Ensure it's a task block
	taskBlock, ok := block.(*blocks.TaskBlock)
	if !ok {
		h.handleError(w, r, "BlockInnerEditor: block is not a task", "Block is not a task block", "blockID", blockID)
		return
	}

	// Get inner_type from query params
	innerType := r.URL.Query().Get("inner_type")
	if innerType != "" && innerType != taskBlock.InnerType {
		// Create temporary task block with new inner type
		taskBlock = &blocks.TaskBlock{
			BaseBlock: taskBlock.BaseBlock,
			TaskName:  taskBlock.TaskName,
			InnerType: innerType,
		}

		// Create and parse new inner block
		innerBase := blocks.BaseBlock{
			ID:         taskBlock.ID + "_inner",
			LocationID: taskBlock.LocationID,
			Type:       innerType,
			Order:      0,
			Points:     taskBlock.Points,
		}
		inner, err := blocks.CreateFromBaseBlock(innerBase)
		if err == nil {
			// Set inner block directly to avoid serialization
			taskBlock.SetInnerBlock(inner)
		}
	}

	// Render just the inner editor section
	err = templates.TaskInnerEditor(user.CurrentInstance.Settings, *taskBlock).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("BlockInnerEditor: rendering template", "error", err)
		h.handleError(w, r, "BlockInnerEditor: render failed", "Could not load editor", "error", err)
		return
	}
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
		// Default to location context for context-agnostic queries
		blockContext = blocks.ContextLocationContent
	}

	// Check access to the owner (instance for start/finish, location otherwise)
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
		h.logger.Error("BlockList: encoding JSON", "error", err)
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
