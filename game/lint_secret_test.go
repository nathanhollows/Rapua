package game_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/stretchr/testify/assert"
)

func secretDoc() *game.GameDoc {
	doc := validDoc()
	sec := section(doc)
	sec.Children = append(sec.Children, game.ObjectiveDoc{
		Slug: "rooftop", Title: "Rooftop", Secret: boolPtr(true),
		Proof: game.ObjectiveContextDoc{Blocks: []game.BlockDoc{{"type": "scan"}}},
	})
	return doc
}

func TestLint_SecretWithAScanIsValid(t *testing.T) {
	result := game.Lint(secretDoc(), newTestRegistry())
	for _, code := range []string{"SECRET_NEEDS_SCAN", "ONLY_SECRET_CHILDREN", "BAND_OUT_OF_RANGE"} {
		assert.False(t, result.HasError(code), code)
	}
}

func TestLint_RootSecret(t *testing.T) {
	doc := validDoc()
	doc.Structure.Secret = boolPtr(true)

	assert.True(t, game.Lint(doc, newTestRegistry()).HasError("ROOT_SECRET"))
}

func TestLint_SecretNeedsAScan(t *testing.T) {
	doc := secretDoc()
	secret := &section(doc).Children[1]
	secret.Proof.Blocks = []game.BlockDoc{{"type": "quiz"}}

	assert.True(t, game.Lint(doc, newTestRegistry()).HasError("SECRET_NEEDS_SCAN"),
		"nothing lists it, so a quiz has nowhere to be answered")

	secret.Proof.Blocks = []game.BlockDoc{{"type": "scan"}, {"type": "quiz"}}
	assert.False(t, game.Lint(doc, newTestRegistry()).HasError("SECRET_NEEDS_SCAN"),
		"a quiz after the scan is fine")
}

func TestLint_SecretSideQuestCannotPassThrough(t *testing.T) {
	doc := secretDoc()
	secret := &section(doc).Children[1]
	secret.PassThrough = boolPtr(true)
	secret.Routing = game.RouteStrategyFreeRoam
	secret.Children = []game.ObjectiveDoc{{Slug: "ledge", Title: "Ledge",
		Proof: game.ObjectiveContextDoc{Blocks: []game.BlockDoc{{"type": "quiz"}}}}}

	assert.True(t, game.Lint(doc, newTestRegistry()).HasError("SECRET_PASS_THROUGH"))
}

func TestLint_BandsCountVisibleChildrenOnly(t *testing.T) {
	doc := secretDoc()
	section(doc).ChildrenMin = intPtr(2)
	section(doc).ChildrenMax = intPtr(2)

	assert.True(t, game.Lint(doc, newTestRegistry()).HasError("BAND_OUT_OF_RANGE"),
		"a band of two over one visible child would require the find")
}

func TestLintVars_SecretTakesNoTurnInOrder(t *testing.T) {
	doc := varDoc()
	doc.Structure.Routing = game.RouteStrategyOrdered
	// The door comes first but is secret, so the gate after it does not wait on it.
	doc.Structure.Children[0], doc.Structure.Children[1] = doc.Structure.Children[1], doc.Structure.Children[0]
	secret := &doc.Structure.Children[0]
	secret.Secret = boolPtr(true)
	secret.Proof.Blocks = []game.BlockDoc{{"type": "scan"}}

	errs, _ := lintCodes(doc)
	assert.Empty(t, errs)
}

func TestLintVars_SecretIsOutsideTheWindow(t *testing.T) {
	doc := varDoc()
	doc.Structure.Routing = game.RouteStrategyRandomised
	doc.Structure.MaxNext = 1
	door(doc).Secret = boolPtr(true)
	door(doc).Proof.Blocks = []game.BlockDoc{{"type": "scan"}}

	_, warns := lintCodes(doc)
	assert.NotContains(t, warns, "DEPENDS_IN_RANDOMISED_WINDOW", "a secret never takes a place in it")
}

func TestLint_OnlySecretChildren(t *testing.T) {
	doc := secretDoc()
	sec := section(doc)
	sec.Children = sec.Children[1:]

	result := game.Lint(doc, newTestRegistry())
	assert.True(t, result.HasError("ONLY_SECRET_CHILDREN"),
		"with nothing visible it completes on arrival and closes over its secrets")
	assert.False(t, result.HasWarning("ALL_CHILDREN_DRAFT"))
}
