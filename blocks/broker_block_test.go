package blocks

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrokerBlock_Getters(t *testing.T) {
	block := BrokerBlock{
		BaseBlock: BaseBlock{
			ID:         "test-broker-id",
			LocationID: "location-456",
			Order:      4,
			Points:     0, // Broker blocks don't use completion bonus
		},
		Prompt:      "The merchant eyes you suspiciously...",
		DefaultInfo: "I don't know anything.",
		InformationTiers: []InformationTier{
			{PointsRequired: 10, Content: "Basic info here"},
			{PointsRequired: 25, Content: "Premium info here"},
		},
	}

	assert.Equal(t, "Broker", block.GetName())
	assert.Equal(t, "broker", block.GetType())
	assert.Equal(t, "test-broker-id", block.GetID())
	assert.Equal(t, "location-456", block.GetLocationID())
	assert.Equal(t, 4, block.GetOrder())
	assert.Equal(t, 0, block.GetPoints())
	assert.Contains(t, block.GetDescription(), "pay points")
	assert.Contains(t, block.GetIconSVG(), "handshake")
}

func TestBrokerBlock_ParseData(t *testing.T) {
	data := `{
		"prompt":"Test prompt",
		"default_info":"Default response",
		"information_tiers":[
			{"points_required":10,"content":"Tier 1 info"},
			{"points_required":20,"content":"Tier 2 info"}
		]
	}`
	block := BrokerBlock{
		BaseBlock: BaseBlock{
			Data: json.RawMessage(data),
		},
	}

	err := block.ParseData()
	require.NoError(t, err)
	assert.Equal(t, "Test prompt", block.Prompt)
	assert.Equal(t, "Default response", block.DefaultInfo)
	assert.Len(t, block.InformationTiers, 2)
	assert.Equal(t, 10, block.InformationTiers[0].PointsRequired)
	assert.Equal(t, "Tier 1 info", block.InformationTiers[0].Content)
}

func TestBrokerBlock_UpdateBlockData(t *testing.T) {
	block := BrokerBlock{}
	input := map[string][]string{
		"points":       {"0"}, // This will be ignored
		"prompt":       {"Merchant greeting"},
		"default_info": {"Basic response"},
		"tier_points":  {"10", "25", ""}, // Array format - empty tier should be ignored
		"tier_content": {"First tier info", "Second tier info", ""},
	}

	err := block.UpdateBlockData(input)
	require.NoError(t, err)
	
	assert.Equal(t, 0, block.Points) // Broker blocks always have 0 completion bonus
	assert.Equal(t, "Merchant greeting", block.Prompt)
	assert.Equal(t, "Basic response", block.DefaultInfo)
	assert.Len(t, block.InformationTiers, 2)
	
	// Verify tiers are sorted by points
	assert.Equal(t, 10, block.InformationTiers[0].PointsRequired)
	assert.Equal(t, "First tier info", block.InformationTiers[0].Content)
	assert.Equal(t, 25, block.InformationTiers[1].PointsRequired)
	assert.Equal(t, "Second tier info", block.InformationTiers[1].Content)
}

func TestBrokerBlock_UpdateBlockData_IgnoresNegativeTiers(t *testing.T) {
	block := BrokerBlock{}
	input := map[string][]string{
		"tier_points":  {"0", "-5", "10"},   // First two should be ignored
		"tier_content": {"Zero tier", "Negative tier", "Valid tier"},
	}

	err := block.UpdateBlockData(input)
	require.NoError(t, err)
	
	// Should only have the valid tier
	assert.Len(t, block.InformationTiers, 1)
	assert.Equal(t, 10, block.InformationTiers[0].PointsRequired)
	assert.Equal(t, "Valid tier", block.InformationTiers[0].Content)
}

func TestBrokerBlock_RequiresValidation(t *testing.T) {
	block := BrokerBlock{}
	assert.True(t, block.RequiresValidation())
}

