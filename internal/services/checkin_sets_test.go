package services //nolint:testpackage // white-box test for unexported writeSetsVars

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// fakeVarStateRepo records Upsert calls so tests can assert on the values
// actually persisted, not just the variable names.
type fakeVarStateRepo struct {
	written map[string]string
}

func newFakeVarStateRepo() *fakeVarStateRepo {
	return &fakeVarStateRepo{written: map[string]string{}}
}

func (f *fakeVarStateRepo) Upsert(_ context.Context, _, _, varName, varValue string) error {
	f.written[varName] = varValue
	return nil
}

func (f *fakeVarStateRepo) GetAll(_ context.Context, _, _ string) (map[string]string, error) {
	return f.written, nil
}

func (f *fakeVarStateRepo) DeleteByTeamAndInstance(_ context.Context, _ *bun.Tx, _, _ string) error {
	f.written = map[string]string{}
	return nil
}

// setsBlock is a minimal Block carrying a fixed SetsField.
type setsBlock struct {
	blocks.BaseBlock
}

func (b *setsBlock) GetID() string                               { return b.ID }
func (b *setsBlock) GetType() string                             { return "sets" }
func (b *setsBlock) GetOwnerID() string                          { return b.OwnerID }
func (b *setsBlock) GetOrder() int                               { return b.Order }
func (b *setsBlock) GetPoints() int                              { return b.Points }
func (b *setsBlock) GetName() string                             { return "sets" }
func (b *setsBlock) GetDescription() string                      { return "" }
func (b *setsBlock) GetIconSVG() string                          { return "" }
func (b *setsBlock) GetData() json.RawMessage                    { return nil }
func (b *setsBlock) RequiresValidation() bool                    { return true }
func (b *setsBlock) ParseData() error                            { return nil }
func (b *setsBlock) UpdateBlockData(_ map[string][]string) error { return nil }
func (b *setsBlock) ValidatePlayerInput(
	state blocks.PlayerState,
	_ map[string][]string,
) (blocks.PlayerState, error) {
	return state, nil
}

// stubState is a PlayerState whose completion can be set directly.
type stubState struct {
	complete bool
}

func (s *stubState) GetBlockID() string              { return "block-1" }
func (s *stubState) GetPlayerID() string             { return "RUN1" }
func (s *stubState) GetQuestID() string              { return "quest-1" }
func (s *stubState) GetPlayerData() json.RawMessage  { return nil }
func (s *stubState) SetPlayerData(_ json.RawMessage) {}
func (s *stubState) IsComplete() bool                { return s.complete }
func (s *stubState) SetComplete(complete bool)       { s.complete = complete }
func (s *stubState) GetPointsAwarded() int           { return 0 }
func (s *stubState) SetPointsAwarded(_ int)          {}

func TestWriteSetsVars_WritesEachNameAsTrue(t *testing.T) {
	tests := []struct {
		name string
		sets game.SetsField
		want map[string]string
	}{
		{
			name: "a single name",
			sets: game.SetsField{"found_clue"},
			want: map[string]string{"found_clue": "true"},
		},
		{
			name: "every name in the list",
			sets: game.SetsField{"score", "clue", "found"},
			want: map[string]string{"score": "true", "clue": "true", "found": "true"},
		},
		{
			name: "an empty list writes nothing",
			sets: game.SetsField{},
			want: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeVarStateRepo()
			svc := &CheckInService{varStateRepo: repo}
			block := &setsBlock{BaseBlock: blocks.BaseBlock{ID: "block-1", Sets: tt.sets}}

			err := svc.writeSetsVars(
				context.Background(),
				models.Run{Code: "RUN1", QuestID: "quest-1"},
				block,
				&stubState{complete: true},
			)

			require.NoError(t, err)
			assert.Equal(t, tt.want, repo.written)
		})
	}
}

func TestWriteSetsVars_SkipsIncompleteBlock(t *testing.T) {
	repo := newFakeVarStateRepo()
	svc := &CheckInService{varStateRepo: repo}
	block := &setsBlock{BaseBlock: blocks.BaseBlock{ID: "block-1", Sets: game.SetsField{"score"}}}

	err := svc.writeSetsVars(
		context.Background(),
		models.Run{Code: "RUN1", QuestID: "quest-1"},
		block,
		&stubState{complete: false},
	)

	require.NoError(t, err)
	assert.Empty(t, repo.written)
}
