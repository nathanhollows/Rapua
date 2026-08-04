package game

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// SetsField holds variable assignments keyed by name: {"var_name": "value"}.
//
// Values are stored as strings. When clauses coerce numerically where both
// sides parse as numbers, so a numeric literal in the document round-trips
// through the var store as its decimal text.
type SetsField map[string]string

// UnmarshalJSON implements json.Unmarshaler. Scalar values are stringified, so
// numbers and booleans are accepted alongside strings:
//
//	{"var_name": "value", "score": 40}      → {"var_name": "value", "score": "40"}
func (s *SetsField) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		return nil
	}

	// Decode values as any so numbers and booleans are accepted and stringified
	// rather than rejected with a misleading type error.
	var raw map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return fmt.Errorf(`sets: must be an object {"name": "value"}: %w`, err)
	}

	out := make(SetsField, len(raw))
	for name, v := range raw {
		val, err := setsValueToString(v)
		if err != nil {
			return fmt.Errorf("sets: %q: %w", name, err)
		}
		out[name] = val
	}
	*s = out
	return nil
}

// setsValueToString renders a scalar JSON value as the string the var store holds.
func setsValueToString(v any) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case json.Number:
		return t.String(), nil
	case bool:
		return strconv.FormatBool(t), nil
	case nil:
		return "", nil
	default:
		return "", fmt.Errorf("unsupported value type %T (want string, number or boolean)", v)
	}
}
