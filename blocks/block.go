package blocks

import (
	"encoding/json"
	"fmt"
)

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

var registeredBlocks = Blocks{
	// Content blocks
	&MarkdownBlock{}, // Text is always first
	&AlertBlock{},
	&DividerBlock{},
	&ImageBlock{},
	&YoutubeBlock{},
	// Interactive blocks
	&BrokerBlock{},
	&ChecklistBlock{},
	&ClueBlock{},
	&PasswordBlock{},
	&PincodeBlock{},
	// &PhotoBlock{},
	&QuizBlock{},
	&SortingBlock{},
}

func GetRegisteredBlocks() Blocks {
	return registeredBlocks
}

func CreateFromBaseBlock(baseBlock BaseBlock) (Block, error) {
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

//
// func NewPhotoBlock(base BaseBlock) *PhotoBlock {
// 	return &PhotoBlock{
// 		BaseBlock: base,
// 	}
// }
