package models

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// BlockDef represents a block in YAML format.
// It marshals/unmarshals as a flat map with "type" and optional "id"/"points" keys,
// plus all block-specific fields inline.
type BlockDef struct {
	ID     string         `yaml:"-"`
	Type   string         `yaml:"-"`
	Points int            `yaml:"-"`
	Fields map[string]any `yaml:"-"`
}

// MarshalYAML produces an ordered mapping: type first, then points (if set),
// then id (if set), then all block-specific fields.
func (b BlockDef) MarshalYAML() (interface{}, error) {
	node := &yaml.Node{Kind: yaml.MappingNode}

	// type always first
	appendYAMLStringPair(node, "type", b.Type)

	// points near the top
	if b.Points != 0 {
		if err := appendYAMLPair(node, "points", b.Points); err != nil {
			return nil, fmt.Errorf("marshalling field %q: %w", "points", err)
		}
	}

	// id if present
	if b.ID != "" {
		appendYAMLStringPair(node, "id", b.ID)
	}

	// block-specific fields in sorted order for deterministic output
	keys := make([]string, 0, len(b.Fields))
	for k := range b.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := appendYAMLPair(node, k, b.Fields[k]); err != nil {
			return nil, fmt.Errorf("marshalling field %q: %w", k, err)
		}
	}

	return node, nil
}

// UnmarshalYAML parses a flat map into BlockDef, extracting type/id/points
// and putting everything else into Fields.
func (b *BlockDef) UnmarshalYAML(value *yaml.Node) error {
	var raw map[string]any
	if err := value.Decode(&raw); err != nil {
		return fmt.Errorf("decoding block: %w", err)
	}

	if t, ok := raw["type"]; ok {
		b.Type, _ = t.(string)
		delete(raw, "type")
	}
	if id, ok := raw["id"]; ok {
		b.ID, _ = id.(string)
		delete(raw, "id")
	}
	if p, ok := raw["points"]; ok {
		switch v := p.(type) {
		case int:
			b.Points = v
		case float64:
			b.Points = int(v)
		}
		delete(raw, "points")
	}

	b.Fields = raw
	return nil
}

// appendYAMLStringPair adds a string key-value pair to a mapping node.
func appendYAMLStringPair(node *yaml.Node, key, value string) {
	node.Content = append(node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Value: value},
	)
}

// appendYAMLPair adds a key and any-typed value to a mapping node.
func appendYAMLPair(node *yaml.Node, key string, value any) error {
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}

	// Marshal the value to YAML, then unmarshal into a Node to get proper typing
	data, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshalling value: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("unmarshalling value node: %w", err)
	}

	var valNode *yaml.Node
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		valNode = doc.Content[0]
	} else {
		valNode = &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%v", value)}
	}

	node.Content = append(node.Content, keyNode, valNode)
	return nil
}
