package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v8/models"
)

// These reach the author verbatim, so they say what to fix.
var (
	errCoordinatePair   = errors.New("give both a latitude and a longitude, or leave both blank")
	errCoordinateNumber = errors.New("latitude and longitude must be numbers")
	errRadiusNumber     = errors.New("radius must be a number")
	errBoundaryJSON     = errors.New("the boundary must be a GeoJSON geometry, as exported by a map tool")
)

func (h *Handler) Spaces(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	spaces, err := h.spaceService.FindSpacesByQuest(r.Context(), user.CurrentQuestID)
	if err != nil {
		h.handleError(w, r, "Spaces: finding spaces", "Error loading spaces", "error", err)
		return
	}

	c := templates.SpacesPage(spaces)
	if err := templates.Layout(c, *user, "Spaces", "Spaces").Render(r.Context(), w); err != nil {
		h.logger.ErrorContext(r.Context(), "Spaces: rendering template", "error", err)
	}
}

func (h *Handler) SpaceNew(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	c := templates.EditSpace(models.Space{Kind: game.SpaceKindPoint}, true)
	if err := templates.Layout(c, *user, "Spaces", "New space").Render(r.Context(), w); err != nil {
		h.logger.ErrorContext(r.Context(), "SpaceNew: rendering template", "error", err)
	}
}

func (h *Handler) SpaceCreate(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	in, err := parseSpaceForm(r)
	if err != nil {
		h.handleError(w, r, "SpaceCreate: parsing form", "Error reading form", "error", err)
		return
	}

	space, err := h.spaceService.CreateSpace(r.Context(), user.CurrentQuestID, in)
	if err != nil {
		h.handleError(w, r, "SpaceCreate: creating space", spaceErrorMessage(err), "error", err)
		return
	}

	h.redirect(w, r, "/admin/spaces/"+space.ID)
}

func (h *Handler) SpaceEdit(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	spaceID := chi.URLParam(r, "id")

	space, err := h.spaceService.GetSpaceByID(r.Context(), spaceID)
	if err != nil {
		h.handleError(w, r, "SpaceEdit: finding space", "Space not found", "error", err)
		return
	}
	if space.QuestID != user.CurrentQuestID {
		h.handleError(w, r, "SpaceEdit: space belongs to another quest", "Space not found",
			"space_id", spaceID, "quest_id", user.CurrentQuestID)
		return
	}

	c := templates.EditSpace(space, false)
	if err := templates.Layout(c, *user, "Spaces", space.Name).Render(r.Context(), w); err != nil {
		h.logger.ErrorContext(r.Context(), "SpaceEdit: rendering template", "error", err)
	}
}

func (h *Handler) SpaceEditPost(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	spaceID := chi.URLParam(r, "id")

	space, err := h.spaceService.GetSpaceByID(r.Context(), spaceID)
	if err != nil {
		h.handleError(w, r, "SpaceEditPost: finding space", "Space not found", "error", err)
		return
	}
	if space.QuestID != user.CurrentQuestID {
		h.handleError(w, r, "SpaceEditPost: space belongs to another quest", "Space not found",
			"space_id", spaceID, "quest_id", user.CurrentQuestID)
		return
	}

	in, err := parseSpaceForm(r)
	if err != nil {
		h.handleError(w, r, "SpaceEditPost: parsing form", "Error reading form", "error", err)
		return
	}

	if _, err := h.spaceService.UpdateSpace(r.Context(), spaceID, in); err != nil {
		h.handleError(w, r, "SpaceEditPost: updating space", spaceErrorMessage(err), "error", err)
		return
	}

	h.handleSuccess(w, r, "Space saved")
}

func (h *Handler) SpaceDelete(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	spaceID := chi.URLParam(r, "id")

	space, err := h.spaceService.GetSpaceByID(r.Context(), spaceID)
	if err != nil {
		h.handleError(w, r, "SpaceDelete: finding space", "Space not found", "error", err)
		return
	}
	if space.QuestID != user.CurrentQuestID {
		h.handleError(w, r, "SpaceDelete: space belongs to another quest", "Space not found",
			"space_id", spaceID, "quest_id", user.CurrentQuestID)
		return
	}

	if err := h.spaceService.DeleteSpace(r.Context(), spaceID); err != nil {
		h.handleError(w, r, "SpaceDelete: deleting space", "Error deleting space", "error", err)
		return
	}

	h.redirect(w, r, "/admin/spaces")
}

