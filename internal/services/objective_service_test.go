package services_test

import (
	"context"
	"testing"

	"database/sql"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupObjectiveService(t *testing.T) (services.ObjectiveService, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)
	objectiveRepo := repositories.NewObjectiveRepository(dbc)
	objectiveService := services.NewObjectiveService(transactor, objectiveRepo)
	return objectiveService, dbc, cleanup
}

func TestObjectiveService_CreateObjective(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()

	t.Run("Create objective", func(t *testing.T) {
		objective, err := service.CreateObjective(context.Background(), validQuestID(t, dbc), "", gofakeit.Sentence(3))
		require.NoError(t, err)
		assert.NotEmpty(t, objective.ID)
	})

	t.Run("Create objective with invalid instance ID", func(t *testing.T) {
		_, err := service.CreateObjective(context.Background(), "", "", gofakeit.Sentence(3))
		require.Error(t, err)
	})

	t.Run("Create objective with invalid title", func(t *testing.T) {
		_, err := service.CreateObjective(context.Background(), validQuestID(t, dbc), "", "")
		require.Error(t, err)
	})

	t.Run("carries a routing strategy", func(t *testing.T) {
		// An empty routing played as free roam while reading as unset, so a
		// gameplay decision was being made by omission. Lint refuses it, which
		// means creation must not produce it.
		objective, err := service.CreateObjective(
			context.Background(), validQuestID(t, dbc), "", gofakeit.Sentence(3))
		require.NoError(t, err)
		assert.NotEmpty(t, objective.Routing, "a new objective must not be stored without a routing")
		_, parseErr := models.ParseRouteStrategy(string(objective.Routing))
		assert.NoError(t, parseErr, "and it must be one of the real strategies")
	})
}

// A real failure while checking slug availability must propagate, not be
// silently treated as "slug is available" the way a genuine not-found is.
func TestObjectiveService_CreateObjective_RealSlugCheckErrorPropagates(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	questID := validQuestID(t, dbc)

	cleanup() // close the DB so the slug-availability check fails with something other than sql.ErrNoRows.

	_, err := service.CreateObjective(context.Background(), questID, "", "Find the key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checking slug availability")
}

func TestObjectiveService_CreateObjective_Slug(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("slug generated from title", func(t *testing.T) {
		objective, err := service.CreateObjective(ctx, validQuestID(t, dbc), "", "Citrus Collection")
		require.NoError(t, err)
		assert.Equal(t, "citrus-collection", objective.Slug)
	})

	t.Run("duplicate title in same instance gets unique slug", func(t *testing.T) {
		questID := validQuestID(t, dbc)
		obj1, err := service.CreateObjective(ctx, questID, "", "Garden Walk")
		require.NoError(t, err)
		obj2, err := service.CreateObjective(ctx, questID, "", "Garden Walk")
		require.NoError(t, err)
		assert.NotEqual(t, obj1.Slug, obj2.Slug)
	})

	t.Run("same title in different instances can share slug", func(t *testing.T) {
		obj1, err := service.CreateObjective(ctx, validQuestID(t, dbc), "", "Garden Walk")
		require.NoError(t, err)
		obj2, err := service.CreateObjective(ctx, validQuestID(t, dbc), "", "Garden Walk")
		require.NoError(t, err)
		assert.Equal(t, obj1.Slug, obj2.Slug)
	})
}

func TestObjectiveService_GetByQuestIDAndSlug(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()
	ctx := context.Background()

	questID := validQuestID(t, dbc)
	objective, err := service.CreateObjective(ctx, questID, "", gofakeit.Sentence(3))
	require.NoError(t, err)
	require.NotEmpty(t, objective.Slug)

	t.Run("found by slug", func(t *testing.T) {
		found, findErr := service.GetByQuestIDAndSlug(ctx, questID, objective.Slug)
		require.NoError(t, findErr)
		assert.Equal(t, objective.ID, found.ID)
	})

	t.Run("not found with wrong slug", func(t *testing.T) {
		_, findErr := service.GetByQuestIDAndSlug(ctx, questID, gofakeit.Word())
		require.Error(t, findErr)
	})

	t.Run("not found in wrong instance", func(t *testing.T) {
		_, findErr := service.GetByQuestIDAndSlug(ctx, gofakeit.UUID(), objective.Slug)
		require.Error(t, findErr)
	})
}

