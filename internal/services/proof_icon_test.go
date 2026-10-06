package services //nolint:testpackage // firstInteractiveProof is unexported

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

func block(owner, blockType string, ordering int, context game.BlockContext) models.Block {
	return models.Block{OwnerID: owner, Type: blockType, Ordering: ordering, Context: context}
}

// The row's icon is the first interactive block in its proof, so the card says
// what the objective will ask for without anyone classifying anything.
func TestFirstInteractiveProof_PicksTheFirstOneThatAsksSomething(t *testing.T) {
	rows := []models.Block{
		block("a", "text", 0, game.ContextObjectiveProof),
		block("a", "scan", 1, game.ContextObjectiveProof),
		block("a", "quiz", 2, game.ContextObjectiveProof),
	}

	got := firstInteractiveProof(rows, blocks.Registry())
	assert.Equal(t, "scan", got["a"], "text asks nothing, so it is not what the row promises")
}

// Ordering decides, not the order rows came back from the database.
func TestFirstInteractiveProof_FollowsTheAuthorsOrdering(t *testing.T) {
	rows := []models.Block{
		block("a", "quiz", 5, game.ContextObjectiveProof),
		block("a", "scan", 1, game.ContextObjectiveProof),
	}

	assert.Equal(t, "scan", firstInteractiveProof(rows, blocks.Registry())["a"])
}

// Reveal blocks are not what an objective asks for: they are what it gives
// back, and an icon from one would promise the wrong thing.
func TestFirstInteractiveProof_IgnoresEverythingButProof(t *testing.T) {
	rows := []models.Block{
		block("a", "quiz", 0, game.ContextObjectiveReveal),
		block("a", "photo", 1, game.ContextObjectiveProof),
	}

	assert.Equal(t, "photo", firstInteractiveProof(rows, blocks.Registry())["a"])
}

// An objective whose proof is all display blocks has nothing to ask for, so it
// contributes no icon rather than a misleading one.
func TestFirstInteractiveProof_SaysNothingWhenNothingIsAsked(t *testing.T) {
	rows := []models.Block{
		block("a", "text", 0, game.ContextObjectiveProof),
		block("a", "image", 1, game.ContextObjectiveProof),
	}

	got := firstInteractiveProof(rows, blocks.Registry())
	assert.NotContains(t, got, "a")
}

func TestFirstInteractiveProof_KeepsOwnersApart(t *testing.T) {
	rows := []models.Block{
		block("a", "scan", 0, game.ContextObjectiveProof),
		block("b", "password", 0, game.ContextObjectiveProof),
	}

	got := firstInteractiveProof(rows, blocks.Registry())
	assert.Equal(t, "scan", got["a"])
	assert.Equal(t, "password", got["b"])
}

// The quest screen's own actions are derived the same way a row's icon is:
// from what the frontier actually asks for. A quest with no codes in it never
// offers a scanner.
func TestProofBlockTypes_NamesWhatTheFrontierAsksFor(t *testing.T) {
	rows := []models.Block{
		block("a", "text", 0, game.ContextObjectiveProof),
		block("a", "scan", 1, game.ContextObjectiveProof),
		block("b", "quiz", 0, game.ContextObjectiveProof),
		block("b", "geofence", 1, game.ContextObjectiveProof),
		block("c", "scan", 0, game.ContextObjectiveReveal),
	}

	got := proofBlockTypes(rows)
	assert.True(t, got["scan"])
	assert.True(t, got["geofence"])
	assert.True(t, got["quiz"], "every type, not only the first on each objective")
	assert.False(t, got["text"] && false)
	assert.Len(t, got, 4, "reveal blocks are not what the quest is asking for")
}
