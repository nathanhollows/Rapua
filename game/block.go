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
	// SupportsVariableSets returns true when the block can carry a "sets" field.
	// Only blocks that fire sets triggers on validation should return true;
	// the linter warns if a "sets" field appears on a block that returns false.
	SupportsVariableSets() bool

	// Conditional visibility
	GetSets() []string
	GetWhen() *WhenClause
	SetWhen(when *WhenClause)
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
	Sets    []string        `json:"sets,omitempty"`
	When    *WhenClause     `json:"when,omitempty"`
}

// SupportsVariableSets returns false by default; interactive blocks override this.
func (b *BaseBlock) SupportsVariableSets() bool { return false }

// GetSets returns the variable names this block sets on completion.
func (b *BaseBlock) GetSets() []string { return b.Sets }

// GetWhen returns the visibility condition clause for this block.
func (b *BaseBlock) GetWhen() *WhenClause { return b.When }

// SetWhen sets the visibility condition clause for this block.
func (b *BaseBlock) SetWhen(when *WhenClause) { b.When = when }

// RegisteredBlock holds block metadata for the registry.
type RegisteredBlock struct {
	BlockType string
	// Prototype is a zero-value exemplar of the block type, kept so the
	// registry can interrogate the type itself (specs, doc vars, capabilities).
	// It carries no per-block data and is never rendered.
	Prototype         Block
	SupportedContexts []BlockContext
}
