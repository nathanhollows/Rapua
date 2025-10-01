package blocks

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
)

type BrokerBlock struct {
	BaseBlock
	Prompt           string            `json:"prompt"`
	DefaultInfo      string            `json:"default_info"`
	InformationTiers []InformationTier `json:"information_tiers"`
}

type InformationTier struct {
	PointsRequired int    `json:"points_required"`
	Content        string `json:"content"`
}

type brokerBlockData struct {
	PointsPaid   int    `json:"points_paid"`
	InfoReceived string `json:"info_received"`
	HasPurchased bool   `json:"has_purchased"`
}

// Basic Attributes Getters

func (b *BrokerBlock) GetID() string         { return b.ID }
func (b *BrokerBlock) GetType() string       { return "broker" }
func (b *BrokerBlock) GetLocationID() string { return b.LocationID }
func (b *BrokerBlock) GetName() string       { return "Broker" }
func (b *BrokerBlock) GetDescription() string {
	return "Players can pay points to unlock information. The more they pay, the better information they might receive."
}
func (b *BrokerBlock) GetOrder() int  { return b.Order }
func (b *BrokerBlock) GetPoints() int { return b.Points }
func (b *BrokerBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-handshake"><path d="m11 17 2 2a1 1 0 1 0 3-3"/><path d="m14 14 2.5 2.5a1 1 0 1 0 3-3l-3.88-3.88a3 3 0 0 0-4.24 0l-.88.88a1 1 0 1 1-3-3l2.81-2.81a5.79 5.79 0 0 1 7.06-.87l.47.28a2 2 0 0 0 1.42.25L21 4"/><path d="m21 3 1 11h-2"/><path d="M3 3 2 14l6.5 6.5a1 1 0 1 0 3-3"/><path d="M3 4h8"/></svg>`
}
func (b *BrokerBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations

func (b *BrokerBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *BrokerBlock) UpdateBlockData(input map[string][]string) error {
	// Broker blocks don't use completion bonus points
	b.Points = 0

	// Prompt text
	if prompt, exists := input["prompt"]; exists && len(prompt) > 0 {
		b.Prompt = prompt[0]
	}

	// Default info (what they get for 0 points)
	if defaultInfo, exists := input["default_info"]; exists && len(defaultInfo) > 0 {
		b.DefaultInfo = defaultInfo[0]
	}

	// Parse information tiers from unnumbered arrays
	b.InformationTiers = []InformationTier{}

	// Get the tier_points and tier_content arrays
	pointsInputs, hasPoints := input["tier_points"]
	contentInputs, hasContent := input["tier_content"]

	if hasPoints && hasContent {
		// Process pairs in order - use the shorter array length to avoid index errors
		maxTiers := len(pointsInputs)
		if len(contentInputs) < maxTiers {
			maxTiers = len(contentInputs)
		}

		for i := range maxTiers {
			pointsStr := pointsInputs[i]
			contentStr := contentInputs[i]

			// Skip if either field is empty
			if pointsStr == "" || contentStr == "" {
				continue
			}

			points, err := strconv.Atoi(pointsStr)
			if err != nil {
				return errors.New("tier points must be integers")
			}

			// Skip tiers with 0 or negative points (reserved for default)
			if points <= 0 {
				continue
			}

			b.InformationTiers = append(b.InformationTiers, InformationTier{
				PointsRequired: points,
				Content:        contentStr,
			})
		}
	}

	// Sort tiers by points required (ascending)
	sort.Slice(b.InformationTiers, func(i, j int) bool {
		return b.InformationTiers[i].PointsRequired < b.InformationTiers[j].PointsRequired
	})

	return nil
}

// Validation and Points Calculation

func (b *BrokerBlock) RequiresValidation() bool { return true }

func (b *BrokerBlock) ValidatePlayerInput(state PlayerState, input map[string][]string) (PlayerState, error) {
	newState := state

	// Parse current player data if it exists
	var playerData brokerBlockData
	if state.GetPlayerData() != nil {
		if err := json.Unmarshal(state.GetPlayerData(), &playerData); err != nil {
			return state, errors.New("failed to parse player data")
		}
	}

	// Check if player is trying to make a purchase
	if pointsBidInput, exists := input["points_bid"]; exists && len(pointsBidInput) > 0 && pointsBidInput[0] != "" {
		pointsBid, err := strconv.Atoi(pointsBidInput[0])
		if err != nil {
			return state, errors.New("points bid must be an integer")
		}

		// Ensure non-negative bid
		if pointsBid < 0 {
			pointsBid = 0
		}

		// Determine what information to provide
		var infoToProvide string
		var actualPointsCharged int

		if pointsBid == 0 {
			// Always provide default info for 0 points
			infoToProvide = b.DefaultInfo
			actualPointsCharged = 0
		} else {
			// Find the best tier they can afford
			bestTier := InformationTier{}
			found := false

			for _, tier := range b.InformationTiers {
				if pointsBid >= tier.PointsRequired {
					bestTier = tier
					found = true
				} else {
					break // Since tiers are sorted, we can stop here
				}
			}

			if found {
				infoToProvide = bestTier.Content
				actualPointsCharged = pointsBid // Charge exactly what they bid
			} else {
				// They bid something but not enough for any tier
				infoToProvide = b.DefaultInfo
				actualPointsCharged = pointsBid // Still charge what they bid
			}
		}

		// Update player data
		playerData.PointsPaid = actualPointsCharged
		playerData.InfoReceived = infoToProvide
		playerData.HasPurchased = true

		// Save updated player data
		newPlayerData, err := json.Marshal(playerData)
		if err != nil {
			return state, errors.New("failed to save player data")
		}
		newState.SetPlayerData(newPlayerData)

		// Mark as complete and deduct points
		newState.SetComplete(true)
		newState.SetPointsAwarded(-actualPointsCharged) // Deduct exactly what they bid
	}

	return newState, nil
}
