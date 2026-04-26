package game

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// MarshalJSON serializes ChildDoc as {"location": ...} or {"group": ...}.
func (c ChildDoc) MarshalJSON() ([]byte, error) {
	if c.Location != nil && c.Group != nil {
		return nil, errors.New("ChildDoc: both Location and Group are set")
	}
	if c.Location != nil {
		return json.Marshal(map[string]any{"location": c.Location})
	}
	if c.Group != nil {
		return json.Marshal(map[string]any{"group": c.Group})
	}
	return nil, errors.New("ChildDoc: neither Location nor Group is set")
}

// UnmarshalJSON deserializes {"location": ...} or {"group": ...} into ChildDoc.
func (c *ChildDoc) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if locRaw, ok := raw["location"]; ok {
		c.Location = &LocationDoc{}
		return json.Unmarshal(locRaw, c.Location)
	}

	if grpRaw, ok := raw["group"]; ok {
		c.Group = &GroupDoc{}
		return json.Unmarshal(grpRaw, c.Group)
	}

	return fmt.Errorf("ChildDoc: expected key \"location\" or \"group\", got %v", keysOf(raw))
}

// MarshalJSON serializes BlockDoc with "type" first, "id" second (if present),
// then remaining keys in alphabetical order.
func (b BlockDoc) MarshalJSON() ([]byte, error) {
	// Collect and sort the remaining keys
	rest := make([]string, 0, len(b))
	for k := range b {
		if k == "type" || k == "id" {
			continue
		}
		rest = append(rest, k)
	}
	sort.Strings(rest)

	var buf bytes.Buffer
	buf.WriteByte('{')

	first := true
	writeKey := func(k string) error {
		if !first {
			buf.WriteByte(',')
		}
		first = false
		keyBytes, err := json.Marshal(k)
		if err != nil {
			return err
		}
		buf.Write(keyBytes)
		buf.WriteByte(':')
		valBytes, err := json.Marshal(b[k])
		if err != nil {
			return err
		}
		buf.Write(valBytes)
		return nil
	}

	if _, hasType := b["type"]; hasType {
		if err := writeKey("type"); err != nil {
			return nil, err
		}
	}
	if _, hasID := b["id"]; hasID {
		if err := writeKey("id"); err != nil {
			return nil, err
		}
	}
	for _, k := range rest {
		if err := writeKey(k); err != nil {
			return nil, err
		}
	}

	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func keysOf(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
