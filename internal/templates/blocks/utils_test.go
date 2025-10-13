package blocks

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetBrokerInfoReceived(t *testing.T) {
	tests := []struct {
		name       string
		playerData json.RawMessage
		expected   string
	}{
		{
			name:       "nil player data",
			playerData: nil,
			expected:   "No information purchased yet.",
		},
		{
			name:       "empty player data",
			playerData: json.RawMessage("{}"),
			expected:   "No information purchased yet.",
		},
		{
			name:       "has not purchased",
			playerData: json.RawMessage(`{"has_purchased": false, "info_received": ""}`),
			expected:   "No information purchased yet.",
		},
		{
			name: "has purchased with info",
			playerData: json.RawMessage(
				`{"points_paid": 15, "info_received": "Secret information revealed!", "has_purchased": true}`,
			),
			expected: "Secret information revealed!",
		},
		{
			name:       "invalid JSON",
			playerData: json.RawMessage(`{invalid json`),
			expected:   "Error loading purchased information.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock player state
			mockState := &mockPlayerState{
				blockID:    "test-block",
				playerID:   "test-player",
				playerData: tt.playerData,
			}

			result := getBrokerInfoReceived(mockState)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Mock implementation for testing.
type mockPlayerState struct {
	blockID       string
	playerID      string
	playerData    json.RawMessage
	complete      bool
	pointsAwarded int
}

func (m *mockPlayerState) GetBlockID() string                 { return m.blockID }
func (m *mockPlayerState) GetPlayerID() string                { return m.playerID }
func (m *mockPlayerState) GetPlayerData() json.RawMessage     { return m.playerData }
func (m *mockPlayerState) SetPlayerData(data json.RawMessage) { m.playerData = data }
func (m *mockPlayerState) IsComplete() bool                   { return m.complete }
func (m *mockPlayerState) SetComplete(complete bool)          { m.complete = complete }
func (m *mockPlayerState) GetPointsAwarded() int              { return m.pointsAwarded }
func (m *mockPlayerState) SetPointsAwarded(points int)        { m.pointsAwarded = points }