func TestObjectiveService_UpdateObjective(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("slug updates when title changes", func(t *testing.T) {
		objective, err := service.CreateObjective(ctx, validQuestID(t, dbc), "", "Original Title")
		require.NoError(t, err)
		assert.Equal(t, "original-title", objective.Slug)

		err = service.UpdateObjective(ctx, &objective, services.ObjectiveUpdateData{Title: "New Title"})
		require.NoError(t, err)
		assert.Equal(t, "new-title", objective.Slug)
		assert.Equal(t, "New Title", objective.Title)
	})

	t.Run("slug unchanged when title not provided", func(t *testing.T) {
		objective, err := service.CreateObjective(ctx, validQuestID(t, dbc), "", "Original Title")
		require.NoError(t, err)
		originalSlug := objective.Slug

		err = service.UpdateObjective(ctx, &objective, services.ObjectiveUpdateData{})
		require.NoError(t, err)
		assert.Equal(t, originalSlug, objective.Slug)
	})

	t.Run("duplicate title in same instance gets unique slug on rename", func(t *testing.T) {
		questID := validQuestID(t, dbc)
		_, err := service.CreateObjective(ctx, questID, "", "Garden Walk")
		require.NoError(t, err)
		obj2, err := service.CreateObjective(ctx, questID, "", gofakeit.Sentence(3))
		require.NoError(t, err)

		err = service.UpdateObjective(ctx, &obj2, services.ObjectiveUpdateData{Title: "Garden Walk"})
		require.NoError(t, err)
		assert.NotEqual(t, "garden-walk", obj2.Slug) // collision resolved with suffix.
		assert.Contains(t, obj2.Slug, "garden-walk")
	})

	t.Run("update persists to storage", func(t *testing.T) {
		questID := validQuestID(t, dbc)
		objective, err := service.CreateObjective(ctx, questID, "", "Original Title")
		require.NoError(t, err)

		err = service.UpdateObjective(ctx, &objective, services.ObjectiveUpdateData{Title: "New Title"})
		require.NoError(t, err)

		reloaded, err := service.GetByQuestIDAndSlug(ctx, questID, "new-title")
		require.NoError(t, err)
		assert.Equal(t, "New Title", reloaded.Title)
	})
}

func TestObjectiveService_FindByQuestID(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()
	ctx := context.Background()

	questID := validQuestID(t, dbc)
	obj1, err := service.CreateObjective(ctx, questID, "", "Find the key")
	require.NoError(t, err)
	obj2, err := service.CreateObjective(ctx, questID, "", "Open the door")
	require.NoError(t, err)

	t.Run("returns all objectives for the quest", func(t *testing.T) {
		objectives, findErr := service.FindByQuestID(ctx, questID)
		require.NoError(t, findErr)
		require.Len(t, objectives, 2)
		ids := []string{objectives[0].ID, objectives[1].ID}
		assert.ElementsMatch(t, []string{obj1.ID, obj2.ID}, ids)
	})

	t.Run("returns empty for a quest with no objectives", func(t *testing.T) {
		objectives, findErr := service.FindByQuestID(ctx, validQuestID(t, dbc))
		require.NoError(t, findErr)
		assert.Empty(t, objectives)
	})
}

// A quest can be live while it is being built, so a new objective is parked
// until its author says otherwise. The root is the exception: it is the quest
// rather than a place in it, and drafting it takes the whole game out of play.
func TestObjectiveService_CreateObjective_Draft(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()
	ctx := context.Background()
	questID := validQuestID(t, dbc)

	root, err := service.CreateObjective(ctx, questID, "", "Root")
	require.NoError(t, err)
	assert.False(t, root.Draft, "the root is never a draft")

	child, err := service.CreateObjective(ctx, questID, root.ID, "A new objective")
	require.NoError(t, err)
	assert.True(t, child.Draft, "everything else arrives parked")
}

