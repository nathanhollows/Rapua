package blocks

import (
	"fmt"
	"slices"

	"github.com/nathanhollows/Rapua/v7/game"
)

// BlockContext and related type aliases re-export game/ vocabulary so callers don't need to change imports.
type (
	BlockContext    = game.BlockContext
	Block           = game.Block
	BaseBlock       = game.BaseBlock
	PlayerState     = game.PlayerState
	RegisteredBlock = game.RegisteredBlock
	Blocks          = game.Blocks
)

// ErrBlockTypeNotFound is returned when a block type is not registered.
var ErrBlockTypeNotFound = game.ErrBlockTypeNotFound

// FormValueTrue is the string value "true" used in form checkbox comparisons.
const FormValueTrue = game.FormValueTrue

// BlockContext constants re-exported from game/.
const (
	ContextLocationContent = game.ContextLocationContent
	ContextNavigation      = game.ContextNavigation
	ContextStart           = game.ContextStart
	ContextFinish          = game.ContextFinish
)

//nolint:gochecknoglobals // Central block registry pattern requires package-level state
var (
	blockRegistry   = make(map[string]*RegisteredBlock)
	contextRegistry = make(map[BlockContext][]string)
)

// registerBlock is an internal helper to register blocks with their contexts.
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

//nolint:gochecknoinits // Block registry initialization requires init for package-level setup
func init() {
	// Content blocks
	registerBlock(
		&MarkdownBlock{},
		[]BlockContext{ContextLocationContent, ContextNavigation, ContextFinish, ContextStart},
	)
	registerBlock(&AlertBlock{}, []BlockContext{ContextLocationContent, ContextFinish, ContextStart})
	registerBlock(&ButtonBlock{}, []BlockContext{ContextLocationContent, ContextFinish, ContextStart})
	registerBlock(&DividerBlock{}, []BlockContext{ContextLocationContent, ContextFinish, ContextStart})
	registerBlock(&HeaderBlock{}, []BlockContext{ContextLocationContent, ContextStart, ContextFinish})
	registerBlock(
		&ImageBlock{},
		[]BlockContext{ContextLocationContent, ContextNavigation, ContextFinish, ContextStart},
	)
	registerBlock(&MapBlock{}, []BlockContext{ContextLocationContent, ContextNavigation, ContextFinish, ContextStart})
	registerBlock(&RandomClueBlock{}, []BlockContext{ContextNavigation})
	registerBlock(
		&ToggleTextBlock{},
		[]BlockContext{ContextLocationContent, ContextNavigation, ContextStart, ContextFinish},
	)
	registerBlock(&YoutubeBlock{}, []BlockContext{ContextLocationContent, ContextFinish, ContextStart})

	// Interactive blocks
	registerBlock(&BrokerBlock{}, []BlockContext{ContextLocationContent, ContextNavigation})
	registerBlock(&ChecklistBlock{}, []BlockContext{ContextLocationContent, ContextStart})
	registerBlock(&ClueBlock{}, []BlockContext{ContextLocationContent, ContextNavigation})
	registerBlock(&PasswordBlock{}, []BlockContext{ContextLocationContent})
	registerBlock(&PhotoBlock{}, []BlockContext{ContextLocationContent, ContextFinish})
	registerBlock(&PincodeBlock{}, []BlockContext{ContextLocationContent})
	registerBlock(&QuizBlock{}, []BlockContext{ContextLocationContent})
	registerBlock(&RatingBlock{}, []BlockContext{ContextLocationContent, ContextFinish})
	registerBlock(&SortingBlock{}, []BlockContext{ContextLocationContent})

	registerBlock(&TaskBlock{}, []BlockContext{ContextNavigation})

	// System blocks
	registerBlock(&GameStatusAlertBlock{}, []BlockContext{ContextStart})
	registerBlock(&StartGameButtonBlock{}, []BlockContext{ContextStart})
	registerBlock(&TeamNameChangerBlock{}, []BlockContext{ContextStart})
}

// Public API functions

// GetBlocksForContext returns block instances available for a specific context.
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

// CanBlockBeUsedInContext checks if a block type can be used in a specific context.
func CanBlockBeUsedInContext(blockType string, context BlockContext) bool {
	registration := blockRegistry[blockType]
	if registration == nil {
		return false
	}

	if slices.Contains(registration.SupportedContexts, context) {
		return true
	}

	return false
}

// Registry returns a game.BlockRegistry backed by this package's block registry.
// Used by the linter and import service so they don't need to import blocks/ directly.
func Registry() game.BlockRegistry {
	return &registryImpl{}
}

// GetRegisteredBlocks returns all registered block instances (one per type).
func GetRegisteredBlocks() []RegisteredBlock {
	out := make([]RegisteredBlock, 0, len(blockRegistry))
	for _, reg := range blockRegistry {
		out = append(out, *reg)
	}
	return out
}

