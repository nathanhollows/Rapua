package admin

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/blocks"
)

// blockForVars loads the block a variable request names, after checking the
// user may edit it. It writes the error itself and returns nil on failure.
func (h *Handler) blockForVars(w http.ResponseWriter, r *http.Request, action string) blocks.Block {
	user := h.UserFromContext(r.Context())
	blockID := chi.URLParam(r, "id")

	access, err := h.accessService.CanAdminAccessBlock(r.Context(), user.ID, blockID)
	if err != nil {
		h.handleError(w, r, action+": checking access", "Could not load block", "error", err)
		return nil
	}
	if !access {
		h.handleError(w, r, action+": access denied", "Could not load block. Access denied", "blockID", blockID)
		return nil
	}
	block, err := h.blockService.GetByBlockID(r.Context(), blockID)
	if err != nil {
		h.handleError(w, r, action+": getting block", "Could not find block", "error", err)
		return nil
	}
	return block
}

// BlockVarsEdit opens the inline editor for what a block sets or waits on,
// offering every variable the quest already sets.
// GET /admin/blocks/{id}/vars.
func (h *Handler) BlockVarsEdit(w http.ResponseWriter, r *http.Request) {
	block := h.blockForVars(w, r, "BlockVarsEdit")
	if block == nil {
		return
	}
	user := h.UserFromContext(r.Context())

	// Suggestions are a convenience: an editor without them still works.
	known, err := h.variableService.QuestVariables(r.Context(), user.CurrentQuestID)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "BlockVarsEdit: listing quest variables", "error", err)
	}
	if err := templates.BlockVarsEditor(block, known).Render(r.Context(), w); err != nil {
		h.logger.ErrorContext(r.Context(), "BlockVarsEdit: rendering editor", "error", err)
	}
}

// BlockVarsUpdate saves what a block sets or waits on and folds the editor
// back into its button.
// PUT /admin/blocks/{id}/vars.
func (h *Handler) BlockVarsUpdate(w http.ResponseWriter, r *http.Request) {
	block := h.blockForVars(w, r, "BlockVarsUpdate")
	if block == nil {
		return
	}
	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "BlockVarsUpdate: parsing form", "Could not save the variable", "error", err)
		return
	}

	// A field the form did not send is left alone, so the key editor cannot
	// clear a lock and the other way round.
	var sets *string
	if _, ok := r.PostForm["sets"]; ok {
		value := r.PostForm.Get("sets")
		sets = &value
	}
	var depends *[]string
	if values, ok := r.PostForm["depends"]; ok {
		depends = &values
	}

	updated, err := h.blockService.UpdateBlockVars(r.Context(), block, sets, depends)
	if errors.Is(err, services.ErrVarsNotAllowed) || errors.Is(err, services.ErrInvalidVarName) {
		h.handleError(w, r, "BlockVarsUpdate: refused", err.Error(), "error", err)
		return
	}
	if err != nil {
		h.handleError(w, r, "BlockVarsUpdate: saving", "Could not save the variable", "error", err)
		return
	}

	if err := templates.BlockVarsButtonOOB(updated).Render(r.Context(), w); err != nil {
		h.logger.ErrorContext(r.Context(), "BlockVarsUpdate: rendering button", "error", err)
	}
	h.refreshLintPanel(w, r, updated.GetOwnerID(), h.contextOfBlock(r, updated))
	h.handleSuccess(w, r, "Saved")
}
