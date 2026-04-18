package services_test

import (
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v7/internal/services"
	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

func makeTeam(points int, checkIns []models.CheckIn) *models.Team {
	return &models.Team{
		Points:   points,
		CheckIns: checkIns,
	}
}

func makeTeamWithInstance(points int, instance models.Instance) *models.Team {
	return &models.Team{
		Points:   points,
		Instance: instance,
	}
}

func makeCheckIn(slug string, blocksCompleted bool) models.CheckIn {
	return models.CheckIn{
		Location:        models.Location{Slug: slug},
		BlocksCompleted: blocksCompleted,
		TimeIn:          time.Now(),
	}
}

func TestCompositeResolver_Points(t *testing.T) {
	r := services.NewPlayerVarResolver(makeTeam(42, nil), nil, nil)
	val, ok := r.ResolveVar("points")
	assert.True(t, ok)
	assert.Equal(t, "42", val)
}

func TestCompositeResolver_LocationVisited(t *testing.T) {
	team := makeTeam(0, []models.CheckIn{
		makeCheckIn("the-dell", false),
	})
	r := services.NewPlayerVarResolver(team, nil, nil)

	val, ok := r.ResolveVar("location.the-dell.visited")
	assert.True(t, ok)
	assert.Equal(t, "true", val)

	val, ok = r.ResolveVar("location.citrus-lab.visited")
	assert.True(t, ok, "unvisited location should still return ok=true")
	assert.Equal(t, "false", val)
}

func TestCompositeResolver_LocationCheckedIn(t *testing.T) {
	team := makeTeam(0, []models.CheckIn{
		makeCheckIn("the-dell", true),
		makeCheckIn("citrus-lab", false),
	})
	r := services.NewPlayerVarResolver(team, nil, nil)

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

func TestCompositeResolver_CreatorVars(t *testing.T) {
	creator := map[string]string{"took_bergamot": "true"}
	r := services.NewPlayerVarResolver(makeTeam(0, nil), creator, nil)

	val, ok := r.ResolveVar("took_bergamot")
	assert.True(t, ok)
	assert.Equal(t, "true", val)
}

func TestCompositeResolver_ExternalVars(t *testing.T) {
	ext := map[string]string{"weather": "raining"}
	r := services.NewPlayerVarResolver(makeTeam(0, nil), nil, ext)

	val, ok := r.ResolveVar("weather")
	assert.True(t, ok)
	assert.Equal(t, "raining", val)
}

func TestCompositeResolver_Priority(t *testing.T) {
	// built-in beats creator
	creator := map[string]string{"points": "999"}
	r := services.NewPlayerVarResolver(makeTeam(10, nil), creator, nil)
	val, _ := r.ResolveVar("points")
	assert.Equal(t, "10", val, "built-in points must beat creator var named 'points'")

	// creator beats external
	ext := map[string]string{"weather": "sunny"}
	creator2 := map[string]string{"weather": "raining"}
	r2 := services.NewPlayerVarResolver(makeTeam(0, nil), creator2, ext)
	val, _ = r2.ResolveVar("weather")
	assert.Equal(t, "raining", val, "creator var must beat external var")
}

func TestCompositeResolver_UnknownVar(t *testing.T) {
	r := services.NewPlayerVarResolver(makeTeam(0, nil), nil, nil)
	val, ok := r.ResolveVar("undefined_var")
	assert.False(t, ok)
	assert.Equal(t, "", val)
}

func TestCompositeResolver_NilMaps(t *testing.T) {
	r := services.NewPlayerVarResolver(makeTeam(5, nil), nil, nil)
	// Should not panic
	val, ok := r.ResolveVar("points")
	assert.True(t, ok)
	assert.Equal(t, "5", val)

	_, ok = r.ResolveVar("missing")
	assert.False(t, ok)
}

func TestPlayerVarResolver_GameStatus(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		instance models.Instance
		want     string
	}{
		{
			name:     "active",
			instance: models.Instance{StartTime: bun.NullTime{Time: now.Add(-1 * time.Hour)}},
			want:     "active",
		},
		{
			name: "scheduled",
			instance: models.Instance{StartTime: bun.NullTime{Time: now.Add(1 * time.Hour)}},
			want: "scheduled",
		},
		{
			name:     "closed (no start time)",
			instance: models.Instance{},
			want:     "closed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := services.NewPlayerVarResolver(makeTeamWithInstance(0, tt.instance), nil, nil)
			val, ok := r.ResolveVar("game.status")
			assert.True(t, ok)
			assert.Equal(t, tt.want, val)
		})
	}
}

func TestPlayerVarResolver_TeamCount(t *testing.T) {
	r := services.NewPlayerVarResolver(makeTeam(0, nil), nil, nil)

	val, ok := r.ResolveVar("game.team_count")
	assert.True(t, ok)
	assert.Equal(t, "0", val, "default team count is 0")

	r = r.WithTeamCount(17)
	val, ok = r.ResolveVar("game.team_count")
	assert.True(t, ok)
	assert.Equal(t, "17", val)
}
