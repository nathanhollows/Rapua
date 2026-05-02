package game

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const (
	opEq    = "eq"
	opNeq   = "neq"
	opGt    = "gt"
	opLt    = "lt"
	opGte   = "gte"
	opLte   = "lte"
	opIn    = "in"
	opNotIn = "not_in"
)

// Condition is a single structured visibility condition.
//
// Bare check (op omitted): the variable must be truthy.
// Comparison (op set): the variable value is coerced and compared against Value.
// Not inverts the result.
type Condition struct {
	Var   string `json:"var"`
	Op    string `json:"op,omitempty"`    // eq|neq|gt|lt|gte|lte|in|not_in; omit for truthy check
	Value any    `json:"value,omitempty"` // string, number, bool, or []any for in/not_in
	Not   bool   `json:"not,omitempty"`
}

// WhenClause holds the visibility conditions for a block, location, or group.
//
// AllOf: all conditions must be true (AND).
// AnyOf: at least one condition must be true (OR).
// Both can be combined: (all AllOf) AND (any one of AnyOf).
type WhenClause struct {
	AllOf []Condition `json:"all_of,omitempty"`
	AnyOf []Condition `json:"any_of,omitempty"`
}

// VarResolver looks up variable values by name.
// Returns (value, true) if set, ("", false) if not set.
type VarResolver interface {
	ResolveVar(name string) (string, bool)
}

// isTruthy returns true if a raw string value is considered truthy.
//
// Falsy values: unset (exists=false), "", "false", "0".
// Everything else is truthy.
func isTruthy(val string, exists bool) bool {
	if !exists {
		return false
	}
	switch val {
	case "", "false", "0":
		return false
	}
	return true
}

// Evaluate checks a single Condition against the resolver.
// Returns true if the condition is met.
func Evaluate(cond Condition, resolver VarResolver) bool {
	val, exists := resolver.ResolveVar(cond.Var)

	var result bool
	if cond.Op == "" {
		// Bare truthy check
		result = isTruthy(val, exists)
	} else {
		result = evalOp(val, cond.Op, cond.Value)
	}

	if cond.Not {
		return !result
	}
	return result
}

// EvaluateWhen checks a full WhenClause. Returns true if the element should be visible.
//
// Logic:
//  1. nil → visible
//  2. All AllOf conditions must be true
//  3. If AnyOf is non-empty, at least one must be true
//  4. Otherwise visible
func EvaluateWhen(when *WhenClause, resolver VarResolver) bool {
	if when == nil {
		return true
	}
	for _, cond := range when.AllOf {
		if !Evaluate(cond, resolver) {
			return false
		}
	}
	if len(when.AnyOf) > 0 {
		met := false
		for _, cond := range when.AnyOf {
			if Evaluate(cond, resolver) {
				met = true
				break
			}
		}
		if !met {
			return false
		}
	}
	return true
}

// evalOp compares raw (string from resolver) against expected using op.
func evalOp(raw, op string, expected any) bool {
	switch op {
	case opEq:
		// If expected is numeric, prefer numeric equality so "5.0" == float64(5).
		// Falls back to string comparison if either side cannot be parsed.
		rawF, rawErr := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		expF, expErr := toFloat64(expected)
		if rawErr == nil && expErr == nil {
			return rawF == expF
		}
		return raw == toString(expected)
	case opNeq:
		rawF, rawErr := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		expF, expErr := toFloat64(expected)
		if rawErr == nil && expErr == nil {
			return rawF != expF
		}
		return raw != toString(expected)
	case opGt, opLt, opGte, opLte:
		return compareNumeric(raw, op, expected)
	case opIn:
		return containsValue(raw, expected)
	case opNotIn:
		return !containsValue(raw, expected)
	}
	return false
}

// toString converts any value to its string representation for eq/neq comparisons.
func toString(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// compareNumeric parses both sides as float64 and compares.
// Falls back to string comparison if either side cannot be parsed as a number.
func compareNumeric(raw, op string, expected any) bool {
	rawF, rawErr := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	expF, expErr := toFloat64(expected)

	if rawErr != nil || expErr != nil {
		// Fall back to string comparison
		rawStr := raw
		expStr := toString(expected)
		switch op {
		case opGt:
			return rawStr > expStr
		case opLt:
			return rawStr < expStr
		case opGte:
			return rawStr >= expStr
		case opLte:
			return rawStr <= expStr
		}
		return false
	}

	switch op {
	case opGt:
		return rawF > expF
	case opLt:
		return rawF < expF
	case opGte:
		return rawF >= expF
	case opLte:
		return rawF <= expF
	}
	return false
}

// toFloat64 converts a value to float64. Returns an error if not a numeric type.
func toFloat64(v any) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case float32:
		return float64(n), nil
	case int:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case int32:
		return float64(n), nil
	case string:
		return strconv.ParseFloat(strings.TrimSpace(n), 64)
	}
	return 0, fmt.Errorf("cannot convert %T to float64", v)
}

// containsValue checks whether raw matches any element in the expected slice.
// Accepts []any (from JSON decode) or []string (from Go code).
func containsValue(raw string, expected any) bool {
	switch slice := expected.(type) {
	case []any:
		for _, elem := range slice {
			if raw == toString(elem) {
				return true
			}
		}
	case []string:
		for _, elem := range slice {
			if raw == elem {
				return true
			}
		}
	}
	return false
}

// Value implements driver.Valuer for bun/sql database storage (TEXT column).
// Returns nil (SQL NULL) when the clause is empty.
//

func (w *WhenClause) Value() (driver.Value, error) {
	if len(w.AllOf) == 0 && len(w.AnyOf) == 0 {
		return nil, nil //nolint:nilnil // nil driver.Value = SQL NULL; nil error = no failure
	}
	data, err := json.Marshal(w)
	if err != nil {
		return nil, fmt.Errorf("marshalling WhenClause: %w", err)
	}
	return string(data), nil
}

// Scan implements sql.Scanner for bun/sql database retrieval.
func (w *WhenClause) Scan(src any) error {
	if src == nil {
		return nil
	}
	var data []byte
	switch v := src.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into WhenClause", src)
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, w)
}
