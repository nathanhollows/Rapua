package game

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
}

// BlockSpec describes a block type for AI-assisted game authoring.
type BlockSpec struct {
	Type        string      `json:"type"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Contexts    []string    `json:"contexts"`
	Fields      []FieldSpec `json:"fields"`
}

// FieldSpec describes a single field within a block.
type FieldSpec struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`        // "string", "int", "bool", "markdown", "list", "object", "enum"
	Description string      `json:"description,omitempty"`
	Required    bool        `json:"required,omitempty"`
	Default     any         `json:"default,omitempty"`
	Enum        []string    `json:"enum,omitempty"`  // for enum types
	Items       *FieldSpec  `json:"items,omitempty"` // for list types
	Fields      []FieldSpec `json:"fields,omitempty"` // for object types
}
