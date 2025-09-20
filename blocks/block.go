package blocks

import (
	"encoding/json"
	"fmt"
)

// BlockContext represents where a block can be used
type BlockContext string

const (
	ContextLocation BlockContext = "content" // Regular location content blocks
	// ContextStart   BlockContext = "start"   // Start pages - introductions, rules, set team name
	// ContextEnd     BlockContext = "end"     // End pages
)

// RegisteredBlock holds block metadata for the registry
type RegisteredBlock struct {
	BlockType         string
	Instance          Block
	SupportedContexts []BlockContext
}

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

type Block interface {
	// Basic Attributes Getters
	GetID() string
	GetType() string
	GetLocationID() string
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
}

type Blocks []Block

type BaseBlock struct {
	ID         string          `json:"-"`
	LocationID string          `json:"-"`
	Type       string          `json:"-"`
	Data       json.RawMessage `json:"-"`
	Order      int             `json:"-"`
	Points     int             `json:"-"`
}

// Central block registry with context support
var blockRegistry = make(map[string]*RegisteredBlock)
var contextRegistry = make(map[BlockContext][]string)

// registerBlock is an internal helper to register blocks with their contexts
func registerBlock(instance Block, contexts []BlockContext) {
	registration := &RegisteredBlock{
		BlockType:         instance.GetType(),
		Instance:          instance,
		SupportedContexts: contexts,
	}

	blockRegistry[instance.GetType()] = registration

	// Update context registry
	for _, context := range contexts {
		if contextRegistry[context] == nil {
			contextRegistry[context] = make([]string, 0)
		}
		contextRegistry[context] = append(contextRegistry[context], instance.GetType())
	}
}

// Initialize block registry
func init() {
	// Content blocks
	registerBlock(&MarkdownBlock{}, []BlockContext{ContextLocation})
	registerBlock(&AlertBlock{}, []BlockContext{ContextLocation})
	registerBlock(&ButtonBlock{}, []BlockContext{ContextLocation})
	registerBlock(&DividerBlock{}, []BlockContext{ContextLocation})
	registerBlock(&ImageBlock{}, []BlockContext{ContextLocation})
	registerBlock(&YoutubeBlock{}, []BlockContext{ContextLocation})

	// Interactive blocks
	registerBlock(&BrokerBlock{}, []BlockContext{ContextLocation})
	registerBlock(&ChecklistBlock{}, []BlockContext{ContextLocation})
	registerBlock(&ClueBlock{}, []BlockContext{ContextLocation})
	registerBlock(&PasswordBlock{}, []BlockContext{ContextLocation})
	registerBlock(&PincodeBlock{}, []BlockContext{ContextLocation})
	registerBlock(&QuizBlock{}, []BlockContext{ContextLocation})
	registerBlock(&SortingBlock{}, []BlockContext{ContextLocation})
}

// Public API functions

// GetRegisteredBlocks returns all blocks (for backward compatibility)
func GetRegisteredBlocks() Blocks {
	blocks := make(Blocks, 0, len(blockRegistry))
	for _, registration := range blockRegistry {
		blocks = append(blocks, registration.Instance)
	}
	return blocks
}

// GetBlocksForContext returns block instances available for a specific context
func GetBlocksForContext(context BlockContext) Blocks {
	blockTypes := contextRegistry[context]
	if blockTypes == nil {
		return Blocks{}
	}

	blocks := make(Blocks, 0, len(blockTypes))
	for _, blockType := range blockTypes {
		if registration := blockRegistry[blockType]; registration != nil {
			blocks = append(blocks, registration.Instance)
		}
	}

	return blocks
}

// CanBlockBeUsedInContext checks if a block type can be used in a specific context
func CanBlockBeUsedInContext(blockType string, context BlockContext) bool {
	registration := blockRegistry[blockType]
	if registration == nil {
		return false
	}

	for _, supportedContext := range registration.SupportedContexts {
		if supportedContext == context {
			return true
		}
	}

	return false
}

func CreateFromBaseBlock(baseBlock BaseBlock) (Block, error) {
	// Check if block type exists in registry
	registration := blockRegistry[baseBlock.Type]
	if registration == nil {
		return nil, fmt.Errorf("block type %s not found", baseBlock.Type)
	}

	// Use the existing constructor functions
	switch baseBlock.Type {
	case "markdown":
		return NewMarkdownBlock(baseBlock), nil
	case "divider":
		return NewDividerBlock(baseBlock), nil
	case "alert":
		return NewAlertBlock(baseBlock), nil
	case "answer":
		return NewAnswerBlock(baseBlock), nil
	case "pincode":
		return NewPincodeBlock(baseBlock), nil
	case "checklist":
		return NewChecklistBlock(baseBlock), nil
	case "youtube":
		return NewYoutubeBlock(baseBlock), nil
	case "image":
		return NewImageBlock(baseBlock), nil
	case "sorting":
		return NewSortingBlock(baseBlock), nil
	case "quiz_block":
		return NewQuizBlock(baseBlock), nil
	case "clue":
		return NewClueBlock(baseBlock), nil
	case "broker":
		return NewBrokerBlock(baseBlock), nil
	case "button":
		return NewButtonBlock(baseBlock), nil
	// case "photo":
	// 	return NewPhotoBlock(baseBlock), nil
	default:
		return nil, fmt.Errorf("block type %s not found", baseBlock.Type)
	}
}

// Example constructor functions.
func NewMarkdownBlock(base BaseBlock) *MarkdownBlock {
	return &MarkdownBlock{
		BaseBlock: base,
	}
}

func NewDividerBlock(base BaseBlock) *DividerBlock {
	return &DividerBlock{
		BaseBlock: base,
	}
}

func NewAlertBlock(base BaseBlock) *AlertBlock {
	return &AlertBlock{
		BaseBlock: base,
	}
}

func NewAnswerBlock(base BaseBlock) *PasswordBlock {
	return &PasswordBlock{
		BaseBlock: base,
	}
}

func NewPincodeBlock(base BaseBlock) *PincodeBlock {
	return &PincodeBlock{
		BaseBlock: base,
	}
}

func NewChecklistBlock(base BaseBlock) *ChecklistBlock {
	return &ChecklistBlock{
		BaseBlock: base,
	}
}

func NewYoutubeBlock(base BaseBlock) *YoutubeBlock {
	return &YoutubeBlock{
		BaseBlock: base,
	}
}

func NewImageBlock(base BaseBlock) *ImageBlock {
	return &ImageBlock{
		BaseBlock: base,
	}
}

func NewSortingBlock(base BaseBlock) *SortingBlock {
	return &SortingBlock{
		BaseBlock: base,
	}
}

func NewClueBlock(base BaseBlock) *ClueBlock {
	return &ClueBlock{
		BaseBlock: base,
	}
}

func NewBrokerBlock(base BaseBlock) *BrokerBlock {
	return &BrokerBlock{
		BaseBlock: base,
	}
}

func NewButtonBlock(base BaseBlock) *ButtonBlock {
	return &ButtonBlock{
		BaseBlock: base,
	}
}

//
// func NewPhotoBlock(base BaseBlock) *PhotoBlock {
// 	return &PhotoBlock{
// 		BaseBlock: base,
// 	}
// }
