package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/nathanhollows/Rapua/v6/internal/services"
	templates "github.com/nathanhollows/Rapua/v6/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v6/models"
)

// Locations shows admin the locations.
func (h *Handler) Locations(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	// Load locations and their relations into the game structure recursively
	err := h.gameStructureService.LoadWithRelations(
		r.Context(),
		user.CurrentInstanceID,
		&user.CurrentInstance.GameStructure,
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
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	// Load blocks for all locations (needed for displaying clues/marker indicators)
	err = h.gameStructureService.LoadBlocksForStructure(
		r.Context(),
		&user.CurrentInstance.GameStructure,
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
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	c := templates.LocationGroupList(user.CurrentInstance.Settings, user.CurrentInstance.GameStructure)
	err = templates.Layout(c, *user, "Locations", "Locations").Render(r.Context(), w)
	if err != nil {
		h.logger.Error("Locations: rendering template", "error", err)
	}
}

// LocationNew shows the form to create a new location.
func (h *Handler) LocationNew(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	instances, err := h.instanceService.FindInstanceIDsForUser(r.Context(), user.ID)
	if err != nil {
		h.handleError(w, r, "LocationNew: getting instances", "Error getting instances", "error", err)
		return
	}

	duplicatable, err := h.markerService.FindMarkersNotInInstance(r.Context(), user.CurrentInstanceID, instances)
	if err != nil {
		h.handleError(
			w,
			r,
			"LocationNew: getting markers",
			"Error getting markers",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	data := templates.AddLocationData{
		Settings:     user.CurrentInstance.Settings,
		Neighbouring: user.CurrentInstance.Locations,
		Duplicatable: duplicatable,
	}

	c := templates.AddLocation(data)
	err = templates.Layout(c, *user, "Locations", "New Location").Render(r.Context(), w)
	if err != nil {
		h.logger.Error("LocationNew: rendering template", "error", err)
	}
}

// LocationNewPost handles creating a new location.
func (h *Handler) LocationNewPost(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := r.ParseForm()
	if err != nil {
		h.handleError(
			w,
			r,
			"LocationNewPost: parsing form",
			"Error parsing form",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	if !r.Form.Has("name") || r.FormValue("name") == "" {
		h.handleError(w, r, "LocationNewPost: missing name", "Location name is required")
		return
	}

	var lat, lng float64
	if r.FormValue("latitude") != "" {
		lat, err = strconv.ParseFloat(r.FormValue("latitude"), 64)
		if err != nil {
			h.handleError(
				w,
				r,
				"LocationNewPost: converting latitude",
				"Error converting latitude",
				"error",
				err,
				"instance_id",
				user.CurrentInstanceID,
			)
			return
		}
		lng, err = strconv.ParseFloat(r.FormValue("longitude"), 64)
		if err != nil {
			h.handleError(
				w,
				r,
				"LocationNewPost: converting longitude",
				"Error converting longitude",
				"error",
				err,
				"instance_id",
				user.CurrentInstanceID,
			)
			return
		}
	}

	points := 0
	if user.CurrentInstance.Settings.EnablePoints && r.FormValue("points") != "" {
		points, err = strconv.Atoi(r.FormValue("points"))
		if err != nil {
			h.handleError(
				w,
				r,
				"LocationNewPost: converting points",
				"Error converting points",
				"error",
				err,
				"instance_id",
				user.CurrentInstanceID,
			)
			return
		}
	}

	marker := r.FormValue("marker")
	location, err := h.createLocationWithOrWithoutMarker(w, r, user, marker, lat, lng, points)
	if err != nil {
		return
	}

	// Add the new location to the root group (unassigned area)
	if err = h.addLocationToRootGroup(r.Context(), user.CurrentInstanceID, location.ID); err != nil {
		h.logger.Error("LocationNewPost: adding location to root group", "error", err, "location_id", location.ID)
		// Don't fail the request - the location was created successfully
		// The orphan detection in Save will catch this later
	}

	h.redirect(w, r, "/admin/locations/"+location.MarkerID)
}

// createLocationWithOrWithoutMarker creates a location either from coordinates or from an existing marker.
func (h *Handler) createLocationWithOrWithoutMarker(
	w http.ResponseWriter,
	r *http.Request,
	user *models.User,
	marker string,
	lat, lng float64,
	points int,
) (models.Location, error) {
	if marker == "" {
		location, err := h.locationService.CreateLocation(
			r.Context(),
			user.CurrentInstanceID,
			r.FormValue("name"),
			lat,
			lng,
			points,
		)
		if err != nil {
			h.handleError(
				w,
				r,
				"LocationNewPost: creating location without marker",
				"Error creating location without marker",
				"error",
				err,
				"instance_id",
				user.CurrentInstanceID,
			)
			return models.Location{}, err
		}
		return location, nil
	}

	access, accessErr := h.accessService.CanAdminAccessMarker(r.Context(), user.ID, marker)
	if accessErr != nil {
		h.handleError(
			w,
			r,
			"LocationNewPost: checking marker access",
			"Error checking marker access",
			"error",
			accessErr,
			"instance_id",
			user.CurrentInstanceID,
		)
		return models.Location{}, accessErr
	}
	if !access {
		h.handleError(w, r, "LocationNewPost: no access to marker", "You do not have access to this marker")
		return models.Location{}, accessErr
	}

	location, err := h.locationService.CreateLocationFromMarker(
		r.Context(),
		user.CurrentInstanceID,
		r.FormValue("name"),
		points,
		marker,
	)
	if err != nil {
		h.handleError(
			w,
			r,
			"LocationNewPost: creating location from marker",
			"Error creating location from marker",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return models.Location{}, err
	}
	return location, nil
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
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	locations := r.Form["location"]
	err = h.locationService.ReorderLocations(r.Context(), user.CurrentInstanceID, locations)
	if err != nil {
		h.handleError(
			w,
			r,
			"ReorderLocations: reordering locations",
			"Error reordering locations",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
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
	code := chi.URLParam(r, "id")
	user := h.UserFromContext(r.Context())

	location, err := h.locationService.GetByInstanceAndCode(r.Context(), user.CurrentInstanceID, code)
	if err != nil {
		h.logger.Error(
			"LocationEdit: finding location",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
			"location_code",
			code,
		)
		h.redirect(w, r, "/admin/locations")
		return
	}

	contentBlocks, err := h.blockService.FindByOwnerIDAndContext(
		r.Context(),
		location.ID,
		blocks.ContextLocationContent,
	)
	if err != nil {
		h.logger.Error(
			"LocationEdit: getting blocks",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
			"location_id",
			location.ID,
		)
		h.redirect(w, r, "/admin/locations")
		return
	}

	navigationBlocks, err := h.blockService.FindByOwnerIDAndContext(
		r.Context(),
		location.ID,
		blocks.ContextLocationClues,
	)
	if err != nil {
		h.logger.Error(
			"LocationEdit: getting blocks",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
			"location_id",
			location.ID,
		)
		h.redirect(w, r, "/admin/locations")
		return
	}

	// Find the parent group to get navigation settings
	parentGroup := h.gameStructureService.FindGroupByLocationID(&user.CurrentInstance.GameStructure, location.ID)
	navigationMode := models.NavigationDisplayCustom // Default to custom
	if parentGroup != nil {
		navigationMode = parentGroup.Navigation
	} else {
		h.logger.Warn(
			"LocationEdit: no parent group found",
			"location_id",
			location.ID,
			"using_default",
			navigationMode,
		)
	}

	data := templates.EditLocationData{
		Settings:         user.CurrentInstance.Settings,
		Location:         *location,
		ContentBlocks:    contentBlocks,
		NavigationBlocks: navigationBlocks,
		NavigationMode:   navigationMode,
	}

	c := templates.EditLocation(data)
	err = templates.Layout(c, *user, "Locations", "Edit Location").Render(r.Context(), w)
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
	locationCode := chi.URLParam(r, "id")

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

	location, err := h.locationService.GetByInstanceAndCode(r.Context(), user.CurrentInstanceID, locationCode)
	if err != nil {
		h.handleError(w, r, "LocationEditPost: finding location", "Error finding location", "error", err)
		return
	}

	markerID := location.MarkerID
	err = h.locationService.UpdateLocation(r.Context(), location, data)
	if err != nil {
		h.handleError(w, r, "LocationEditPost: updating location", "Error updating location", "error", err)
		return
	}

	if markerID != location.MarkerID {
		h.redirect(w, r, "/admin/locations/"+location.MarkerID)
		return
	}

	h.handleSuccess(w, r, "Location updated")
}

// LocationDelete handles deleting a location.
func (h *Handler) LocationDelete(w http.ResponseWriter, r *http.Request) {
	locationCode := chi.URLParam(r, "id")

	user := h.UserFromContext(r.Context())

	location, err := h.locationService.GetByInstanceAndCode(r.Context(), user.CurrentInstanceID, locationCode)
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

	h.redirect(w, r, "/admin/locations")
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
			"instance_id",
			user.CurrentInstanceID,
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
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	// Validate and save using the service
	if err := h.gameStructureService.Save(r.Context(), user.CurrentInstanceID, &structure); err != nil {
		h.handleError(
			w,
			r,
			"SaveGameStructure: saving structure",
			"Validation failed: "+err.Error(),
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	h.handleSuccess(w, r, "Game structure saved")
}

// addLocationToRootGroup adds a newly created location to the root group (unassigned area).
func (h *Handler) addLocationToRootGroup(ctx context.Context, instanceID, locationID string) error {
	// Load the current instance
	instance, err := h.instanceService.GetByID(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("loading instance: %w", err)
	}

	// Add the location to the root group
	instance.GameStructure.LocationIDs = append(instance.GameStructure.LocationIDs, locationID)

	// Save the updated structure
	if err = h.gameStructureService.Save(ctx, instanceID, &instance.GameStructure); err != nil {
		return fmt.Errorf("saving structure: %w", err)
	}

	return nil
}

// StartPageEdit shows the start page editor.
func (h *Handler) StartPageEdit(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	// Get blocks for the start page.
	pageBlocks, err := h.blockService.FindByOwnerIDAndContext(
		r.Context(),
		user.CurrentInstanceID,
		blocks.ContextStart,
	)
	if err != nil {
		h.logger.Error(
			"StartPageEdit: getting blocks",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		h.redirect(w, r, "/admin/locations")
		return
	}

	data := templates.EditPageData{
		Settings:   user.CurrentInstance.Settings,
		PageBlocks: pageBlocks,
		PageTitle:  "Start",
		PageType:   "start",
	}

	c := templates.EditPage(data)
	err = templates.Layout(c, *user, "Locations", "Edit Start Page").Render(r.Context(), w)
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
		user.CurrentInstanceID,
		blocks.ContextFinish,
	)
	if err != nil {
		h.logger.Error(
			"completePageEdit: getting blocks",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		h.redirect(w, r, "/admin/locations")
		return
	}

	data := templates.EditPageData{
		Settings:   user.CurrentInstance.Settings,
		PageBlocks: pageBlocks,
		PageTitle:  "Complete",
		PageType:   "complete",
	}

	c := templates.EditPage(data)
	err = templates.Layout(c, *user, "Locations", "Edit Complete Page").Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "CompletePageEdit: rendering template", "Error rendering template", "error", err)
	}
}