func TestBrokerBlock_ValidatePlayerInput(t *testing.T) {
	tests := []struct {
		name                string
		block               BrokerBlock
		pointsBid           string
		expectedPoints      int  // What they should be charged (negative = deducted)
		expectedComplete    bool
		expectedInfoContent string
	}{
		{
			name: "zero points gets default info",
			block: BrokerBlock{
				BaseBlock: BaseBlock{Points: 0}, // No completion bonus
				DefaultInfo: "Basic info",
				InformationTiers: []InformationTier{
					{PointsRequired: 10, Content: "Premium info"},
				},
			},
			pointsBid:           "0",
			expectedPoints:      0,  // No points charged or awarded
			expectedComplete:    true,
			expectedInfoContent: "Basic info",
		},
		{
			name: "bid meets tier requirement",
			block: BrokerBlock{
				BaseBlock: BaseBlock{Points: 0},
				DefaultInfo: "Basic info",
				InformationTiers: []InformationTier{
					{PointsRequired: 10, Content: "Tier 1 info"},
					{PointsRequired: 20, Content: "Tier 2 info"},
				},
			},
			pointsBid:           "15",
			expectedPoints:      -15, // Exactly what they bid
			expectedComplete:    true,
			expectedInfoContent: "Tier 1 info",
		},
		{
			name: "bid exceeds highest tier",
			block: BrokerBlock{
				BaseBlock: BaseBlock{Points: 0},
				DefaultInfo: "Basic info",
				InformationTiers: []InformationTier{
					{PointsRequired: 10, Content: "Tier 1 info"},
					{PointsRequired: 20, Content: "Tier 2 info"},
				},
			},
			pointsBid:           "30",
			expectedPoints:      -30, // Pay exactly what they bid
			expectedComplete:    true,
			expectedInfoContent: "Tier 2 info",
		},
		{
			name: "bid insufficient for any tier",
			block: BrokerBlock{
				BaseBlock: BaseBlock{Points: 0},
				DefaultInfo: "Not enough payment",
				InformationTiers: []InformationTier{
					{PointsRequired: 15, Content: "Premium info"},
				},
			},
			pointsBid:           "5",
			expectedPoints:      -5, // Still charged, but get default
			expectedComplete:    true,
			expectedInfoContent: "Not enough payment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialState := &mockPlayerState{
				blockID:  "block-1",
				playerID: "player-1",
			}

			input := map[string][]string{
				"points_bid": {tt.pointsBid},
			}

			newState, err := tt.block.ValidatePlayerInput(initialState, input)
			require.NoError(t, err)
			
			assert.Equal(t, tt.expectedComplete, newState.IsComplete())
			assert.Equal(t, tt.expectedPoints, newState.GetPointsAwarded())
			
			// Verify player data is set correctly
			var playerData brokerBlockData
			err = json.Unmarshal(newState.GetPlayerData(), &playerData)
			require.NoError(t, err)
			assert.True(t, playerData.HasPurchased)
			assert.Equal(t, tt.expectedInfoContent, playerData.InfoReceived)
		})
	}
}

func TestBrokerBlock_ValidatePlayerInput_InvalidBid(t *testing.T) {
	block := BrokerBlock{}
	state := &mockPlayerState{blockID: "block-1", playerID: "player-1"}
	
	input := map[string][]string{
		"points_bid": {"invalid"},
	}

	_, err := block.ValidatePlayerInput(state, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "points bid must be an integer")
}

func TestBrokerBlock_ValidatePlayerInput_NegativeBid(t *testing.T) {
	block := BrokerBlock{
		DefaultInfo: "Default response",
	}
	state := &mockPlayerState{blockID: "block-1", playerID: "player-1"}
	
	input := map[string][]string{
		"points_bid": {"-10"},
	}

	newState, err := block.ValidatePlayerInput(state, input)
	require.NoError(t, err)
	
	// Negative bids should be treated as 0
	assert.Equal(t, 0, newState.GetPointsAwarded())
	
	var playerData brokerBlockData
	err = json.Unmarshal(newState.GetPlayerData(), &playerData)
	require.NoError(t, err)
	assert.Equal(t, 0, playerData.PointsPaid)
}

func TestBrokerBlock_GetData(t *testing.T) {
	block := BrokerBlock{
		BaseBlock: BaseBlock{
			ID:     "test-id",
			Points: 0,
		},
		Prompt:      "Test prompt",
		DefaultInfo: "Default info",
		InformationTiers: []InformationTier{
			{PointsRequired: 15, Content: "Premium content"},
		},
	}

	data := block.GetData()
	assert.NotNil(t, data)
	
	// Verify we can unmarshal the data
	var unmarshaled BrokerBlock
	err := json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, "Test prompt", unmarshaled.Prompt)
	assert.Equal(t, "Default info", unmarshaled.DefaultInfo)
	assert.Len(t, unmarshaled.InformationTiers, 1)
}