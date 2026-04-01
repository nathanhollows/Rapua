package blocks

// FieldType represents the data type of a block field in its YAML representation.
type FieldType int

const (
	FieldString   FieldType = iota // Plain string value
	FieldBool                      // Boolean value
	FieldInt                       // Integer value
	FieldFloat                     // Float value
	FieldMarkdown                  // Markdown-formatted string
	FieldList                      // Ordered list; Children defines the item structure
	FieldEnum                      // String restricted to a set of allowed values (see Enum)
)

// FieldDef describes a single field in a block's YAML representation.
type FieldDef struct {
	Name     string     // YAML key name (matches json tag on the block struct)
	Type     FieldType  // Data type
	Desc     string     // Human-readable description
	Required bool       // Whether the field must be present on import
	Enum     []string   // Allowed values for FieldEnum fields
	Children []FieldDef // Sub-field definitions for FieldList items
}

// FieldSpecProvider is implemented by blocks that expose their field definitions.
// All registered block types implement this interface.
type FieldSpecProvider interface {
	FieldSpec() []FieldDef
}

// GetFieldSpec returns the field definitions for a registered block type.
// Returns nil if the block type is not registered or doesn't implement FieldSpecProvider.
func GetFieldSpec(blockType string) []FieldDef {
	registration := blockRegistry[blockType]
	if registration == nil {
		return nil
	}
	provider, ok := registration.Instance.(FieldSpecProvider)
	if !ok {
		return nil
	}
	return provider.FieldSpec()
}

// GetAllRegisteredTypes returns all registered block type strings.
func GetAllRegisteredTypes() []string {
	types := make([]string, 0, len(blockRegistry))
	for t := range blockRegistry {
		types = append(types, t)
	}
	return types
}

// GetRegisteredBlock returns the registry entry for a block type, or nil if not found.
func GetRegisteredBlock(blockType string) *RegisteredBlock {
	return blockRegistry[blockType]
}
