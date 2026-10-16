package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmission_Password(t *testing.T) {
	block := &blocks.PasswordBlock{Answer: "secret"}

	wrong := map[string][]string{"answer": {"guess"}}
	state, err := block.ValidatePlayerInput(&blocks.MockPlayerState{}, wrong)
	require.NoError(t, err)
	assert.JSONEq(t, `{"answer":"guess"}`, string(block.Submission(wrong, state)))

	right := map[string][]string{"answer": {"secret"}}
	state, err = block.ValidatePlayerInput(&blocks.MockPlayerState{}, right)
	require.NoError(t, err)
	assert.Nil(t, block.Submission(right, state), "a solve carries nothing")
}

func TestSubmission_Pincode(t *testing.T) {
	block := &blocks.PincodeBlock{Pincode: "1234"}

	wrong := map[string][]string{"pincode": {"4321"}}
	state, err := block.ValidatePlayerInput(&blocks.MockPlayerState{}, wrong)
	require.NoError(t, err)
	assert.JSONEq(t, `{"pincode":"4321"}`, string(block.Submission(wrong, state)))

	right := map[string][]string{"pincode": {"1234"}}
	state, err = block.ValidatePlayerInput(&blocks.MockPlayerState{}, right)
	require.NoError(t, err)
	assert.Nil(t, block.Submission(right, state))
}

func TestSubmission_ScanReadsTheRequest(t *testing.T) {
	block := &blocks.ScanBlock{
		Codes: []blocks.ScanCode{{Value: "right"}},
		Match: blocks.MatchExact,
	}

	wrong := map[string][]string{"code": {"wrong"}}
	state, err := block.ValidatePlayerInput(&blocks.MockPlayerState{}, wrong)
	require.NoError(t, err)
	assert.JSONEq(t, `{"scanned":"wrong","modality":"typed"}`, string(block.Submission(wrong, state)))

	right := map[string][]string{"code": {"right"}}
	state, err = block.ValidatePlayerInput(&blocks.MockPlayerState{}, right)
	require.NoError(t, err)
	assert.Nil(t, block.Submission(right, state))
}

// A quiz without retries completes on a wrong answer, so the report cannot go by
// completion the way the other graders do.
func TestSubmission_QuizWrongButComplete(t *testing.T) {
	block := &blocks.QuizBlock{
		Options:      []blocks.QuizOption{{ID: "a", IsCorrect: true}, {ID: "b"}},
		RetryEnabled: false,
	}
	in := map[string][]string{"quiz_option": {"b"}}

	state, err := block.ValidatePlayerInput(&blocks.MockPlayerState{}, in)
	require.NoError(t, err)
	require.True(t, state.IsComplete(), "a quiz without retries completes on a wrong answer")

	assert.JSONEq(t, `{"selected_options":["b"]}`, string(block.Submission(in, state)))
}

func TestSubmission_SortingWrongOrder(t *testing.T) {
	block := &blocks.SortingBlock{
		ScoringScheme: blocks.RetryUntilCorrect,
		Items: []blocks.SortingItem{
			{ID: "a", Position: 1},
			{ID: "b", Position: 2},
		},
	}

	wrong := map[string][]string{"sorting-item-order": {"b", "a"}}
	state, err := block.ValidatePlayerInput(&blocks.MockPlayerState{}, wrong)
	require.NoError(t, err)
	assert.JSONEq(t, `{"player_order":["b","a"]}`, string(block.Submission(wrong, state)))

	right := map[string][]string{"sorting-item-order": {"a", "b"}}
	state, err = block.ValidatePlayerInput(&blocks.MockPlayerState{}, right)
	require.NoError(t, err)
	assert.Nil(t, block.Submission(right, state))
}

// The geofence reads its detail off the state rather than the request, so this is
// the reporter most likely to drift from the block's own data.
func TestSubmission_GeofenceReadsTheState(t *testing.T) {
	block := &blocks.GeofenceBlock{Area: blocks.NewCircle(0, 0, 10)}

	outside := map[string][]string{"lat": {"1"}, "lng": {"1"}, "accuracy": {"5"}}
	state, err := block.ValidatePlayerInput(&blocks.MockPlayerState{}, outside)
	require.NoError(t, err)

	var got map[string]float64
	require.NoError(t, json.Unmarshal(block.Submission(outside, state), &got))
	assert.InDelta(t, 1, got["lat"], 0.0001)
	assert.InDelta(t, 5, got["accuracy"], 0.0001)
	assert.Positive(t, got["metres_away"], "a near miss records how near")

	inside := map[string][]string{"lat": {"0"}, "lng": {"0"}, "accuracy": {"5"}}
	state, err = block.ValidatePlayerInput(&blocks.MockPlayerState{}, inside)
	require.NoError(t, err)
	assert.Nil(t, block.Submission(inside, state))
}

// A checklist posts on every tick, so each row carries the set as it stood. The
// state keeps only the latest, and nothing else dates the earlier ones.
func TestSubmission_ChecklistProgress(t *testing.T) {
	block := &blocks.ChecklistBlock{
		List: []blocks.ChecklistItem{{ID: "a"}, {ID: "b"}, {ID: "c"}},
	}

	partial := map[string][]string{"checklist-item-ids": {"a", "c"}}
	state, err := block.ValidatePlayerInput(&blocks.MockPlayerState{}, partial)
	require.NoError(t, err)
	require.False(t, state.IsComplete())
	assert.JSONEq(t, `{"checked_items":["a","c"]}`, string(block.Submission(partial, state)))

	done := map[string][]string{"checklist-item-ids": {"a", "b", "c"}}
	state, err = block.ValidatePlayerInput(&blocks.MockPlayerState{}, done)
	require.NoError(t, err)
	require.True(t, state.IsComplete())
	assert.Nil(t, block.Submission(done, state), "a finished list carries nothing")
}

func TestSubmission_FreeTextIsSilent(t *testing.T) {
	var block game.Block = &blocks.FreeTextBlock{}
	_, reports := block.(game.SubmissionReporter)
	assert.False(t, reports, "the response is the whole record, and the state keeps it")
}

// Reporting is by omission, so a new interactive block would record nothing and
// say nothing. Every block that takes input must either report or be named here
// with the reason it does not.
func TestSubmission_EveryInteractiveBlockDecides(t *testing.T) {
	exempt := map[string]string{
		"choice":    "any valid choice is accepted, so it cannot be wrong",
		"clue":      "revealing is the whole submission, and it completes",
		"free_text": "the final response is the whole record",
		"photo":     "an upload is its own event, and a deletion is not graded",
		"rating":    "a rating cannot be wrong, and it completes",
		"team_name": "the name is a run_named event",
	}

	for _, reg := range blocks.GetRegisteredBlocks() {
		if !reg.Prototype.RequiresValidation() {
			continue
		}
		_, reports := reg.Prototype.(game.SubmissionReporter)
		_, isExempt := exempt[reg.BlockType]
		assert.NotEqual(t, reports, isExempt,
			"%s must either implement SubmissionReporter or be exempt, not both or neither", reg.BlockType)
	}
}
