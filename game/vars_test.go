package game_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/stretchr/testify/assert"
)

func TestValidVarName(t *testing.T) {
	valid := []string{"a", "_", "met_guide", "door2", "_hidden"}
	for _, name := range valid {
		assert.True(t, game.ValidVarName(name), name)
	}
	invalid := []string{"", "2door", "Met", "met-guide", "met guide", "objective.x", string(make([]byte, 65))}
	for _, name := range invalid {
		assert.False(t, game.ValidVarName(name), name)
	}
}

func TestDependsMet(t *testing.T) {
	vars := map[string]bool{"a": true, "b": true}
	assert.True(t, game.DependsMet(nil, nil), "no depends is met")
	assert.True(t, game.DependsMet(game.DependsField{"a", "b"}, vars))
	assert.False(t, game.DependsMet(game.DependsField{"a", "c"}, vars), "every name must be set")
	assert.False(t, game.DependsMet(game.DependsField{""}, vars), "an empty name fails closed")
}

func TestNormaliseDepends(t *testing.T) {
	assert.Equal(t, game.DependsField{"b", "a"}, game.NormaliseDepends([]string{"b", "", "a", "b"}))
	assert.Nil(t, game.NormaliseDepends([]string{"", ""}))
}

// stubBlock is the least a block needs to answer the variable helpers.
type stubBlock struct {
	game.BaseBlock
	interactive bool
}

func (b *stubBlock) GetID() string                             { return b.ID }
func (b *stubBlock) GetType() string                           { return "stub" }
func (b *stubBlock) GetOwnerID() string                        { return b.OwnerID }
func (b *stubBlock) GetName() string                           { return "Stub" }
func (b *stubBlock) GetDescription() string                    { return "" }
func (b *stubBlock) GetOrder() int                             { return b.Order }
func (b *stubBlock) GetPoints() int                            { return b.Points }
func (b *stubBlock) GetIconSVG() string                        { return "" }
func (b *stubBlock) GetData() json.RawMessage                  { return nil }
func (b *stubBlock) ParseData() error                          { return nil }
func (b *stubBlock) UpdateBlockData(map[string][]string) error { return nil }
func (b *stubBlock) RequiresValidation() bool                  { return b.interactive }
func (b *stubBlock) ValidatePlayerInput(s game.PlayerState, _ map[string][]string) (game.PlayerState, error) {
	return s, nil
}

type stubState struct{ complete bool }

func (s *stubState) GetBlockID() string             { return "" }
func (s *stubState) GetPlayerID() string            { return "" }
func (s *stubState) GetQuestID() string             { return "" }
func (s *stubState) GetPlayerData() json.RawMessage { return nil }
func (s *stubState) SetPlayerData(json.RawMessage)  {}
func (s *stubState) IsComplete() bool               { return s.complete }
func (s *stubState) SetComplete(c bool)             { s.complete = c }
func (s *stubState) GetPointsAwarded() int          { return 0 }
func (s *stubState) SetPointsAwarded(int)           {}

func TestVarsSetBy(t *testing.T) {
	interactive := &stubBlock{BaseBlock: game.BaseBlock{Sets: "met_guide"}, interactive: true}
	assert.Equal(t, []string{"met_guide"}, game.VarsSetBy(interactive, &stubState{complete: true}))
	assert.Nil(t, game.VarsSetBy(interactive, &stubState{}), "an unsolved block sets nothing")
	assert.Nil(t, game.VarsSetBy(interactive, nil))

	content := &stubBlock{BaseBlock: game.BaseBlock{Sets: "met_guide"}}
	assert.Nil(t, game.VarsSetBy(content, &stubState{complete: true}), "content cannot earn a gate")
	assert.Nil(t, game.DeclaredVars(content))
	assert.Equal(t, []string{"met_guide"}, game.DeclaredVars(interactive))
}
