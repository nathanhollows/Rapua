package blocks

import "encoding/json"

// Export unexported types for testing in blocks_test package
// These are only available during test compilation

// MockPlayerState is a mock implementation of PlayerState for testing.
type MockPlayerState struct {
	BlockID       string
	PlayerID      string
	PlayerData    json.RawMessage
	IsCompleteVal bool
	PointsAwarded int
}

func (m *MockPlayerState) GetBlockID() string                 { return m.BlockID }
func (m *MockPlayerState) GetPlayerID() string                { return m.PlayerID }
func (m *MockPlayerState) GetPlayerData() json.RawMessage     { return m.PlayerData }
func (m *MockPlayerState) SetPlayerData(data json.RawMessage) { m.PlayerData = data }
func (m *MockPlayerState) IsComplete() bool                   { return m.IsCompleteVal }
func (m *MockPlayerState) SetComplete(complete bool)          { m.IsCompleteVal = complete }
func (m *MockPlayerState) GetPointsAwarded() int              { return m.PointsAwarded }
func (m *MockPlayerState) SetPointsAwarded(points int)        { m.PointsAwarded = points }

type PincodeBlockData = pincodeBlockData
type ChecklistPlayerData = checklistPlayerData
type BrokerBlockData = brokerBlockData
type ClueBlockData = clueBlockData
type PhotoBlockData = photoBlockData
type RatingBlockData = ratingBlockData

// Export unexported functions for testing.
var DeterministicShuffle = deterministicShuffle

// CalculatePoints exposes the calculatePoints method for testing.
func (b *QuizBlock) CalculatePoints(selectedOptions []string) (int, bool) {
	return b.calculatePoints(selectedOptions)
}

// OrderIsCorrect exposes the orderIsCorrect method for testing.
func (b *SortingBlock) OrderIsCorrect(playerOrder []string) bool {
	return b.orderIsCorrect(playerOrder)
}

// CalculateCorrectItemCorrectPlacePoints exposes the calculateCorrectItemCorrectPlacePoints method for testing.
func (b *SortingBlock) CalculateCorrectItemCorrectPlacePoints(playerOrder []string) int {
	return b.calculateCorrectItemCorrectPlacePoints(playerOrder)
}
