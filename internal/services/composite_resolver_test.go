package services_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

func makeTeam(points int) *models.Run {
	return &models.Run{Points: points}
}

func TestCompositeResolver_Points(t *testing.T) {
	r := services.NewPlayerVarResolver(makeTeam(42), nil)
	val, ok := r.ResolveVar("points")
	assert.True(t, ok)
	assert.Equal(t, "42", val)
}

func TestCompositeResolver_ObjectiveStub(t *testing.T) {
	// objective.<slug> is recognised but always "" until objectives ship (Stage C).
	r := services.NewPlayerVarResolver(makeTeam(0), nil)

	val, ok := r.ResolveVar("objective.find-maisie")
	assert.True(t, ok)
	assert.Empty(t, val)

	val, ok = r.ResolveVar("objective.any-slug")
	assert.True(t, ok)
	assert.Empty(t, val)

	// Bare "objective." with no slug is not a built-in — falls through to creator vars.
	val, ok = r.ResolveVar("objective.")
	assert.False(t, ok)
	assert.Empty(t, val)
}

func TestCompositeResolver_CreatorVars(t *testing.T) {
	creator := map[string]string{"took_bergamot": "true"}
	r := services.NewPlayerVarResolver(makeTeam(0), creator)

	val, ok := r.ResolveVar("took_bergamot")
	assert.True(t, ok)
	assert.Equal(t, "true", val)
}

func TestCompositeResolver_Priority(t *testing.T) {
	// built-in beats creator
	creator := map[string]string{"points": "999"}
	r := services.NewPlayerVarResolver(makeTeam(10), creator)
	val, _ := r.ResolveVar("points")
	assert.Equal(t, "10", val, "built-in points must beat creator var named 'points'")
}

func TestCompositeResolver_UnknownVar(t *testing.T) {
	r := services.NewPlayerVarResolver(makeTeam(0), nil)
	val, ok := r.ResolveVar("undefined_var")
	assert.False(t, ok)
	assert.Empty(t, val)
}

func TestCompositeResolver_NilMaps(t *testing.T) {
	r := services.NewPlayerVarResolver(makeTeam(5), nil)
	// Should not panic
	val, ok := r.ResolveVar("points")
	assert.True(t, ok)
	assert.Equal(t, "5", val)

	_, ok = r.ResolveVar("missing")
	assert.False(t, ok)
}

func TestPlayerVarResolver_TeamCount(t *testing.T) {
	r := services.NewPlayerVarResolver(makeTeam(0), nil)

	val, ok := r.ResolveVar("game.team_count")
	assert.True(t, ok)
	assert.Equal(t, "0", val, "default team count is 0")

	r = r.WithRunCount(17)
	val, ok = r.ResolveVar("game.team_count")
	assert.True(t, ok)
	assert.Equal(t, "17", val)
}
