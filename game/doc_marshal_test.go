package game_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ChildDoc marshal / unmarshal ---

func TestBlockDoc_MarshalJSON_TypeFirst(t *testing.T) {
	b := game.BlockDoc{"type": "quiz", "question": "What?", "answer": "42"}
	data, err := json.Marshal(b)
	require.NoError(t, err)

	// "type" must appear first
	raw := string(data)
	typeIdx := findFieldIdx(raw, `"type"`)
	questionIdx := findFieldIdx(raw, `"question"`)
	answerIdx := findFieldIdx(raw, `"answer"`)

	assert.Less(t, typeIdx, questionIdx, `"type" should come before "question"`)
	assert.Less(t, typeIdx, answerIdx, `"type" should come before "answer"`)
	// remaining fields are alphabetical: answer before question
	assert.Less(t, answerIdx, questionIdx, `"answer" should come before "question" alphabetically`)
}

func TestBlockDoc_MarshalJSON_IDSecond(t *testing.T) {
	b := game.BlockDoc{"type": "text", "id": "abc-123", "content": "hello"}
	data, err := json.Marshal(b)
	require.NoError(t, err)

	raw := string(data)
	typeIdx := findFieldIdx(raw, `"type"`)
	idIdx := findFieldIdx(raw, `"id"`)
	contentIdx := findFieldIdx(raw, `"content"`)

	assert.Less(t, typeIdx, idIdx, `"type" should come before "id"`)
	assert.Less(t, idIdx, contentIdx, `"id" should come before "content"`)
}

func TestBlockDoc_MarshalJSON_UnmarshalableType(t *testing.T) {
	// Exercises the writeKey("type") error return path.
	b := game.BlockDoc{"type": make(chan int)}
	_, err := json.Marshal(b)
	assert.Error(t, err)
}

func TestBlockDoc_MarshalJSON_UnmarshalableID(t *testing.T) {
	b := game.BlockDoc{"type": "text", "id": make(chan int)}
	_, err := json.Marshal(b)
	assert.Error(t, err)
}

func TestBlockDoc_MarshalJSON_UnmarshalableField(t *testing.T) {
	b := game.BlockDoc{"type": "text", "content": make(chan int)}
	_, err := json.Marshal(b)
	assert.Error(t, err)
}

func TestBlockDoc_MarshalJSON_NoType(t *testing.T) {
	b := game.BlockDoc{"answer": "42", "question": "What?"}
	data, err := json.Marshal(b)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, "42", out["answer"])
	assert.Equal(t, "What?", out["question"])
}

func TestBlockDoc_RoundTrip(t *testing.T) {
	b := game.BlockDoc{
		"type":    "quiz",
		"id":      "q1",
		"points":  float64(5),
		"content": "What is 2+2?",
	}
	data, err := json.Marshal(b)
	require.NoError(t, err)

	var out game.BlockDoc
	require.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, "quiz", out["type"])
	assert.Equal(t, "q1", out["id"])
}

// --- Full GameDoc round-trip ---

func TestGameDoc_RoundTrip(t *testing.T) {
	minChildren := 1
	doc := &game.GameDoc{
		Rapua: "v8",
		Name:  "Round Trip Game",
		Settings: game.SettingsDoc{
			EnablePoints: true,
		},
		Start:  []game.BlockDoc{{"type": "start_button"}},
		Finish: []game.BlockDoc{},
		Structure: game.ObjectiveDoc{
			Slug:    "root",
			Title:   "Round Trip Game",
			Routing: game.RouteStrategyFreeRoam,
			Children: []game.ObjectiveDoc{
				{
					Slug:  "lobby",
					Title: "The Lobby",
					Proof: game.ObjectiveContextDoc{
						Blocks: []game.BlockDoc{{"type": "text", "content": "Hello"}},
					},
				},
				{
					Slug:        "east-wing",
					Title:       "East Wing",
					Color:       "primary",
					Routing:     game.RouteStrategyFreeRoam,
					ChildrenMin: &minChildren,
					FinishLabel: "Leave the wing",
					Children: []game.ObjectiveDoc{
						{Slug: "room-a", Title: "Room A"},
					},
				},
			},
		},
	}

	data, err := json.Marshal(doc)
	require.NoError(t, err)

	var out game.GameDoc
	require.NoError(t, json.Unmarshal(data, &out))

	assert.Equal(t, "v8", out.Rapua)
	assert.Equal(t, "Round Trip Game", out.Name)
	assert.Equal(t, "root", out.Structure.Slug)
	require.Len(t, out.Structure.Children, 2)

	assert.Equal(t, "lobby", out.Structure.Children[0].Slug)
	assert.Empty(t, out.Structure.Children[0].Children, "a leaf has no children")

	wing := out.Structure.Children[1]
	assert.Equal(t, "east-wing", wing.Slug)
	assert.Equal(t, "Leave the wing", wing.FinishLabel)
	require.Len(t, wing.Children, 1)
	assert.Equal(t, "room-a", wing.Children[0].Slug)

	require.NotNil(t, wing.ChildrenMin)
	assert.Equal(t, 1, *wing.ChildrenMin)
	assert.Nil(t, wing.ChildrenMax, "an omitted bound must stay omitted through a round trip")
}

// An explicit children_min of 0 is a different node from one that omits it, so
// the zero must survive marshalling rather than being dropped as empty.
func TestGameDoc_RoundTrip_ExplicitZeroMinSurvives(t *testing.T) {
	zero := 0
	doc := &game.GameDoc{
		Rapua: "v8",
		Name:  "Zero Min",
		Structure: game.ObjectiveDoc{
			Slug: "root", Title: "Zero Min",
			Routing:     game.RouteStrategyFreeRoam,
			ChildrenMin: &zero,
			Children:    []game.ObjectiveDoc{{Slug: "bonus", Title: "Bonus"}},
		},
	}

	data, err := json.Marshal(doc)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"children_min":0`)

	var out game.GameDoc
	require.NoError(t, json.Unmarshal(data, &out))
	require.NotNil(t, out.Structure.ChildrenMin)
	assert.Equal(t, 0, *out.Structure.ChildrenMin)
	assert.Equal(t, game.Band{Min: 0, Max: 1}, out.Structure.Band())
}

// findFieldIdx returns the index of the first occurrence of key in the JSON string.
func findFieldIdx(s, key string) int {
	for i := 0; i <= len(s)-len(key); i++ {
		if s[i:i+len(key)] == key {
			return i
		}
	}
	return -1
}
