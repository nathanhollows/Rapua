package game

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

func (c ChildDoc) MarshalJSON() ([]byte, error) {
	set := 0
	if c.Group != nil {
		set++
	}
	if c.Objective != nil {
		set++
	}
	if set > 1 {
		return nil, errors.New("ChildDoc: more than one of Group, Objective is set")
	}
	if c.Group != nil {
		return json.Marshal(map[string]any{"group": c.Group})
	}
	if c.Objective != nil {
		return json.Marshal(map[string]any{"objective": c.Objective})
	}
	return nil, errors.New("ChildDoc: none of Group, Objective is set")
}

func (c *ChildDoc) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	known := 0
	for _, key := range []string{"group", "objective"} {
		if _, ok := raw[key]; ok {
			known++
		}
	}
	if known > 1 {
		return fmt.Errorf("ChildDoc: expected exactly one of \"group\", \"objective\", got %v", keysOf(raw))
	}

	if grpRaw, ok := raw["group"]; ok {
		c.Group = &GroupDoc{}
		return json.Unmarshal(grpRaw, c.Group)
	}

	if objRaw, ok := raw["objective"]; ok {
		c.Objective = &ObjectiveDoc{}
		return json.Unmarshal(objRaw, c.Objective)
	}

	return fmt.Errorf("ChildDoc: expected key \"group\", or \"objective\", got %v", keysOf(raw))
}

func (b BlockDoc) MarshalJSON() ([]byte, error) {
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
