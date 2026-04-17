package game

import (
	"encoding/json"
	"errors"
)

// ErrBlockTypeNotFound is returned when a block type is not registered.
var ErrBlockTypeNotFound = errors.New("block type not found")

// FormValueTrue is the string value "true" used in form checkbox comparisons.
const FormValueTrue = "true"

// PlayerState tracks a player's progress on a block.
type PlayerState interface {
	GetBlockID() string
	GetPlayerID() string
	GetPlayerData() json.RawMessage
	SetPlayerData(data json.RawMessage)
	IsComplete() bool
	SetComplete(complete bool)
	GetPointsAwarded() int
	SetPointsAwarded(points int)
}

// Block defines the contract for all block types.
type Block interface {
	// Basic Attributes Getters
	GetID() string
	GetType() string
	GetOwnerID() string
	GetName() string
	GetDescription() string
	GetOrder() int
	GetPoints() int
	GetIconSVG() string
	GetData() json.RawMessage

	// Data Operations
	ParseData() error
	UpdateBlockData(data map[string][]string) error

	// Validation and Points Calculation
	RequiresValidation() bool
	ValidatePlayerInput(state PlayerState, input map[string][]string) (newState PlayerState, err error)

	// Conditional visibility
	GetSets() map[string]string
	GetWhen() *WhenClause
}

// Blocks is a slice of Block.
type Blocks []Block

// BaseBlock holds the common fields shared by all block implementations.
type BaseBlock struct {
	ID      string          `json:"-"`
	OwnerID string          `json:"-"`
	Type    string          `json:"-"`
	Data    json.RawMessage `json:"-"`
	Order   int             `json:"-"`
	Points  int             `json:"-"`
	Sets    map[string]string `json:"sets,omitempty"`
	When    *WhenClause       `json:"when,omitempty"`
}

// GetSets returns the sets map for this block (var name → trigger keyword).
func (b *BaseBlock) GetSets() map[string]string { return b.Sets }

// GetWhen returns the visibility condition clause for this block.
func (b *BaseBlock) GetWhen() *WhenClause { return b.When }

// RegisteredBlock holds block metadata for the registry.
type RegisteredBlock struct {
	BlockType         string
	Instance          Block
	SupportedContexts []BlockContext
}
