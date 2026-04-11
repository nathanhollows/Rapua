package game_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v7/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ChildDoc marshal / unmarshal ---

func TestChildDoc_MarshalJSON_Location(t *testing.T) {
	c := game.ChildDoc{
		Location: &game.LocationDoc{
			Slug: "lobby",
			Name: "The Lobby",
		},
	}
	data, err := json.Marshal(c)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Contains(t, raw, "location")
	assert.NotContains(t, raw, "group")
}

func TestChildDoc_MarshalJSON_Group(t *testing.T) {
	c := game.ChildDoc{
		Group: &game.GroupDoc{
			Name:  "My Group",
			Color: "primary",
		},
	}
	data, err := json.Marshal(c)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	assert.Contains(t, raw, "group")
	assert.NotContains(t, raw, "location")
}

func TestChildDoc_MarshalJSON_Neither_Error(t *testing.T) {
	c := game.ChildDoc{}
	_, err := json.Marshal(c)
	assert.Error(t, err)
}

func TestChildDoc_UnmarshalJSON_Location(t *testing.T) {
	input := `{"location":{"slug":"lobby","name":"The Lobby","content":[],"clues":[],"tasks":[],"checkpoint":[]}}`
	var c game.ChildDoc
	require.NoError(t, json.Unmarshal([]byte(input), &c))
	require.NotNil(t, c.Location)
	assert.Equal(t, "lobby", c.Location.Slug)
	assert.Nil(t, c.Group)
}

func TestChildDoc_UnmarshalJSON_Group(t *testing.T) {
	input := `{"group":{"name":"My Group","color":"primary","routing":"free_roam","navigation":"map","completion":"all","children":[]}}`
	var c game.ChildDoc
	require.NoError(t, json.Unmarshal([]byte(input), &c))
	require.NotNil(t, c.Group)
	assert.Equal(t, "My Group", c.Group.Name)
	assert.Nil(t, c.Location)
}

func TestChildDoc_UnmarshalJSON_InvalidJSON(t *testing.T) {
	// Go's JSON scanner rejects completely invalid input before calling UnmarshalJSON,
	// so use valid JSON that isn't an object to trigger the error inside UnmarshalJSON.
	var c game.ChildDoc
	err := json.Unmarshal([]byte(`42`), &c)
	assert.Error(t, err)
}

func TestChildDoc_UnmarshalJSON_NeitherKey(t *testing.T) {
	var c game.ChildDoc
	err := json.Unmarshal([]byte(`{"foo":"bar"}`), &c)
	assert.Error(t, err)
}

func TestChildDoc_UnmarshalJSON_MalformedLocation(t *testing.T) {
	var c game.ChildDoc
	err := json.Unmarshal([]byte(`{"location":"not-an-object"}`), &c)
	assert.Error(t, err)
}

func TestChildDoc_UnmarshalJSON_MalformedGroup(t *testing.T) {
	var c game.ChildDoc
	err := json.Unmarshal([]byte(`{"group":"not-an-object"}`), &c)
	assert.Error(t, err)
}

// --- BlockDoc marshal ---

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
	doc := &game.GameDoc{
		Rapua: "v7",
		Name:  "Round Trip Game",
		Settings: game.SettingsDoc{
			EnablePoints: true,
		},
		Start:  []game.BlockDoc{{"type": "start_button"}},
		Finish: []game.BlockDoc{},
		Structure: game.StructureDoc{
			Routing:    game.RouteStrategyFreeRoam,
			Navigation: game.NavigationMap,
			Completion: game.CompletionAll,
			Children: []game.ChildDoc{
				{Location: &game.LocationDoc{
					Slug:    "lobby",
					Name:    "The Lobby",
					Content: []game.BlockDoc{{"type": "text", "content": "Hello"}},
				}},
				{Group: &game.GroupDoc{
					Name:       "East Wing",
					Color:      "primary",
					Routing:    game.RouteStrategyFreeRoam,
					Navigation: game.NavigationMap,
					Completion: game.CompletionAll,
					Children: []game.ChildDoc{
						{Location: &game.LocationDoc{
							Slug:    "room-a",
							Name:    "Room A",
							Content: []game.BlockDoc{},
						}},
					},
				}},
			},
		},
	}

	data, err := json.Marshal(doc)
	require.NoError(t, err)

	var out game.GameDoc
	require.NoError(t, json.Unmarshal(data, &out))

	assert.Equal(t, "v7", out.Rapua)
	assert.Equal(t, "Round Trip Game", out.Name)
	require.Len(t, out.Structure.Children, 2)

	// First child is a location
	require.NotNil(t, out.Structure.Children[0].Location)
	assert.Equal(t, "lobby", out.Structure.Children[0].Location.Slug)

	// Second child is a group
	require.NotNil(t, out.Structure.Children[1].Group)
	assert.Equal(t, "East Wing", out.Structure.Children[1].Group.Name)
	require.Len(t, out.Structure.Children[1].Group.Children, 1)
	require.NotNil(t, out.Structure.Children[1].Group.Children[0].Location)
	assert.Equal(t, "room-a", out.Structure.Children[1].Group.Children[0].Location.Slug)
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