// Publishing and parking is the one edit that has to survive a form that never
// mentions the title, and a title change must not quietly publish anything.
func TestObjectiveService_UpdateObjective_Draft(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()
	ctx := context.Background()
	questID := validQuestID(t, dbc)

	root, err := service.CreateObjective(ctx, questID, "", "Root")
	require.NoError(t, err)
	objective, err := service.CreateObjective(ctx, questID, root.ID, "Parked")
	require.NoError(t, err)
	require.True(t, objective.Draft)

	t.Run("a form with no opinion leaves it parked", func(t *testing.T) {
		require.NoError(t, service.UpdateObjective(ctx, &objective, services.ObjectiveUpdateData{
			Title: "Renamed",
		}))
		assert.True(t, objective.Draft)
		assert.Equal(t, "Renamed", objective.Title)
	})

	t.Run("publishing", func(t *testing.T) {
		published := false
		require.NoError(t, service.UpdateObjective(ctx, &objective, services.ObjectiveUpdateData{
			Draft: &published,
		}))
		assert.False(t, objective.Draft)

		reloaded, findErr := service.GetByQuestIDAndSlug(ctx, questID, objective.Slug)
		require.NoError(t, findErr)
		assert.False(t, reloaded.Draft, "and it persisted")
	})

	t.Run("parking again", func(t *testing.T) {
		parked := true
		require.NoError(t, service.UpdateObjective(ctx, &objective, services.ObjectiveUpdateData{
			Draft: &parked,
		}))
		assert.True(t, objective.Draft)
	})
}

// Parking the root takes every objective out of play at once, and the builder
// does not list the root, so nothing on screen would explain the empty quest.
func TestObjectiveService_UpdateObjective_CannotDraftRoot(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()
	ctx := context.Background()
	questID := validQuestID(t, dbc)

	root, err := service.CreateObjective(ctx, questID, "", "Root")
	require.NoError(t, err)

	parked := true
	err = service.UpdateObjective(ctx, &root, services.ObjectiveUpdateData{Draft: &parked})
	require.ErrorIs(t, err, services.ErrCannotDraftRoot)
	assert.False(t, root.Draft)

	// Publishing it is always allowed, so a root that somehow got parked can be
	// brought back.
	root.Draft = true
	published := false
	require.NoError(t, service.UpdateObjective(ctx, &root, services.ObjectiveUpdateData{Draft: &published}))
	assert.False(t, root.Draft)
}

// A band counts published children, so parking one lowers what its section can
// ever reach. Past an explicit minimum the section can never complete and
// everything after it stays locked, which lint says about a document and the
// toggle would otherwise let through in silence.
func TestObjectiveService_UpdateObjective_ParkingCannotBreakABand(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()
	ctx := context.Background()
	questID := validQuestID(t, dbc)

	root, err := service.CreateObjective(ctx, questID, "", "Root")
	require.NoError(t, err)
	section, err := service.CreateObjective(ctx, questID, root.ID, "Section")
	require.NoError(t, err)

	first, err := service.CreateObjective(ctx, questID, section.ID, "First")
	require.NoError(t, err)
	second, err := service.CreateObjective(ctx, questID, section.ID, "Second")
	require.NoError(t, err)

	// The section needs both of them.
	minChildren := 2
	section.ChildrenMin = &minChildren
	tx, err := db.NewTransactor(dbc).BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	require.NoError(t, repositories.NewObjectiveRepository(dbc).UpdateTx(ctx, tx, &section))
	require.NoError(t, tx.Commit())

	// Both children start parked, so publish them before the band can bind.
	published := false
	require.NoError(t, service.UpdateObjective(ctx, &first, services.ObjectiveUpdateData{Draft: &published}))
	require.NoError(t, service.UpdateObjective(ctx, &second, services.ObjectiveUpdateData{Draft: &published}))

	parked := true
	err = service.UpdateObjective(ctx, &second, services.ObjectiveUpdateData{Draft: &parked})
	require.ErrorIs(t, err, services.ErrParkingBreaksBand)
	assert.False(t, second.Draft, "and it stays in play")
}

