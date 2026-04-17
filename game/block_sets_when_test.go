package game_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v7/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// concreteBlock is a minimal Block implementation for testing BaseBlock behaviour.
type concreteBlock struct {
	game.BaseBlock
	Content string `json:"content,omitempty"`
}

func (b *concreteBlock) GetName() string        { return "" }
func (b *concreteBlock) GetDescription() string { return "" }
func (b *concreteBlock) GetIconSVG() string     { return "" }
func (b *concreteBlock) RequiresValidation() bool { return false }
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
	assert.Nil(t, b.GetWhen())
}

func TestBaseBlock_GetSets_PopulatedByParseData(t *testing.T) {
	raw := json.RawMessage(`{
		"content": "hello",
		"sets": {"took_bergamot": "correct", "visited_lab": "attempted"}
	}`)
	b := &concreteBlock{BaseBlock: game.BaseBlock{Data: raw}}
	require.NoError(t, b.ParseData())

	assert.Equal(t, map[string]string{
		"took_bergamot": "correct",
		"visited_lab":   "attempted",
	}, b.GetSets())
	assert.Equal(t, "hello", b.Content)
}

func TestBaseBlock_GetWhen_PopulatedByParseData(t *testing.T) {
	raw := json.RawMessage(`{
		"when": {
			"all_of": [
				{"var": "took_bergamot"},
				{"var": "points", "op": "gte", "value": 10}
			]
		}
	}`)
	b := &concreteBlock{BaseBlock: game.BaseBlock{Data: raw}}
	require.NoError(t, b.ParseData())

	w := b.GetWhen()
	require.NotNil(t, w)
	require.Len(t, w.AllOf, 2)
	assert.Equal(t, "took_bergamot", w.AllOf[0].Var)
	assert.Equal(t, "points", w.AllOf[1].Var)
	assert.Equal(t, "gte", w.AllOf[1].Op)
}

func TestBaseBlock_NoSetsOrWhenInJSON(t *testing.T) {
	raw := json.RawMessage(`{"content": "plain block"}`)
	b := &concreteBlock{BaseBlock: game.BaseBlock{Data: raw}}
	require.NoError(t, b.ParseData())

	assert.Nil(t, b.GetSets())
	assert.Nil(t, b.GetWhen())
	assert.Equal(t, "plain block", b.Content)
}
