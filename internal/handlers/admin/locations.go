package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v8/models"
)

// Locations shows admin the locations.
func (h *Handler) Locations(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	// Load locations and their relations into the game structure recursively
	err := h.gameStructureService.LoadWithRelations(
		r.Context(),
		user.CurrentQuestID,
		&user.CurrentQuest.GameStructure,
		true, // recursive
	)
	if err != nil {
		h.handleError(
			w,
			r,
			"Locations: loading game structure",
			"Error loading locations",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	// Load blocks for all locations (needed for displaying clues/marker indicators)
	err = h.gameStructureService.LoadBlocksForStructure(
		r.Context(),
		&user.CurrentQuest.GameStructure,
		true, // recursive
	)
	if err != nil {
		h.handleError(
			w,
			r,
			"Locations: loading blocks",
			"Error loading location blocks",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	c := templates.LocationGroupList(user.CurrentQuest.Settings, user.CurrentQuest.GameStructure)
	err = templates.Layout(c, *user, "Quest", "Quest").Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Locations: rendering template", "error", err)
	}
}

// LocationNew creates a new location with a default name and redirects to the edit page.
func (h *Handler) LocationNew(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	location, err := h.locationService.CreateLocation(
		r.Context(), user.CurrentQuestID, "New Location", 0, 0, 0,
	)
	if err != nil {
		h.handleError(w, r, "LocationNew: creating location", "Error creating location", "error", err)
		return
	}

	groupID := r.URL.Query().Get("groupId")
	afterLocationID := r.URL.Query().Get("afterLocationId")
	beforeLocationID := r.URL.Query().Get("beforeLocationId")

	if groupID != "" { //nolint:nestif // linear structure placement logic with adjacent-ID fallback
		if err = h.gameStructureService.InsertLocationIntoGroup(
			r.Context(), user.CurrentQuestID, location.ID, groupID, afterLocationID, beforeLocationID,
		); err != nil {
			h.logger.ErrorContext(
				r.Context(),
				"LocationNew: inserting location into group",
				"error",
				err,
				"location_id",
				location.ID,
			)
		}
		adjacentID := afterLocationID
		if adjacentID == "" {
			adjacentID = beforeLocationID
		}
		if adjacentID != "" {
			h.copyClueBlockIfSingle(r.Context(), location.ID, adjacentID)
		}
	} else {
		if err = h.addLocationToRootGroup(r.Context(), user.CurrentQuestID, location.ID); err != nil {
			h.logger.ErrorContext(
				r.Context(),
				"LocationNew: adding location to root group",
				"error",
				err,
				"location_id",
				location.ID,
			)
		}
	}

	editPath := "/admin/objective/" + location.Slug
	if r.Header.Get("Hx-Request") == "true" {
		w.Header().Set("Hx-Location", editPath)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, editPath, http.StatusFound)
}

// ReorderLocations handles reordering locations.
// Returns a 200 status code if successful,
// Otherwise, returns a 500 status code.
func (h *Handler) ReorderLocations(w http.ResponseWriter, r *http.Request) {
	// Check HTMX headers
	if r.Header.Get("Hx-Request") != htmxHeaderTrue {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	user := h.UserFromContext(r.Context())

	err := r.ParseForm()
	if err != nil {
		h.handleError(
			w,
			r,
			"ReorderLocations: parsing form",
			"Error parsing form",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	locations := r.Form["location"]
	err = h.locationService.ReorderLocations(r.Context(), user.CurrentQuestID, locations)
	if err != nil {
		h.handleError(
			w,
			r,
			"ReorderLocations: reordering locations",
			"Error reordering locations",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	h.handleSuccess(w, r, "Order updated")
}

// LocationEdit shows the form to edit a location.
func (h *Handler) LocationEdit(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "parsing form", "Error parsing form", "error", err)
		return
	}

	// Get the location from the chi context
	slug := chi.URLParam(r, "slug")
	user := h.UserFromContext(r.Context())

	location, err := h.locationService.GetByInstanceAndSlug(r.Context(), user.CurrentQuestID, slug)
	if err != nil {
		h.logger.ErrorContext(r.Context(),
			"LocationEdit: finding location",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
			"location_slug",
			slug,
		)
		h.redirect(w, r, "/admin/quest")
		return
	}

	contentBlocks, err := h.blockService.FindByOwnerIDAndContext(
		r.Context(),
		location.ID,
		blocks.ContextLocationContent,
	)
	if err != nil {
		h.logger.ErrorContext(r.Context(),
			"LocationEdit: getting blocks",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
			"location_id",
			location.ID,
		)
		h.redirect(w, r, "/admin/quest")
		return
	}

	// Load navigation blocks for this location
	navigationBlocks, err := h.blockService.FindByOwnerIDAndContext(
		r.Context(),
		location.ID,
		blocks.ContextNavigation,
	)
	if err != nil {
		h.logger.ErrorContext(r.Context(),
			"LocationEdit: getting navigation blocks",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
			"location_id",
			location.ID,
		)
		h.redirect(w, r, "/admin/quest")
		return
	}

	data := templates.EditLocationData{
		Settings:         user.CurrentQuest.Settings,
		Location:         *location,
		ContentBlocks:    contentBlocks,
		NavigationBlocks: navigationBlocks,
	}

	c := templates.EditLocation(data)
	err = templates.Layout(c, *user, "Quest", "Edit Location").Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "LocationEdit: rendering template", "Error rendering template", "error", err)
	}
}

// LocationEditPost handles updating a location.
func (h *Handler) LocationEditPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "LocationEditPost: parsing form", "Error parsing form", "error", err)
		return
	}

	user := h.UserFromContext(r.Context())
	locationSlug := chi.URLParam(r, "slug")

	var points int
	var err error
	if r.FormValue("points") == "" {
		points = -1
	} else {
		points, err = strconv.Atoi(r.FormValue("points"))
		if err != nil {
			h.handleError(w, r, "LocationEditPost: converting points", "Error converting points", "error", err)
			return
		}
	}

	var lat, lng float64
	if r.FormValue("latitude") != "" {
		lat, err = strconv.ParseFloat(r.FormValue("latitude"), 64)
		if err != nil {
			h.handleError(w, r, "LocationEditPost: converting latitude", "Error converting latitude", "error", err)
			return
		}
	}

	if r.FormValue("longitude") != "" {
		lng, err = strconv.ParseFloat(r.FormValue("longitude"), 64)
		if err != nil {
			h.handleError(w, r, "LocationEditPost: converting longitude", "Error converting longitude", "error", err)
			return
		}
	}

	data := services.LocationUpdateData{
		Name:      r.FormValue("name"),
		Latitude:  lat,
		Longitude: lng,
		Points:    points,
	}

	location, err := h.locationService.GetByInstanceAndSlug(r.Context(), user.CurrentQuestID, locationSlug)
	if err != nil {
		h.handleError(w, r, "LocationEditPost: finding location", "Error finding location", "error", err)
		return
	}

	err = h.locationService.UpdateLocation(r.Context(), location, data)
	if err != nil {
		h.handleError(w, r, "LocationEditPost: updating location", "Error updating location", "error", err)
		return
	}

	if location.Slug != locationSlug {
		h.redirect(w, r, "/admin/objective/"+location.Slug)
		return
	}

	h.handleSuccess(w, r, "Location updated")
}

// LocationDelete handles deleting a location.
func (h *Handler) LocationDelete(w http.ResponseWriter, r *http.Request) {
	locationSlug := chi.URLParam(r, "slug")

	user := h.UserFromContext(r.Context())

	location, err := h.locationService.GetByInstanceAndSlug(r.Context(), user.CurrentQuestID, locationSlug)
	if err != nil {
		h.handleError(w, r, "LocationDelete: finding location", "Error finding location", "error", err)
		return
	}

	if location.ID == "" {
		h.handleError(w, r, "LocationDelete: location not found", "Location not found")
		return
	}

	if err = h.deleteService.DeleteLocation(r.Context(), location.ID); err != nil {
		h.handleError(w, r, "LocationDelete: deleting location", "Error deleting location", "error", err)
		return
	}

	h.redirect(w, r, "/admin/quest")
}

// SaveGameStructure handles saving the game structure from the browser.
func (h *Handler) SaveGameStructure(w http.ResponseWriter, r *http.Request) {
	// Check HTMX headers
	if r.Header.Get("Hx-Request") != htmxHeaderTrue {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	user := h.UserFromContext(r.Context())

	// Parse form data
	if err := r.ParseForm(); err != nil {
		h.handleError(
			w,
			r,
			"SaveGameStructure: parsing form",
			"Error parsing form",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	// Get the structure JSON from form
	structureJSON := r.FormValue("structure")
	if structureJSON == "" {
		h.handleError(w, r, "SaveGameStructure: missing structure", "Missing structure data")
		return
	}

	// Parse the JSON
	var structure models.GameStructure
	if err := json.Unmarshal([]byte(structureJSON), &structure); err != nil {
		h.handleError(
			w,
			r,
			"SaveGameStructure: decoding JSON",
			"Invalid JSON: "+err.Error(),
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	// Validate and save using the service
	if err := h.gameStructureService.Save(r.Context(), user.CurrentQuestID, &structure); err != nil {
		h.handleError(
			w,
			r,
			"SaveGameStructure: saving structure",
			"Validation failed: "+err.Error(),
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	h.handleSuccess(w, r, "Game structure saved")
}

// copyClueBlockIfSingle copies the navigation block type from adjacentLocationID to newLocationID
// when the adjacent location has exactly one navigation block. Errors are non-fatal.
func (h *Handler) copyClueBlockIfSingle(ctx context.Context, newLocationID, adjacentLocationID string) {
	navBlocks, err := h.blockService.FindByOwnerIDAndContext(ctx, adjacentLocationID, blocks.ContextNavigation)
	if err != nil || len(navBlocks) != 1 {
		return
	}
	if _, err = h.blockService.NewBlockWithOwnerAndContext(
		ctx,
		newLocationID,
		blocks.ContextNavigation,
		navBlocks[0].GetType(),
	); err != nil {
		h.logger.WarnContext(ctx, "copyClueBlockIfSingle: creating navigation block",
			"error", err, "location_id", newLocationID)
	}
}

// addLocationToRootGroup adds a newly created location to the root group (unassigned area).
func (h *Handler) addLocationToRootGroup(ctx context.Context, questID, locationID string) error {
	// Load the current instance
	instance, err := h.questService.GetByID(ctx, questID)
	if err != nil {
		return fmt.Errorf("loading instance: %w", err)
	}

	// Add the location to the root group
	instance.GameStructure.LocationIDs = append(instance.GameStructure.LocationIDs, locationID)

	// Save the updated structure
	if err = h.gameStructureService.Save(ctx, questID, &instance.GameStructure); err != nil {
		return fmt.Errorf("saving structure: %w", err)
	}

	return nil
}

// Spaces shows a placeholder page for the Spaces feature (not yet implemented).
func (h *Handler) Spaces(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	c := templates.SpacesPage()
	err := templates.Layout(c, *user, "Spaces", "Spaces").Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Spaces: rendering template", "error", err)
	}
}

// StartPageEdit shows the start page editor.
func (h *Handler) StartPageEdit(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	// Get blocks for the start page.
	pageBlocks, err := h.blockService.FindByOwnerIDAndContext(
		r.Context(),
		user.CurrentQuestID,
		blocks.ContextStart,
	)
	if err != nil {
		h.logger.ErrorContext(r.Context(),
			"StartPageEdit: getting blocks",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		h.redirect(w, r, "/admin/quest")
		return
	}

	data := templates.EditPageData{
		Settings:   user.CurrentQuest.Settings,
		PageBlocks: pageBlocks,
		PageTitle:  "Start",
		PageType:   "start",
	}

	c := templates.EditPage(data)
	err = templates.Layout(c, *user, "Quest", "Edit Start Page").Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "StartPageEdit: rendering template", "Error rendering template", "error", err)
	}
}

// CompletePageEdit shows the complete page editor.
func (h *Handler) CompletePageEdit(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	// Get blocks for the complete page
	pageBlocks, err := h.blockService.FindByOwnerIDAndContext(
		r.Context(),
		user.CurrentQuestID,
		blocks.ContextFinish,
	)
	if err != nil {
		h.logger.ErrorContext(r.Context(),
			"completePageEdit: getting blocks",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		h.redirect(w, r, "/admin/quest")
		return
	}

	data := templates.EditPageData{
		Settings:   user.CurrentQuest.Settings,
		PageBlocks: pageBlocks,
		PageTitle:  "Complete",
		PageType:   "complete",
	}

	c := templates.EditPage(data)
	err = templates.Layout(c, *user, "Quest", "Edit Complete Page").Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "CompletePageEdit: rendering template", "Error rendering template", "error", err)
	}
}