// A section with no explicit minimum needs whatever is left, so parking a child
// is always fine there.
func TestObjectiveService_UpdateObjective_ParkingUnderAnOmittedBandIsFine(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()
	ctx := context.Background()
	questID := validQuestID(t, dbc)

	root, err := service.CreateObjective(ctx, questID, "", "Root")
	require.NoError(t, err)
	section, err := service.CreateObjective(ctx, questID, root.ID, "Section")
	require.NoError(t, err)
	child, err := service.CreateObjective(ctx, questID, section.ID, "Child")
	require.NoError(t, err)

	published := false
	require.NoError(t, service.UpdateObjective(ctx, &child, services.ObjectiveUpdateData{Draft: &published}))

	parked := true
	require.NoError(t, service.UpdateObjective(ctx, &child, services.ObjectiveUpdateData{Draft: &parked}))
	assert.True(t, child.Draft)
}

func strPtr(s string) *string { return &s }

func intPtr(v int) *int { return &v }

// The tree's eye toggle posts Draft alone, so a nil field must mean
// "unchanged," or one click wipes the section's routing, band, colour, label
// and depends.
func TestObjectiveService_UpdateObjective_DraftOnlyLeavesSettingsUntouched(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()
	ctx := context.Background()

	root, err := service.CreateObjective(ctx, validQuestID(t, dbc), "", "Root")
	require.NoError(t, err)
	section, err := service.CreateObjective(ctx, root.QuestID, root.ID, "Section")
	require.NoError(t, err)
	childOne, err := service.CreateObjective(ctx, root.QuestID, section.ID, "Child one")
	require.NoError(t, err)
	childTwo, err := service.CreateObjective(ctx, root.QuestID, section.ID, "Child two")
	require.NoError(t, err)

	// New objectives arrive parked, and the band counts what is in play: a
	// minimum of one against two drafts is a minimum no run could meet.
	published := false
	require.NoError(t, service.UpdateObjective(ctx, &childOne, services.ObjectiveUpdateData{Draft: &published}))
	require.NoError(t, service.UpdateObjective(ctx, &childTwo, services.ObjectiveUpdateData{Draft: &published}))

	settings := services.ObjectiveUpdateData{
		Routing:     strPtr("ordered"),
		MaxNext:     intPtr(2),
		Band:        &services.BandUpdate{Min: intPtr(1), Max: intPtr(2)},
		FinishLabel: strPtr("Done"),
		Color:       strPtr("amber"),
		Depends:     []string{"objective." + root.Slug},
	}
	require.NoError(t, service.UpdateObjective(ctx, &section, settings))

	require.NoError(t, service.UpdateObjective(ctx, &section, services.ObjectiveUpdateData{Draft: &published}))

	reloaded, err := service.GetByQuestIDAndSlug(ctx, root.QuestID, section.Slug)
	require.NoError(t, err)
	assert.False(t, reloaded.Draft, "the toggle publishes")
	assert.Equal(t, "ordered", string(reloaded.Routing), "routing survives the toggle")
	assert.Equal(t, 2, reloaded.MaxNext)
	require.NotNil(t, reloaded.ChildrenMin)
	assert.Equal(t, 1, *reloaded.ChildrenMin)
	require.NotNil(t, reloaded.ChildrenMax)
	assert.Equal(t, 2, *reloaded.ChildrenMax)
	assert.Equal(t, "Done", reloaded.FinishLabel)
	assert.Equal(t, "amber", reloaded.Color)
	assert.Equal(t, []string{"objective." + root.Slug}, []string(reloaded.Depends))
}