// Geometry fields the chosen kind does not use are ignored here, so switching
// kind in the form does not carry stale coordinates through.
func parseSpaceForm(r *http.Request) (services.SpaceInput, error) {
	if err := r.ParseForm(); err != nil {
		return services.SpaceInput{}, err
	}

	kind := game.SpaceKind(strings.TrimSpace(r.FormValue("kind")))
	in := services.SpaceInput{
		Name:    r.FormValue("name"),
		Slug:    strings.TrimSpace(r.FormValue("slug")),
		Kind:    kind,
		Payload: r.FormValue("payload"),
		Mobile:  r.FormValue("mobile") != "",
	}

	if kind == game.SpaceKindObject {
		return in, nil
	}

	// An author who drew a boundary meant the shape, not the pin.
	if kind == game.SpaceKindArea {
		boundary, err := parseAreaBoundary(r)
		if err != nil {
			return services.SpaceInput{}, err
		}
		if boundary != nil {
			in.Geometry = boundary
			return in, nil
		}
	}

	geometry, err := parseCentre(r.FormValue("lat"), r.FormValue("lng"), r.FormValue("radius"))
	if err != nil {
		return services.SpaceInput{}, err
	}
	in.Geometry = geometry
	return in, nil
}

// Returns nil for an empty box, so the caller falls back to the centre boxes.
func parseAreaBoundary(r *http.Request) (*game.SpaceGeometry, error) {
	boundary, err := parseBoundary(r.FormValue("boundary"))
	if err != nil || boundary == nil {
		return nil, err
	}

	// A map tool exporting a marker gives a Point. Pasting one is a centre, so
	// the radius box still applies — dropping it would reject an area whose
	// boxes both look filled in.
	if boundary.Type == game.GeometryPoint && boundary.Radius == 0 {
		radius, radiusErr := parseOptionalFloat(r.FormValue("radius"), errRadiusNumber)
		if radiusErr != nil {
			return nil, radiusErr
		}
		boundary.Radius = radius
	}
	return boundary, nil
}

// A half-filled pair is rejected rather than defaulted: a missing longitude read
// as zero would put the space in the Gulf of Guinea without complaint.
func parseCentre(latStr, lngStr, radiusStr string) (*game.SpaceGeometry, error) {
	latStr, lngStr = strings.TrimSpace(latStr), strings.TrimSpace(lngStr)

	radius, err := parseOptionalFloat(radiusStr, errRadiusNumber)
	if err != nil {
		return nil, err
	}

	if latStr == "" && lngStr == "" {
		if radius == 0 {
			return nil, nil //nolint:nilnil // no geometry given is not a failure; the service decides if the kind needs one
		}
		return nil, errCoordinatePair
	}
	if latStr == "" || lngStr == "" {
		return nil, errCoordinatePair
	}

	lat, err := parseOptionalFloat(latStr, errCoordinateNumber)
	if err != nil {
		return nil, err
	}
	lng, err := parseOptionalFloat(lngStr, errCoordinateNumber)
	if err != nil {
		return nil, err
	}
	return game.NewPointGeometry(lat, lng, radius), nil
}

// Returns nil when the box is empty.
func parseBoundary(raw string) (*game.SpaceGeometry, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil //nolint:nilnil // an empty box is not a failure; the caller falls back to the centre boxes
	}

	var geometry game.SpaceGeometry
	if err := json.Unmarshal([]byte(raw), &geometry); err != nil {
		return nil, errBoundaryJSON
	}
	if geometry.IsZero() {
		return nil, errBoundaryJSON
	}
	return &geometry, nil
}

func parseOptionalFloat(s string, onErr error) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, onErr
	}
	return f, nil
}

// Validation failures reach the author verbatim; anything else stays generic so
// internals do not leak.
func spaceErrorMessage(err error) string {
	for _, known := range []error{
		services.ErrSpaceNameRequired,
		services.ErrInvalidSpaceKind,
		services.ErrPointNeedsCoords,
		services.ErrPointNotAPolygon,
		services.ErrAreaNeedsExtent,
		services.ErrObjectHasNoGeom,
		services.ErrSlugTaken,
		services.ErrPayloadTaken,
		game.ErrGeometryType,
		game.ErrGeometryLatitude,
		game.ErrGeometryLongitude,
		game.ErrNegativeRadius,
		game.ErrEmptyPolygon,
		game.ErrRingTooShort,
		game.ErrRingNotClosed,
		errCoordinatePair,
		errCoordinateNumber,
		errRadiusNumber,
		errBoundaryJSON,
	} {
		if errors.Is(err, known) {
			return err.Error()
		}
	}
	return "Error saving space"
}
