package game

// CodeMinter is implemented by blocks offering codes to print.
type CodeMinter interface {
	MintedCodes() []string
}

// SpecProvider is implemented by block types that provide field-level metadata.
type SpecProvider interface {
	GetSpec() BlockSpec
}

// BlockRegistry allows the linter to validate block types and contexts
// without importing the blocks package (which would create a cycle).
type BlockRegistry interface {
	IsValidType(blockType string) bool
	CanUseInContext(blockType string, ctx BlockContext) bool
	// KnownFields returns the top-level field names valid for a block type,
	// excluding the promoted fields "type", "id", and "points".
	// Returns nil if the block type is unknown or has no spec.
	KnownFields(blockType string) []string
	// IsInteractive returns true when the block type requires player input
	// (i.e. RequiresValidation returns true). Only interactive blocks may
	// carry a "sets" field; the linter warns otherwise.
	IsInteractive(blockType string) bool
	// DocSetsVars returns variable names that a specific block doc instance
	// defines. Used for blocks (e.g. choice) whose vars live in sub-fields
	// rather than a top-level "sets" map.
	DocSetsVars(blockType string, doc BlockDoc) []string
	// ValidateBlock runs block-type-specific structural lint on a block doc,
	// returning separate error and warning diagnostics.
	ValidateBlock(blockType, path string, doc BlockDoc) (errors, warnings []LintDiag)
}

// BlockDocVarsProvider is implemented by block types that expose settable
// variable names through their doc representation (e.g. per-option sets fields)
// rather than the standard top-level "sets" map.
type BlockDocVarsProvider interface {
	DocSetsVars(doc BlockDoc) []string
}

// BlockDocValidator is implemented by block types that perform structural lint
// checks beyond what the generic checkBlockDoc covers.
type BlockDocValidator interface {
	ValidateBlockDoc(path string, doc BlockDoc) (errors, warnings []LintDiag)
}

// BlockSpec describes a block type for AI-assisted game authoring.
type BlockSpec struct {
	Type         string      `json:"type"`
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	Contexts     []string    `json:"contexts"`
	SharedFields []string    `json:"shared_fields,omitempty"` // names of fields defined in FullSpec.block_shared_fields
	Fields       []FieldSpec `json:"fields"`
}

// FieldSpec describes a single field within a block.
type FieldSpec struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"` // "string", "int", "bool", "markdown", "list", "object", "enum"
	Description string      `json:"description,omitempty"`
	Required    bool        `json:"required,omitempty"`
	Default     any         `json:"default,omitempty"`
	Enum        []string    `json:"enum,omitempty"`   // for enum types
	Items       *FieldSpec  `json:"items,omitempty"`  // for list types
	Fields      []FieldSpec `json:"fields,omitempty"` // for object types
}
