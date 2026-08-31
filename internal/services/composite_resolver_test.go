package services_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/stretchr/testify/assert"
)

func TestCompositeResolver_ObjectiveResolvesFromCompletionState(t *testing.T) {
	r := services.NewPlayerVarResolver(nil, map[string]bool{"find-maisie": true})

	val, ok := r.ResolveVar("objective.find-maisie")
	assert.True(t, ok)
	assert.Equal(t, "done", val, "a completed objective must be truthy")

	val, ok = r.ResolveVar("objective.not-yet")
	assert.True(t, ok)
	assert.Empty(t, val, "an incomplete objective resolves, but falsy")

	// Bare "objective." with no slug is not a built-in: falls through to creator vars.
	val, ok = r.ResolveVar("objective.")
	assert.False(t, ok)
	assert.Empty(t, val)
}

func TestCompositeResolver_CreatorVars(t *testing.T) {
	creator := map[string]string{"took_bergamot": "true"}
	r := services.NewPlayerVarResolver(creator, nil)

	val, ok := r.ResolveVar("took_bergamot")
	assert.True(t, ok)
	assert.Equal(t, "true", val)
}

// The objective namespace is claimed whether or not the slug is complete, so a
// creator var can never answer for it.
func TestCompositeResolver_Priority(t *testing.T) {
	creator := map[string]string{"objective.find-maisie": "done"}
	r := services.NewPlayerVarResolver(creator, nil)

	val, ok := r.ResolveVar("objective.find-maisie")
	assert.True(t, ok)
	assert.Empty(t, val, "built-in namespace must beat a creator var of the same name")
}

// The numeric built-ins went with the comparison operators: a document naming
// one now gets no value at all rather than a silently useless one.
func TestCompositeResolver_RetiredBuiltInsAreUnknown(t *testing.T) {
	r := services.NewPlayerVarResolver(nil, nil)

	for _, name := range []string{"points", "player.points", "run.started_at", "game.team_count"} {
		val, ok := r.ResolveVar(name)
		assert.False(t, ok, name)
		assert.Empty(t, val, name)
	}
}

func TestCompositeResolver_UnknownVar(t *testing.T) {
	r := services.NewPlayerVarResolver(nil, nil)
	val, ok := r.ResolveVar("undefined_var")
	assert.False(t, ok)
	assert.Empty(t, val)
}

func TestCompositeResolver_NilMaps(t *testing.T) {
	r := services.NewPlayerVarResolver(nil, nil)
	// Should not panic.
	_, ok := r.ResolveVar("missing")
	assert.False(t, ok)
}
