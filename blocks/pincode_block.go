package blocks

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

type PincodeBlock struct {
	BaseBlock
	Prompt          string `json:"prompt"`
	Pincode         string `json:"pincode"`
	UnlockedContent string `json:"unlocked_content"`
}

type pincodeBlockData struct {
	Attempts int      `json:"attempts"`
	Guesses  []string `json:"guesses"`
}

// Basic Attributes Getters

func (b *PincodeBlock) GetID() string         { return b.ID }
func (b *PincodeBlock) GetType() string       { return "pincode" }
func (b *PincodeBlock) GetLocationID() string { return b.LocationID }
func (b *PincodeBlock) GetName() string       { return "Pincode" }
func (b *PincodeBlock) GetDescription() string {
	return "Players must enter the correct pincode to a prompt."
}
func (b *PincodeBlock) GetOrder() int  { return b.Order }
func (b *PincodeBlock) GetPoints() int { return b.Points }
func (b *PincodeBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-hash"><line x1="4" x2="20" y1="9" y2="9"/><line x1="4" x2="20" y1="15" y2="15"/><line x1="10" x2="8" y1="3" y2="21"/><line x1="16" x2="14" y1="3" y2="21"/></svg>`
}
func (b *PincodeBlock) GetAdminData() interface{} {
	return &b
}
func (b *PincodeBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations

func (b *PincodeBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *PincodeBlock) UpdateBlockData(input map[string][]string) error {
	// Points
	if input["points"] != nil {
		points, err := strconv.Atoi(input["points"][0])
		if err != nil {
			return errors.New("points must be an integer")
		}
		b.Points = points
	}
	// Prompt and Pincode
	if input["prompt"] == nil || input["pincode"] == nil {
		return errors.New("prompt and pincode are required fields")
	}
	b.Prompt = input["prompt"][0]
	b.Pincode = input["pincode"][0]
	if input["unlocked_content"] != nil {
		b.UnlockedContent = input["unlocked_content"][0]
	}
	return nil
}

// Validation and Points Calculation

func (b *PincodeBlock) RequiresValidation() bool { return true }

func (b *PincodeBlock) ValidatePlayerInput(state PlayerState, input map[string][]string) (PlayerState, error) {
	if input["pincode"] == nil {
		return state, errors.New("pincode is a required field")
	}

	if len(input["pincode"]) == 0 {
		return state, errors.New("pincode cannot be empty")
	}

	if len(input["pincode"]) < len(b.Pincode) {
		return state, errors.New("pincode length does not match the required length")
	}

	// Construct the pincode from individual characters
	var pincodeChars string
	for _, char := range input["pincode"] {
		if len(char) != 1 {
			return state, errors.New("pincode must be a single character per input")
		}
		pincodeChars = pincodeChars + char
	}

	newPlayerData := pincodeBlockData{}
	if state.GetPlayerData() != nil {
		err := json.Unmarshal(state.GetPlayerData(), &newPlayerData)
		if err != nil {
			return state, fmt.Errorf("unmarshalling player data %w", err)
		}
	}

	// Increment the number of attempts and save guesses
	newPlayerData.Attempts++
	newPlayerData.Guesses = append(newPlayerData.Guesses, input["pincode"][0])

	if pincodeChars != b.Pincode {
		// Incorrect pincode, save player data and return an error
		playerData, err := json.Marshal(newPlayerData)
		if err != nil {
			return state, errors.New("Error saving player data")
		}
		state.SetPlayerData(playerData)
		return state, nil
	}

	// Correct pincode, update state to complete
	playerData, err := json.Marshal(newPlayerData)
	if err != nil {
		return state, errors.New("Error saving player data")
	}
	state.SetPlayerData(playerData)
	state.SetComplete(true)
	state.SetPointsAwarded(b.Points)
	return state, nil
}
