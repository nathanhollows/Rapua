package services_test

import (
	"context"
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupSpaceService(t *testing.T) (*services.SpaceService, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	return services.NewSpaceService(repositories.NewSpaceRepository(dbc)), dbc, cleanup
}

func pointInput(name string) services.SpaceInput {
	return services.SpaceInput{
		Name:     name,
		Kind:     game.SpaceKindPoint,
		Geometry: game.NewPointGeometry(-45.8788, 170.5028, 20),
	}
}

func closedRing() game.Ring {
	return game.Ring{
		game.NewPosition(-45.87, 170.50),
		game.NewPosition(-45.88, 170.51),
		game.NewPosition(-45.89, 170.49),
		game.NewPosition(-45.87, 170.50),
	}
}

func polygonGeometry(ring game.Ring) *game.SpaceGeometry {
	return &game.SpaceGeometry{Type: game.GeometryPolygon, Rings: []game.Ring{ring}}
}

// --- ValidateSpaceInput ---

func TestValidateSpaceInput(t *testing.T) {
	cases := []struct {
		name    string
		in      services.SpaceInput
		wantErr error
	}{
		{
			name: "point with coordinates",
			in:   pointInput("Totara"),
		},
		{
			name:    "point without coordinates",
			in:      services.SpaceInput{Name: "Totara", Kind: game.SpaceKindPoint},
			wantErr: services.ErrPointNeedsCoords,
		},
		{
			name: "area with boundary",
			in: services.SpaceInput{
				Name: "Aviary", Kind: game.SpaceKindArea,
				Geometry: polygonGeometry(closedRing()),
			},
		},
		{
			name: "area with centre and radius",
			in: services.SpaceInput{
				Name: "Aviary", Kind: game.SpaceKindArea,
				Geometry: game.NewPointGeometry(-45.87, 170.5, 50),
			},
		},
		{
			name: "area with centre but no radius",
			in: services.SpaceInput{
				Name: "Aviary", Kind: game.SpaceKindArea,
				Geometry: game.NewPointGeometry(-45.87, 170.5, 0),
			},
			wantErr: services.ErrAreaNeedsExtent,
		},
		{
			name: "area with no geometry at all",
			in: services.SpaceInput{
				Name: "Aviary", Kind: game.SpaceKindArea,
			},
			wantErr: services.ErrAreaNeedsExtent,
		},
		{
			name: "area with an unclosed boundary",
			in: services.SpaceInput{
				Name: "Aviary", Kind: game.SpaceKindArea,
				Geometry: polygonGeometry(game.Ring{
					game.NewPosition(-45.87, 170.50),
					game.NewPosition(-45.88, 170.51),
					game.NewPosition(-45.89, 170.49),
					game.NewPosition(-45.86, 170.52),
				}),
			},
			wantErr: game.ErrRingNotClosed,
		},
		{
			name: "point given a boundary",
			in: services.SpaceInput{
				Name: "Totara", Kind: game.SpaceKindPoint,
				Geometry: polygonGeometry(closedRing()),
			},
			wantErr: services.ErrPointNotAPolygon,
		},
		{
			name: "object without geometry",
			in:   services.SpaceInput{Name: "Specimen box", Kind: game.SpaceKindObject},
		},
		{
			name: "object with coordinates",
			in: services.SpaceInput{
				Name: "Specimen box", Kind: game.SpaceKindObject,
				Geometry: game.NewPointGeometry(-45.87, 170.5, 0),
			},
			wantErr: services.ErrObjectHasNoGeom,
		},
		{
			name:    "blank name",
			in:      services.SpaceInput{Name: "   ", Kind: game.SpaceKindPoint},
			wantErr: services.ErrSpaceNameRequired,
		},
		{
			name:    "unknown kind",
			in:      services.SpaceInput{Name: "Totara", Kind: game.SpaceKind("venue")},
			wantErr: services.ErrInvalidSpaceKind,
		},
		{
			name: "latitude out of range",
			in: services.SpaceInput{
				Name: "Totara", Kind: game.SpaceKindPoint,
				Geometry: game.NewPointGeometry(91, 170.5, 0),
			},
			wantErr: game.ErrGeometryLatitude,
		},
		{
			name: "longitude out of range",
			in: services.SpaceInput{
				Name: "Totara", Kind: game.SpaceKindPoint,
				Geometry: game.NewPointGeometry(-45.87, 181, 0),
			},
			wantErr: game.ErrGeometryLongitude,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := services.ValidateSpaceInput(c.in)
			if c.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, c.wantErr)
		})
	}
}

// --- CreateSpace ---

