package blocks

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

var ErrInvalidItemFormat = fmt.Errorf("items must be plain strings")

// YAMLExporter is implemented by blocks that can export to YAML.
type YAMLExporter interface {
	ToYAML() map[string]any
}

// FromYAML converts a YAML type + fields map into JSON data bytes suitable for DB storage.
// It adds back internal fields (generates UUIDs for option/item IDs, sets defaults).
func FromYAML(blockType string, fields map[string]any) ([]byte, error) {
	registration := blockRegistry[blockType]
	if registration == nil {
		return nil, fmt.Errorf("%w: %s", ErrBlockTypeNotFound, blockType)
	}

	// Special handling for blocks that simplify their YAML representation.
	switch blockType {
	case "quiz":
		return quizFromYAML(fields)
	case "checklist":
		return checklistFromYAML(fields)
	case "sorting":
		return sortingFromYAML(fields)
	default:
		return json.Marshal(fields)
	}
}

// toAnySlice normalises a slice value to []any regardless of whether it arrived
// as []any (from a YAML unmarshal), []map[string]any or []string (from a ToYAML call).
func toAnySlice(v any) ([]any, bool) {
	switch s := v.(type) {
	case []any:
		return s, true
	case []map[string]any:
		out := make([]any, len(s))
		for i, m := range s {
			out[i] = m
		}
		return out, true
	case []string:
		out := make([]any, len(s))
		for i, str := range s {
			out[i] = str
		}
		return out, true
	}
	return nil, false
}

// quizFromYAML converts quiz YAML (options without IDs/order) to full JSON.
func quizFromYAML(fields map[string]any) ([]byte, error) {
	if options, ok := fields["options"]; ok {
		if optionsList, ok := toAnySlice(options); ok {
			fullOptions := make([]map[string]any, 0, len(optionsList))
			for i, opt := range optionsList {
				if optMap, ok := opt.(map[string]any); ok {
					optMap["id"] = uuid.New().String()
					optMap["order"] = i
					fullOptions = append(fullOptions, optMap)
				}
			}
			fields["options"] = fullOptions
		}
	}
	return json.Marshal(fields)
}

// checklistFromYAML converts checklist YAML (items as string array) to full JSON.
// Items must be plain strings; object items are rejected.
func checklistFromYAML(fields map[string]any) ([]byte, error) {
	if items, ok := fields["items"]; ok {
		if itemsList, ok := toAnySlice(items); ok {
			fullItems := make([]map[string]any, 0, len(itemsList))
			for i, item := range itemsList {
				v, ok := item.(string)
				if !ok {
					return nil, fmt.Errorf("%w: checklist item at index %d is an object; use a plain string", ErrInvalidItemFormat, i)
				}
				fullItems = append(fullItems, map[string]any{
					"id":          uuid.New().String(),
					"description": v,
					"checked":     false,
				})
			}
			fields["items"] = fullItems
		}
	}
	return json.Marshal(fields)
}

// sortingFromYAML converts sorting YAML (items as string array) to full JSON.
// Items must be plain strings; object items are rejected.
func sortingFromYAML(fields map[string]any) ([]byte, error) {
	if items, ok := fields["items"]; ok {
		if itemsList, ok := toAnySlice(items); ok {
			fullItems := make([]map[string]any, 0, len(itemsList))
			for i, item := range itemsList {
				v, ok := item.(string)
				if !ok {
					return nil, fmt.Errorf("%w: sorting item at index %d is an object; use a plain string", ErrInvalidItemFormat, i)
				}
				fullItems = append(fullItems, map[string]any{
					"id":          uuid.New().String(),
					"description": v,
					"position":    i + 1,
				})
			}
			fields["items"] = fullItems
		}
	}
	return json.Marshal(fields)
}