// A band above the child count can never be met, so the service refuses it
// the way lint refuses a document (BAND_OUT_OF_RANGE).
func TestObjectiveService_UpdateObjective_BandBeyondChildCountIsRefused(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()
	ctx := context.Background()

	root, err := service.CreateObjective(ctx, validQuestID(t, dbc), "", "Root")
	require.NoError(t, err)
	section, err := service.CreateObjective(ctx, root.QuestID, root.ID, "Section")
	require.NoError(t, err)
	_, err = service.CreateObjective(ctx, root.QuestID, section.ID, "Only child")
	require.NoError(t, err)

	err = service.UpdateObjective(ctx, &section, services.ObjectiveUpdateData{
		Band: &services.BandUpdate{Min: intPtr(5), Max: intPtr(7)},
	})
	require.ErrorIs(t, err, services.ErrInvalidBand)

	reloaded, err := service.GetByQuestIDAndSlug(ctx, root.QuestID, section.Slug)
	require.NoError(t, err)
	assert.Nil(t, reloaded.ChildrenMin, "the refused band was not written")
	assert.Nil(t, reloaded.ChildrenMax)
}

// A depends entry naming one of the objective's own descendants is a gate
// that can never open: nothing below can complete before the objective.
func TestObjectiveService_UpdateObjective_DependsOnOwnDescendantIsRefused(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()
	ctx := context.Background()

	root, err := service.CreateObjective(ctx, validQuestID(t, dbc), "", "Root")
	require.NoError(t, err)
	section, err := service.CreateObjective(ctx, root.QuestID, root.ID, "Section")
	require.NoError(t, err)
	child, err := service.CreateObjective(ctx, root.QuestID, section.ID, "Child")
	require.NoError(t, err)

	err = service.UpdateObjective(ctx, &section, services.ObjectiveUpdateData{
		Depends: []string{"objective." + child.Slug},
	})
	require.ErrorIs(t, err, services.ErrDependsOnDescendant)

	// A negated entry is refused too: the check cannot tell a deadlock from
	// a trivially true gate, and both are mistakes.
	err = service.UpdateObjective(ctx, &section, services.ObjectiveUpdateData{
		Depends: []string{"not objective." + child.Slug},
	})
	require.ErrorIs(t, err, services.ErrDependsOnDescendant)

	// A sibling is fine.
	other, err := service.CreateObjective(ctx, root.QuestID, root.ID, "Sibling")
	require.NoError(t, err)
	require.NoError(t, service.UpdateObjective(ctx, &section, services.ObjectiveUpdateData{
		Depends: []string{"objective." + other.Slug},
	}))
}

// The editor tells authors that leaving both fields blank requires every
// child. It did not: a nil bound read as "unchanged", so a band once set could
// never be cleared and the page silently kept the old one.
func TestObjectiveService_UpdateObjective_BandCanBeCleared(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()
	ctx := context.Background()

	root, err := service.CreateObjective(ctx, validQuestID(t, dbc), "", "Root")
	require.NoError(t, err)
	section, err := service.CreateObjective(ctx, root.QuestID, root.ID, "Section")
	require.NoError(t, err)
	published := false
	for _, title := range []string{"One", "Two"} {
		child, childErr := service.CreateObjective(ctx, root.QuestID, section.ID, title)
		require.NoError(t, childErr)
		require.NoError(t, service.UpdateObjective(ctx, &child,
			services.ObjectiveUpdateData{Draft: &published}))
	}

	require.NoError(t, service.UpdateObjective(ctx, &section, services.ObjectiveUpdateData{
		Band: &services.BandUpdate{Min: intPtr(1), Max: intPtr(2)},
	}))
	stored, err := service.GetByQuestIDAndSlug(ctx, root.QuestID, section.Slug)
	require.NoError(t, err)
	require.NotNil(t, stored.ChildrenMin, "the band saved")

	// Both fields emptied, which is what the form submits when the author
	// clears them.
	require.NoError(t, service.UpdateObjective(ctx, &section, services.ObjectiveUpdateData{
		Band: &services.BandUpdate{},
	}))
	stored, err = service.GetByQuestIDAndSlug(ctx, root.QuestID, section.Slug)
	require.NoError(t, err)
	assert.Nil(t, stored.ChildrenMin, "and clearing it actually clears it")
	assert.Nil(t, stored.ChildrenMax)
}