// registryImpl implements game.BlockRegistry using the package-level registries.
type registryImpl struct{}

func (r *registryImpl) IsValidType(blockType string) bool {
	return blockRegistry[blockType] != nil
}

func (r *registryImpl) CanUseInContext(blockType string, ctx BlockContext) bool {
	return CanBlockBeUsedInContext(blockType, ctx)
}

func (r *registryImpl) KnownFields(blockType string) []string {
	reg := blockRegistry[blockType]
	if reg == nil {
		return nil
	}
	sp, ok := reg.Instance.(game.SpecProvider)
	if !ok {
		return nil
	}
	spec := sp.GetSpec()
	names := make([]string, 0, len(spec.Fields))
	for _, f := range spec.Fields {
		names = append(names, f.Name)
	}
	return names
}

func CreateFromBaseBlock(baseBlock BaseBlock) (Block, error) { //nolint:funlen
	// Check if block type exists in registry
	registration := blockRegistry[baseBlock.Type]
	if registration == nil {
		return nil, fmt.Errorf("%w: %s", ErrBlockTypeNotFound, baseBlock.Type)
	}

	// Use the existing constructor functions
	switch baseBlock.Type {
	case "text":
		return NewMarkdownBlock(baseBlock), nil
	case "divider":
		return NewDividerBlock(baseBlock), nil
	case "alert":
		return NewAlertBlock(baseBlock), nil
	case "password":
		return NewAnswerBlock(baseBlock), nil
	case "pincode":
		return NewPincodeBlock(baseBlock), nil
	case checklistBlockType:
		return NewChecklistBlock(baseBlock), nil
	case "youtube":
		return NewYoutubeBlock(baseBlock), nil
	case "image":
		return NewImageBlock(baseBlock), nil
	case sortingBlockType:
		return NewSortingBlock(baseBlock), nil
	case quizBlockType:
		return NewQuizBlock(baseBlock), nil
	case "clue":
		return NewClueBlock(baseBlock), nil
	case "broker":
		return NewBrokerBlock(baseBlock), nil
	case "button":
		return NewButtonBlock(baseBlock), nil
	case "random_clue":
		return NewRandomClueBlock(baseBlock), nil
	case "photo":
		return NewPhotoBlock(baseBlock), nil
	case "header":
		return NewHeaderBlock(baseBlock), nil
	case "team_name":
		return NewTeamNameChangerBlock(baseBlock), nil
	case "game_status":
		return NewGameStatusAlertBlock(baseBlock), nil
	case "start_button":
		return NewStartGameButtonBlock(baseBlock), nil
	case "task":
		return NewTaskBlock(baseBlock), nil
	case "rating":
		return NewRatingBlock(baseBlock), nil
	case "toggle_text":
		return NewToggleTextBlock(baseBlock), nil
	case mapBlockType:
		return NewMapBlock(baseBlock), nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrBlockTypeNotFound, baseBlock.Type)
	}
}

// NewMarkdownBlock creates a new markdown block instance.
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

func NewRandomClueBlock(base BaseBlock) *RandomClueBlock {
	return &RandomClueBlock{
		BaseBlock: base,
	}
}

func NewPhotoBlock(base BaseBlock) *PhotoBlock {
	return &PhotoBlock{
		BaseBlock: base,
	}
}

func NewHeaderBlock(base BaseBlock) *HeaderBlock {
	return &HeaderBlock{
		BaseBlock: base,
	}
}

func NewTeamNameChangerBlock(base BaseBlock) *TeamNameChangerBlock {
	return &TeamNameChangerBlock{
		BaseBlock: base,
	}
}

func NewGameStatusAlertBlock(base BaseBlock) *GameStatusAlertBlock {
	return &GameStatusAlertBlock{
		BaseBlock:     base,
		ShowCountdown: true,
	}
}

func NewStartGameButtonBlock(base BaseBlock) *StartGameButtonBlock {
	return &StartGameButtonBlock{
		BaseBlock: base,
	}
}

func NewTaskBlock(base BaseBlock) *TaskBlock {
	return &TaskBlock{
		BaseBlock:   base,
		LinkThrough: true,
	}
}

const defaultMaxRating = 5

func NewRatingBlock(base BaseBlock) *RatingBlock {
	return &RatingBlock{
		BaseBlock: base,
		MaxRating: defaultMaxRating,
	}
}

func NewToggleTextBlock(base BaseBlock) *ToggleTextBlock {
	return &ToggleTextBlock{
		BaseBlock: base,
	}
}

func NewMapBlock(base BaseBlock) *MapBlock {
	return &MapBlock{
		BaseBlock: base,
	}
}