func TestSpaceService_CreateSpace(t *testing.T) {
	service, dbc, cleanup := setupSpaceService(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	space, err := service.CreateSpace(context.Background(), parents.QuestID, pointInput("Totara Grove"))
	require.NoError(t, err)

	assert.NotEmpty(t, space.ID)
	assert.Equal(t, "Totara Grove", space.Name)
	assert.Equal(t, "totara-grove", space.Slug, "slug should derive from the name")
	assert.NotEmpty(t, space.Payload, "a scan payload should be generated")
	assert.Equal(t, space.Payload, strings.ToUpper(space.Payload), "payloads are stored uppercased")
}

func TestSpaceService_CreateSpaceRejectsBlankQuest(t *testing.T) {
	service, _, cleanup := setupSpaceService(t)
	defer cleanup()

	_, err := service.CreateSpace(context.Background(), "", pointInput("Totara"))
	assert.Error(t, err)
}

func TestSpaceService_CreateSpaceValidates(t *testing.T) {
	service, dbc, cleanup := setupSpaceService(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	_, err := service.CreateSpace(context.Background(), parents.QuestID, services.SpaceInput{
		Name: "Totara", Kind: game.SpaceKindPoint,
	})
	require.ErrorIs(t, err, services.ErrPointNeedsCoords)
}

// Two authors naming spaces the same thing must not collide.
func TestSpaceService_CreateSpaceSuffixesDerivedSlug(t *testing.T) {
	service, dbc, cleanup := setupSpaceService(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	first, err := service.CreateSpace(ctx, parents.QuestID, pointInput("Totara Grove"))
	require.NoError(t, err)
	second, err := service.CreateSpace(ctx, parents.QuestID, pointInput("Totara Grove"))
	require.NoError(t, err)
	third, err := service.CreateSpace(ctx, parents.QuestID, pointInput("Totara Grove"))
	require.NoError(t, err)

	assert.Equal(t, "totara-grove", first.Slug)
	assert.Equal(t, "totara-grove-2", second.Slug)
	assert.Equal(t, "totara-grove-3", third.Slug)
}

// A typed slug is a deliberate choice, so a clash is an error rather than a
// silent rename the author would not notice.
func TestSpaceService_CreateSpaceRejectsExplicitSlugClash(t *testing.T) {
	service, dbc, cleanup := setupSpaceService(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	in := pointInput("Totara Grove")
	in.Slug = "the-grove"
	_, err := service.CreateSpace(ctx, parents.QuestID, in)
	require.NoError(t, err)

	other := pointInput("Somewhere Else")
	other.Slug = "the-grove"
	_, err = service.CreateSpace(ctx, parents.QuestID, other)
	require.ErrorIs(t, err, services.ErrSlugTaken)
}

func TestSpaceService_CreateSpaceSlugsScopeToQuest(t *testing.T) {
	service, dbc, cleanup := setupSpaceService(t)
	defer cleanup()

	ctx := context.Background()
	mine := createTestParents(t, dbc)
	theirs := createTestParents(t, dbc)

	first, err := service.CreateSpace(ctx, mine.QuestID, pointInput("Totara Grove"))
	require.NoError(t, err)
	second, err := service.CreateSpace(ctx, theirs.QuestID, pointInput("Totara Grove"))
	require.NoError(t, err)

	assert.Equal(t, first.Slug, second.Slug, "the same slug is free in a different quest")
}

func TestSpaceService_CreateSpaceKeepsSuppliedPayload(t *testing.T) {
	service, dbc, cleanup := setupSpaceService(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	in := pointInput("Totara Grove")
	in.Payload = " kept1 "

	space, err := service.CreateSpace(context.Background(), parents.QuestID, in)
	require.NoError(t, err)
	assert.Equal(t, "KEPT1", space.Payload)
}

func TestSpaceService_CreateObjectDropsGeometry(t *testing.T) {
	service, dbc, cleanup := setupSpaceService(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	space, err := service.CreateSpace(context.Background(), parents.QuestID, services.SpaceInput{
		Name:   "Specimen box",
		Kind:   game.SpaceKindObject,
		Mobile: true,
	})
	require.NoError(t, err)
	assert.True(t, space.Geometry.IsZero())
	assert.True(t, space.Mobile)
}

// --- UpdateSpace ---

func TestSpaceService_UpdateSpace(t *testing.T) {
	service, dbc, cleanup := setupSpaceService(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	space, err := service.CreateSpace(ctx, parents.QuestID, pointInput("Totara Grove"))
	require.NoError(t, err)

	in := pointInput("Kauri Grove")
	in.Geometry.Radius = 75
	updated, err := service.UpdateSpace(ctx, space.ID, in)
	require.NoError(t, err)

	assert.Equal(t, "Kauri Grove", updated.Name)
	assert.InDelta(t, 75, updated.Geometry.Radius, 0.001)
}

// Re-typing a point as an object must not leave the old coordinates behind.
func TestSpaceService_UpdateClearsGeometryWhenKindChanges(t *testing.T) {
	service, dbc, cleanup := setupSpaceService(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	space, err := service.CreateSpace(ctx, parents.QuestID, pointInput("Totara Grove"))
	require.NoError(t, err)
	require.True(t, space.HasCoordinates())

	updated, err := service.UpdateSpace(ctx, space.ID, services.SpaceInput{
		Name: "Totara Grove", Kind: game.SpaceKindObject,
	})
	require.NoError(t, err)
	assert.True(t, updated.Geometry.IsZero())

	stored, err := service.GetSpaceByID(ctx, space.ID)
	require.NoError(t, err)
	assert.True(t, stored.Geometry.IsZero(), "cleared geometry should persist")
}

// A space keeping its own slug is not a clash with itself.
func TestSpaceService_UpdateKeepsOwnSlug(t *testing.T) {
	service, dbc, cleanup := setupSpaceService(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	in := pointInput("Totara Grove")
	in.Slug = "the-grove"
	space, err := service.CreateSpace(ctx, parents.QuestID, in)
	require.NoError(t, err)

	in.Name = "The Grove, renamed"
	updated, err := service.UpdateSpace(ctx, space.ID, in)
	require.NoError(t, err)
	assert.Equal(t, "the-grove", updated.Slug)
}

func TestSpaceService_UpdateRejectsSlugTakenByAnother(t *testing.T) {
	service, dbc, cleanup := setupSpaceService(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	taken := pointInput("Aviary")
	taken.Slug = "aviary"
	_, err := service.CreateSpace(ctx, parents.QuestID, taken)
	require.NoError(t, err)

	other, err := service.CreateSpace(ctx, parents.QuestID, pointInput("Totara Grove"))
	require.NoError(t, err)

	clashing := pointInput("Totara Grove")
	clashing.Slug = "aviary"
	_, err = service.UpdateSpace(ctx, other.ID, clashing)
	require.ErrorIs(t, err, services.ErrSlugTaken)
}

func TestSpaceService_UpdateValidates(t *testing.T) {
	service, dbc, cleanup := setupSpaceService(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	space, err := service.CreateSpace(ctx, parents.QuestID, pointInput("Totara Grove"))
	require.NoError(t, err)

	_, err = service.UpdateSpace(ctx, space.ID, services.SpaceInput{
		Name: "Totara Grove", Kind: game.SpaceKindPoint,
	})
	require.ErrorIs(t, err, services.ErrPointNeedsCoords)
}

func TestSpaceService_UpdateMissingSpace(t *testing.T) {
	service, _, cleanup := setupSpaceService(t)
	defer cleanup()

	_, err := service.UpdateSpace(context.Background(), "no-such-id", pointInput("Totara"))
	assert.Error(t, err)
}

// --- Read and delete ---

func TestSpaceService_FindSpacesByQuest(t *testing.T) {
	service, dbc, cleanup := setupSpaceService(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	_, err := service.CreateSpace(ctx, parents.QuestID, pointInput("Totara Grove"))
	require.NoError(t, err)
	_, err = service.CreateSpace(ctx, parents.QuestID, pointInput("Aviary"))
	require.NoError(t, err)

	spaces, err := service.FindSpacesByQuest(ctx, parents.QuestID)
	require.NoError(t, err)
	require.Len(t, spaces, 2)
	assert.Equal(t, "Aviary", spaces[0].Name, "results are ordered by name")
}

func TestSpaceService_FindSpacesByQuestRejectsBlankQuest(t *testing.T) {
	service, _, cleanup := setupSpaceService(t)
	defer cleanup()

	_, err := service.FindSpacesByQuest(context.Background(), "")
	assert.Error(t, err)
}

func TestSpaceService_DeleteSpace(t *testing.T) {
	service, dbc, cleanup := setupSpaceService(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	space, err := service.CreateSpace(ctx, parents.QuestID, pointInput("Totara Grove"))
	require.NoError(t, err)

	require.NoError(t, service.DeleteSpace(ctx, space.ID))
	_, err = service.GetSpaceByID(ctx, space.ID)
	assert.Error(t, err)
}

func TestSpaceService_DeleteSpaceRejectsBlankID(t *testing.T) {
	service, _, cleanup := setupSpaceService(t)
	defer cleanup()

	assert.Error(t, service.DeleteSpace(context.Background(), ""))
}

// --- Payload handling ---

// An author-supplied code clears the same collision check a generated one does.
func TestSpaceService_CreateRejectsPayloadTakenByAnother(t *testing.T) {
	service, dbc, cleanup := setupSpaceService(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	first := pointInput("Totara Grove")
	first.Payload = "TAKEN"
	_, err := service.CreateSpace(ctx, parents.QuestID, first)
	require.NoError(t, err)

	second := pointInput("Aviary")
	second.Payload = "taken"
	_, err = service.CreateSpace(ctx, parents.QuestID, second)
	require.ErrorIs(t, err, services.ErrPayloadTaken, "case should not smuggle a duplicate through")
}

// A scan does not know which quest it belongs to, so the clash holds across quests.
func TestSpaceService_PayloadUniqueAcrossQuests(t *testing.T) {
	service, dbc, cleanup := setupSpaceService(t)
	defer cleanup()

	ctx := context.Background()
	mine := createTestParents(t, dbc)
	theirs := createTestParents(t, dbc)

	first := pointInput("Totara Grove")
	first.Payload = "GLOBL"
	_, err := service.CreateSpace(ctx, mine.QuestID, first)
	require.NoError(t, err)

	second := pointInput("Aviary")
	second.Payload = "GLOBL"
	_, err = service.CreateSpace(ctx, theirs.QuestID, second)
	require.ErrorIs(t, err, services.ErrPayloadTaken)
}

// Clearing the box must not reissue the code: every QR already printed and every
// NFC tag already programmed for this space would go dead.
func TestSpaceService_UpdateKeepsPayloadWhenBlank(t *testing.T) {
	service, dbc, cleanup := setupSpaceService(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	space, err := service.CreateSpace(ctx, parents.QuestID, pointInput("Totara Grove"))
	require.NoError(t, err)
	original := space.Payload
	require.NotEmpty(t, original)

	in := pointInput("Totara Grove")
	in.Payload = ""
	updated, err := service.UpdateSpace(ctx, space.ID, in)
	require.NoError(t, err)
	assert.Equal(t, original, updated.Payload)

	stored, err := service.GetSpaceByID(ctx, space.ID)
	require.NoError(t, err)
	assert.Equal(t, original, stored.Payload)
}

// Keeping your own payload is not a clash with yourself.
func TestSpaceService_UpdateKeepsOwnPayload(t *testing.T) {
	service, dbc, cleanup := setupSpaceService(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	in := pointInput("Totara Grove")
	in.Payload = "OWNED"
	space, err := service.CreateSpace(ctx, parents.QuestID, in)
	require.NoError(t, err)

	in.Name = "Totara Grove, renamed"
	updated, err := service.UpdateSpace(ctx, space.ID, in)
	require.NoError(t, err)
	assert.Equal(t, "OWNED", updated.Payload)
}

func TestSpaceService_UpdateRejectsPayloadTakenByAnother(t *testing.T) {
	service, dbc, cleanup := setupSpaceService(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	taken := pointInput("Aviary")
	taken.Payload = "CLASH"
	_, err := service.CreateSpace(ctx, parents.QuestID, taken)
	require.NoError(t, err)

	other, err := service.CreateSpace(ctx, parents.QuestID, pointInput("Totara Grove"))
	require.NoError(t, err)

	clashing := pointInput("Totara Grove")
	clashing.Payload = "CLASH"
	_, err = service.UpdateSpace(ctx, other.ID, clashing)
	require.ErrorIs(t, err, services.ErrPayloadTaken)
}

// --- Timestamps ---

// updated_at has a create-time default only, so the repository has to stamp it
// or every space keeps its birth timestamp forever.
func TestSpaceService_UpdateMovesUpdatedAt(t *testing.T) {
	service, dbc, cleanup := setupSpaceService(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	space, err := service.CreateSpace(ctx, parents.QuestID, pointInput("Totara Grove"))
	require.NoError(t, err)

	// Read it back: the insert-time stamp comes from the column default, so the
	// in-memory struct still holds the zero value and would pass trivially.
	created, err := service.GetSpaceByID(ctx, space.ID)
	require.NoError(t, err)
	createdStamp := created.UpdatedAt
	require.False(t, createdStamp.IsZero(), "insert should have stamped updated_at")

	in := pointInput("Kauri Grove")
	_, err = service.UpdateSpace(ctx, space.ID, in)
	require.NoError(t, err)

	stored, err := service.GetSpaceByID(ctx, space.ID)
	require.NoError(t, err)
	assert.True(t, stored.UpdatedAt.After(createdStamp),
		"updated_at should move: %v is not after %v", stored.UpdatedAt, createdStamp)
}

// --- Geometry ---

func TestSpaceService_AreaBoundaryRoundTrip(t *testing.T) {
	service, dbc, cleanup := setupSpaceService(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	space, err := service.CreateSpace(ctx, parents.QuestID, services.SpaceInput{
		Name:     "Aviary",
		Kind:     game.SpaceKindArea,
		Geometry: polygonGeometry(closedRing()),
	})
	require.NoError(t, err)

	stored, err := service.GetSpaceByID(ctx, space.ID)
	require.NoError(t, err)
	require.Equal(t, game.GeometryPolygon, stored.Geometry.Type)
	assert.Equal(t, closedRing(), stored.Geometry.Rings[0])
}