// A refused update must leave the caller's objective as it was. The handler
// renders that objective straight back to the author, so a half-mutated one
// puts the rejected state on screen and every later autosave resubmits it.
func TestObjectiveService_RefusedUpdateLeavesTheObjectiveUntouched(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()
	ctx := context.Background()

	root, err := service.CreateObjective(ctx, validQuestID(t, dbc), "", "Root")
	require.NoError(t, err)
	section, err := service.CreateObjective(ctx, root.QuestID, root.ID, "Section")
	require.NoError(t, err)

	published := false
	require.NoError(t, service.UpdateObjective(ctx, &section,
		services.ObjectiveUpdateData{Draft: &published}))
	require.False(t, section.Draft, "published to begin with")

	// Parking is fine, but the band in the same update is not: a minimum of
	// five against no children at all.
	err = service.UpdateObjective(ctx, &section, services.ObjectiveUpdateData{
		Draft: boolPtr(true),
		Band:  &services.BandUpdate{Min: intPtr(5)},
	})
	require.Error(t, err)

	assert.False(t, section.Draft,
		"the park is rolled back with the band it was refused alongside")
	stored, err := service.GetByQuestIDAndSlug(ctx, root.QuestID, section.Slug)
	require.NoError(t, err)
	assert.False(t, stored.Draft, "and nothing reached the database")
}

// A stranded row has no parent whose band could break. Parking one errored on
// the missing parent, naming a row that is not in the quest.
func TestObjectiveService_ParkingAStrandedRowIsAllowed(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()
	ctx := context.Background()

	root, err := service.CreateObjective(ctx, validQuestID(t, dbc), "", "Root")
	require.NoError(t, err)
	stranded, err := service.CreateObjective(ctx, root.QuestID, root.ID, "Stranded")
	require.NoError(t, err)

	// Strand it: its parent is no longer a row in this quest.
	_, err = dbc.NewUpdate().Model((*models.Objective)(nil)).
		Set("parent_id = ?", gofakeit.UUID()).Where("id = ?", stranded.ID).Exec(ctx)
	require.NoError(t, err)
	reloaded, err := service.GetByQuestIDAndSlug(ctx, root.QuestID, stranded.Slug)
	require.NoError(t, err)

	assert.NoError(t, service.UpdateObjective(ctx, reloaded,
		services.ObjectiveUpdateData{Draft: boolPtr(true)}),
		"the editor draws it so it can be repaired, which includes parking it")
}

// A maximum is where a section closes on its own, not a number a run has to
// reach. With a lower minimum the player finishes by pressing the button, so
// parking a child that leaves fewer than the maximum strands nothing.
func TestObjectiveService_ParkingBelowMaxIsAllowedWhenMinIsLower(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()
	ctx := context.Background()

	root, err := service.CreateObjective(ctx, validQuestID(t, dbc), "", "Root")
	require.NoError(t, err)
	section, err := service.CreateObjective(ctx, root.QuestID, root.ID, "Section")
	require.NoError(t, err)

	published := false
	children := make([]models.Objective, 0, 3)
	for _, title := range []string{"One", "Two", "Three"} {
		child, childErr := service.CreateObjective(ctx, root.QuestID, section.ID, title)
		require.NoError(t, childErr)
		require.NoError(t, service.UpdateObjective(ctx, &child,
			services.ObjectiveUpdateData{Draft: &published}))
		children = append(children, child)
	}

	// Finish anywhere from one to three: the button appears at one.
	require.NoError(t, service.UpdateObjective(ctx, &section, services.ObjectiveUpdateData{
		Band: &services.BandUpdate{Min: intPtr(1), Max: intPtr(3)},
	}))

	// Two left is below the maximum and above the minimum, so the section can
	// still be finished.
	assert.NoError(t, service.UpdateObjective(ctx, &children[0],
		services.ObjectiveUpdateData{Draft: boolPtr(true)}))

	// One left still meets the minimum.
	assert.NoError(t, service.UpdateObjective(ctx, &children[1],
		services.ObjectiveUpdateData{Draft: boolPtr(true)}))

	// None left cannot: there is nothing to complete and no button to press.
	assert.ErrorIs(t, service.UpdateObjective(ctx, &children[2],
		services.ObjectiveUpdateData{Draft: boolPtr(true)}), services.ErrParkingBreaksBand)
}
