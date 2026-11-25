package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
)

// A block change that breaks the gallery should fail here, not in the command.
func TestSeedGame_LintsClean(t *testing.T) {
	doc, err := seedGame()
	require.NoError(t, err)
	result := game.Lint(doc, blocks.Registry())
	require.True(t, result.IsValid(), "%v", result.Errors)
}
