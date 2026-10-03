package blocks_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeofenceBlock_IsRegisteredAndProofOnly(t *testing.T) {
	registry := blocks.Registry()
	require.True(t, registry.IsValidType("geofence"), "the linter must know it")
	assert.True(t, registry.IsInteractive("geofence"), "a proof context needs one of these")
	assert.True(t, registry.CanUseInContext("geofence", game.ContextObjectiveProof))
	assert.False(t, registry.CanUseInContext("geofence", game.ContextStart))
}
