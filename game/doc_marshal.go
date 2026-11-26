package game

import (
	"bytes"
	"encoding/json"
	"sort"
)

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
