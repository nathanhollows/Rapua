package services_test

import (
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v7/internal/services"
	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/stretchr/testify/assert"
)

func makeTeam(points int, checkIns []models.CheckIn) *models.Team {
	return &models.Team{
		Points:   points,
		CheckIns: checkIns,
	}
}

func makeCheckIn(slug, locationID string, blocksCompleted bool) models.CheckIn {
	return models.CheckIn{
		LocationID:      locationID,
		Location:        models.Location{Slug: slug},
		BlocksCompleted: blocksCompleted,
		TimeIn:          time.Now(),
	}
}

func TestCompositeResolver_Points(t *testing.T) {
	r := services.NewPlayerVarResolver(makeTeam(42, nil), nil)
	val, ok := r.ResolveVar("points")
	assert.True(t, ok)
	assert.Equal(t, "42", val)
}

func TestCompositeResolver_LocationVisited(t *testing.T) {
	team := makeTeam(0, []models.CheckIn{
		makeCheckIn("the-dell", "loc-1", false),
	})
	r := services.NewPlayerVarResolver(team, nil)

	val, ok := r.ResolveVar("location.the-dell.visited")
	assert.True(t, ok)
	assert.Equal(t, "true", val)

	val, ok = r.ResolveVar("location.citrus-lab.visited")
	assert.True(t, ok, "unvisited location should still return ok=true")
	assert.Equal(t, "false", val)
}

func TestCompositeResolver_LocationCheckedIn(t *testing.T) {
	team := makeTeam(0, []models.CheckIn{
		makeCheckIn("the-dell", "loc-1", true),
		makeCheckIn("citrus-lab", "loc-2", false),
	})
	r := services.NewPlayerVarResolver(team, nil)

	val, ok := r.ResolveVar("location.the-dell.checked_in")
	assert.True(t, ok)
	assert.Equal(t, "true", val)

	val, ok = r.ResolveVar("location.citrus-lab.checked_in")
	assert.True(t, ok)
	assert.Equal(t, "false", val)

	val, ok = r.ResolveVar("location.nowhere.checked_in")
	assert.True(t, ok)
	assert.Equal(t, "false", val)
}

func TestCompositeResolver_GroupCompleted(t *testing.T) {
	gs := models.GameStructure{
		ID: "root",
		SubGroups: []models.GameStructure{
			{
				ID:             "grp-1",
				Name:           "Top Notes",
				CompletionType: models.CompletionAll,
				LocationIDs:    []string{"loc-a", "loc-b"},
			},
		},
	}
	team := &models.Team{
		Instance: models.Instance{GameStructure: gs},
		CheckIns: []models.CheckIn{
			{LocationID: "loc-a", BlocksCompleted: true},
			{LocationID: "loc-b", BlocksCompleted: true},
		},
	}
	r := services.NewPlayerVarResolver(team, nil)

	val, ok := r.ResolveVar("group.Top Notes.completed")
	assert.True(t, ok)
	assert.Equal(t, "true", val)

	// Incomplete group
	team2 := &models.Team{
		Instance: models.Instance{GameStructure: gs},
		CheckIns: []models.CheckIn{
			{LocationID: "loc-a", BlocksCompleted: true},
		},
	}
	r2 := services.NewPlayerVarResolver(team2, nil)
	val, ok = r2.ResolveVar("group.Top Notes.completed")
	assert.True(t, ok)
	assert.Equal(t, "false", val)

	// Unknown group name
	val, ok = r.ResolveVar("group.Unknown.completed")
	assert.True(t, ok)
	assert.Equal(t, "false", val)
}

func TestCompositeResolver_CreatorVars(t *testing.T) {
	creator := map[string]string{"took_bergamot": "true"}
	r := services.NewPlayerVarResolver(makeTeam(0, nil), creator)

	val, ok := r.ResolveVar("took_bergamot")
	assert.True(t, ok)
	assert.Equal(t, "true", val)
}

func TestCompositeResolver_Priority(t *testing.T) {
	// built-in beats creator
	creator := map[string]string{"points": "999"}
	r := services.NewPlayerVarResolver(makeTeam(10, nil), creator)
	val, _ := r.ResolveVar("points")
	assert.Equal(t, "10", val, "built-in points must beat creator var named 'points'")
}

func TestCompositeResolver_UnknownVar(t *testing.T) {
	r := services.NewPlayerVarResolver(makeTeam(0, nil), nil)
	val, ok := r.ResolveVar("undefined_var")
	assert.False(t, ok)
	assert.Empty(t, val)
}

func TestCompositeResolver_NilMaps(t *testing.T) {
	r := services.NewPlayerVarResolver(makeTeam(5, nil), nil)
	// Should not panic
	val, ok := r.ResolveVar("points")
	assert.True(t, ok)
	assert.Equal(t, "5", val)

	_, ok = r.ResolveVar("missing")
	assert.False(t, ok)
}

func TestPlayerVarResolver_TeamCount(t *testing.T) {
	r := services.NewPlayerVarResolver(makeTeam(0, nil), nil)

	val, ok := r.ResolveVar("game.team_count")
	assert.True(t, ok)
	assert.Equal(t, "0", val, "default team count is 0")

	r = r.WithTeamCount(17)
	val, ok = r.ResolveVar("game.team_count")
	assert.True(t, ok)
	assert.Equal(t, "17", val)
}
