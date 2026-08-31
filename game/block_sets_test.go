package game_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// concreteBlock is a minimal Block implementation for testing BaseBlock behaviour.
type concreteBlock struct {
	game.BaseBlock
	Content string `json:"content,omitempty"`
}

func (b *concreteBlock) GetName() string                             { return "" }
func (b *concreteBlock) GetDescription() string                      { return "" }
func (b *concreteBlock) GetIconSVG() string                          { return "" }
func (b *concreteBlock) RequiresValidation() bool                    { return false }
func (b *concreteBlock) UpdateBlockData(_ map[string][]string) error { return nil }
func (b *concreteBlock) ValidatePlayerInput(state game.PlayerState, _ map[string][]string) (game.PlayerState, error) {
	return state, nil
}

func (b *concreteBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func TestBaseBlock_GetSets_NilByDefault(t *testing.T) {
	b := &concreteBlock{}
	assert.Nil(t, b.GetSets())
}

func TestBaseBlock_GetSets_PopulatedByParseData(t *testing.T) {
	raw := json.RawMessage(`{
		"content": "hello",
		"sets": ["took_bergamot", "found_still"]
	}`)
	b := &concreteBlock{BaseBlock: game.BaseBlock{Data: raw}}
	require.NoError(t, b.ParseData())

	assert.Equal(t, game.SetsField{"took_bergamot", "found_still"}, b.GetSets())
	assert.Equal(t, "hello", b.Content)
}

func TestBaseBlock_ParseData_RejectsObjectSets(t *testing.T) {
	raw := json.RawMessage(`{"sets": {"took_bergamot": "true"}}`)
	b := &concreteBlock{BaseBlock: game.BaseBlock{Data: raw}}

	require.Error(t, b.ParseData())
}

func TestBaseBlock_NoSetsInJSON(t *testing.T) {
	raw := json.RawMessage(`{"content": "plain block"}`)
	b := &concreteBlock{BaseBlock: game.BaseBlock{Data: raw}}
	require.NoError(t, b.ParseData())

	assert.Nil(t, b.GetSets())
	assert.Equal(t, "plain block", b.Content)
}
