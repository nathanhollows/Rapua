package admin

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v4/internal/services"
	templates "github.com/nathanhollows/Rapua/v4/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v4/models"
)

// Locations shows admin the locations.
func (h *AdminHandler) Locations(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	for i, location := range user.CurrentInstance.Locations {
		err := h.locationService.LoadRelations(r.Context(), &location)
		if err != nil {
			h.handleError(w, r, "Locations: loading relations", "Error loading relations", "error", err, "instance_id", user.CurrentInstanceID)
			return
		}
		user.CurrentInstance.Locations[i] = location
	}

	c := templates.LocationsIndex(user.CurrentInstance.Settings, user.CurrentInstance.Locations)
	err := templates.Layout(c, *user, "Locations", "Locations").Render(r.Context(), w)
	if err != nil {
		h.logger.Error("Locations: rendering template", "error", err)
	}
}

// LocationNew shows the form to create a new location.
func (h *AdminHandler) LocationNew(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	instances, err := h.instanceService.FindInstanceIDsForUser(r.Context(), user.ID)
	if err != nil {
		h.handleError(w, r, "LocationNew: getting instances", "Error getting instances", "error", err)
		return
	}

	duplicatable, err := h.markerService.FindMarkersNotInInstance(r.Context(), user.CurrentInstanceID, instances)
	if err != nil {
		h.handleError(w, r, "LocationNew: getting markers", "Error getting markers", "error", err, "instance_id", user.CurrentInstanceID)
		return
	}

	c := templates.AddLocation(user.CurrentInstance.Settings, user.CurrentInstance.Locations, duplicatable)
	err = templates.Layout(c, *user, "Locations", "New Location").Render(r.Context(), w)
	if err != nil {
		h.logger.Error("LocationNew: rendering template", "error", err)
	}
}

// LocationNewPost handles creating a new location.
func (h *AdminHandler) LocationNewPost(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "LocationNewPost: parsing form", "Error parsing form", "error", err, "instance_id", user.CurrentInstanceID)
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
			h.handleError(w, r, "LocationNewPost: converting latitude", "Error converting latitude", "error", err, "instance_id", user.CurrentInstanceID)
			return
		}
		lng, err = strconv.ParseFloat(r.FormValue("longitude"), 64)
		if err != nil {
			h.handleError(w, r, "LocationNewPost: converting longitude", "Error converting longitude", "error", err, "instance_id", user.CurrentInstanceID)
			return
		}
	}

	points := 0
	if user.CurrentInstance.Settings.EnablePoints && r.FormValue("points") != "" {
		points, err = strconv.Atoi(r.FormValue("points"))
		if err != nil {
			h.handleError(w, r, "LocationNewPost: converting points", "Error converting points", "error", err, "instance_id", user.CurrentInstanceID)
			return
		}
	}

	marker := r.FormValue("marker")
	var location models.Location
	if marker == "" {
		location, err = h.locationService.CreateLocation(r.Context(), user.CurrentInstanceID, r.FormValue("name"), lat, lng, points)
		if err != nil {
			h.handleError(w, r, "LocationNewPost: creating location without marker", "Error creating location without marker", "error", err, "instance_id", user.CurrentInstanceID)
			return
		}
	} else {
		access, err := h.accessService.CanAdminAccessMarker(r.Context(), user.ID, marker)
		if err != nil {
			h.handleError(w, r, "LocationNewPost: checking marker access", "Error checking marker access", "error", err, "instance_id", user.CurrentInstanceID)
			return
		}
		if !access {
			h.handleError(w, r, "LocationNewPost: no access to marker", "You do not have access to this marker")
			return
		}
		location, err = h.locationService.CreateLocationFromMarker(r.Context(), user.CurrentInstanceID, r.FormValue("name"), points, marker)
		if err != nil {
			h.handleError(w, r, "LocationNewPost: creating location from marker", "Error creating location from marker", "error", err, "instance_id", user.CurrentInstanceID)
			return
		}
	}

	h.redirect(w, r, "/admin/locations/"+location.MarkerID)
}

// ReorderLocations handles reordering locations.
// Returns a 200 status code if successful,
// Otherwise, returns a 500 status code.
func (h *AdminHandler) ReorderLocations(w http.ResponseWriter, r *http.Request) {
	// Check HTMX headers
	if r.Header.Get("HX-Request") != "true" {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	user := h.UserFromContext(r.Context())

	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "ReorderLocations: parsing form", "Error parsing form", "error", err, "instance_id", user.CurrentInstanceID)
		return
	}

	locations := r.Form["location"]
	err = h.locationService.ReorderLocations(r.Context(), user.CurrentInstanceID, locations)
	if err != nil {
		h.handleError(w, r, "ReorderLocations: reordering locations", "Error reordering locations", "error", err, "instance_id", user.CurrentInstanceID)
		return
	}

	h.handleSuccess(w, r, "Order updated")
}

// LocationEdit shows the form to edit a location.
func (h *AdminHandler) LocationEdit(w http.ResponseWriter, r *http.Request) {
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
		h.logger.Error("LocationEdit: finding location", "error", err, "instance_id", user.CurrentInstanceID, "location_code", code)
		h.redirect(w, r, "/admin/locations")
		return
	}

	blocks, err := h.blockService.FindByLocationID(r.Context(), location.ID)
	if err != nil {
		h.logger.Error("LocationEdit: getting blocks", "error", err, "instance_id", user.CurrentInstanceID, "location_id", location.ID)
		h.redirect(w, r, "/admin/locations")
		return
	}

	err = h.locationService.LoadCluesForLocation(r.Context(), location)
	if err != nil {
		h.handleError(w, r, "LocationEdit: loading clues", "Error loading clues", "error", err, "instance_id", user.CurrentInstanceID, "location_id", location.ID)
		return
	}

	c := templates.EditLocation(*location, user.CurrentInstance.Settings, blocks)
	err = templates.Layout(c, *user, "Locations", "Edit Location").Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "LocationEdit: rendering template", "Error rendering template", "error", err)
	}
}

// LocationEditPost handles updating a location.
func (h *AdminHandler) LocationEditPost(w http.ResponseWriter, r *http.Request) {
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

	if user.CurrentInstance.Settings.NavigationDisplayMode == models.NavigationDisplayClues {
		// Fetch the form clues
		clues := []string{}
		for key, value := range r.Form {
			if key == "clues" {
				clues = value
			}
		}
		clueIDs := []string{}
		for key, value := range r.Form {
			if key == "clue-ids" {
				clueIDs = value
			}
		}

		err = h.clueService.UpdateClues(r.Context(), location, clues, clueIDs)
		if err != nil {
			h.handleError(w, r, "LocationEdit: updating clues", "Error updating clues", "error", err, "instance_id", user.CurrentInstanceID, "location_id", location.ID)
			return
		}
	}

	if markerID != location.MarkerID {
		h.redirect(w, r, "/admin/locations/"+location.MarkerID)
		return
	}

	h.handleSuccess(w, r, "Location updated")
}

// LocationDelete handles deleting a location.
func (h *AdminHandler) LocationDelete(w http.ResponseWriter, r *http.Request) {
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
