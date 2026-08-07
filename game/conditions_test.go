package game_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/stretchr/testify/assert"
)

// mapResolver implements game.VarResolver using a plain map.
type mapResolver map[string]string

func (m mapResolver) ResolveVar(name string) (string, bool) {
	v, ok := m[name]
	return v, ok
}

// --- isTruthy (tested via Evaluate with bare conditions) ---

func TestEvaluate_BareCheck(t *testing.T) {
	tests := []struct {
		name     string
		vars     mapResolver
		varName  string
		expected bool
	}{
		{"unset var is falsy", mapResolver{}, "x", false},
		{"empty string is falsy", mapResolver{"x": ""}, "x", false},
		{"'false' is falsy", mapResolver{"x": "false"}, "x", false},
		{"'0' is falsy", mapResolver{"x": "0"}, "x", false},
		{"'true' is truthy", mapResolver{"x": "true"}, "x", true},
		{"'1' is truthy", mapResolver{"x": "1"}, "x", true},
		{"any other string is truthy", mapResolver{"x": "sunny"}, "x", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := game.Condition{Var: tt.varName}
			assert.Equal(t, tt.expected, game.Evaluate(cond, tt.vars))
		})
	}
}

func TestEvaluate_Not(t *testing.T) {
	vars := mapResolver{"x": "true", "y": "false"}

	assert.False(t, game.Evaluate(game.Condition{Var: "x", Not: true}, vars), "not true = false")
	assert.True(t, game.Evaluate(game.Condition{Var: "y", Not: true}, vars), "not false = true")
	assert.True(t, game.Evaluate(game.Condition{Var: "unset", Not: true}, vars), "not unset = true")
}

func TestEvaluate_NotWithOperator(t *testing.T) {
	vars := mapResolver{"color": "red", "count": "5"}

	assert.False(
		t,
		game.Evaluate(game.Condition{Var: "color", Op: "eq", Value: "red", Not: true}, vars),
		"not (color eq red) = false",
	)
	assert.True(
		t,
		game.Evaluate(game.Condition{Var: "color", Op: "eq", Value: "blue", Not: true}, vars),
		"not (color eq blue) = true",
	)
	assert.False(
		t,
		game.Evaluate(game.Condition{Var: "count", Op: "gt", Value: float64(3), Not: true}, vars),
		"not (count gt 3) = false",
	)
	assert.True(
		t,
		game.Evaluate(game.Condition{Var: "count", Op: "gt", Value: float64(10), Not: true}, vars),
		"not (count gt 10) = true",
	)
	assert.False(
		t,
		game.Evaluate(game.Condition{Var: "color", Op: "in", Value: []any{"red", "blue"}, Not: true}, vars),
		"not (color in [red,blue]) = false",
	)
	assert.True(
		t,
		game.Evaluate(game.Condition{Var: "color", Op: "in", Value: []any{"green", "blue"}, Not: true}, vars),
		"not (color in [green,blue]) = true",
	)
}

// --- Operators ---

func TestEvaluate_Eq(t *testing.T) {
	vars := mapResolver{"color": "red", "count": "5"}

	assert.True(t, game.Evaluate(game.Condition{Var: "color", Op: "eq", Value: "red"}, vars))
	assert.False(t, game.Evaluate(game.Condition{Var: "color", Op: "eq", Value: "blue"}, vars))
	assert.True(t, game.Evaluate(game.Condition{Var: "count", Op: "eq", Value: "5"}, vars))
	assert.True(t, game.Evaluate(game.Condition{Var: "count", Op: "eq", Value: float64(5)}, vars))
}

func TestEvaluate_Neq(t *testing.T) {
	vars := mapResolver{"color": "red"}

	assert.False(t, game.Evaluate(game.Condition{Var: "color", Op: "neq", Value: "red"}, vars))
	assert.True(t, game.Evaluate(game.Condition{Var: "color", Op: "neq", Value: "blue"}, vars))
}

func TestEvaluate_NumericComparisons(t *testing.T) {
	vars := mapResolver{"points": "10"}

	tests := []struct {
		op       string
		value    any
		expected bool
	}{
		{"gt", float64(9), true},
		{"gt", float64(10), false},
		{"lt", float64(11), true},
		{"lt", float64(10), false},
		{"gte", float64(10), true},
		{"gte", float64(11), false},
		{"lte", float64(10), true},
		{"lte", float64(9), false},
		// integer types
		{"gt", int(9), true},
		{"gte", int(10), true},
	}
	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			cond := game.Condition{Var: "points", Op: tt.op, Value: tt.value}
			assert.Equal(t, tt.expected, game.Evaluate(cond, vars))
		})
	}
}

func TestEvaluate_NumericFallbackToString(t *testing.T) {
	// When values can't be parsed as numbers, fall back to string comparison.
	vars := mapResolver{"tier": "b"}

	assert.True(t, game.Evaluate(game.Condition{Var: "tier", Op: "gt", Value: "a"}, vars))
	assert.False(t, game.Evaluate(game.Condition{Var: "tier", Op: "lt", Value: "a"}, vars))
}

func TestEvaluate_Eq_NumericNormalisation(t *testing.T) {
	// "5.0" stored vs float64(5) expected — numeric equality applies.
	vars := mapResolver{"count": "5.0"}
	assert.True(t, game.Evaluate(game.Condition{Var: "count", Op: "eq", Value: float64(5)}, vars))
	assert.False(t, game.Evaluate(game.Condition{Var: "count", Op: "neq", Value: float64(5)}, vars))
}

func TestEvaluate_In_StringSlice(t *testing.T) {
	vars := mapResolver{"weather": "raining"}

	assert.True(t, game.Evaluate(game.Condition{
		Var: "weather", Op: "in", Value: []string{"raining", "storm"},
	}, vars))
	assert.False(t, game.Evaluate(game.Condition{
		Var: "weather", Op: "in", Value: []string{"sunny", "cloudy"},
	}, vars))
}

func TestEvaluate_In(t *testing.T) {
	vars := mapResolver{"weather": "raining"}

	assert.True(t, game.Evaluate(game.Condition{
		Var: "weather", Op: "in", Value: []any{"raining", "storm"},
	}, vars))
	assert.False(t, game.Evaluate(game.Condition{
		Var: "weather", Op: "in", Value: []any{"sunny", "cloudy"},
	}, vars))
	// Non-slice value → false
	assert.False(t, game.Evaluate(game.Condition{
		Var: "weather", Op: "in", Value: "raining",
	}, vars))
}

func TestEvaluate_NotIn(t *testing.T) {
	vars := mapResolver{"weather": "sunny"}

	assert.True(t, game.Evaluate(game.Condition{
		Var: "weather", Op: "not_in", Value: []any{"raining", "storm"},
	}, vars))
	assert.False(t, game.Evaluate(game.Condition{
		Var: "weather", Op: "not_in", Value: []any{"sunny", "cloudy"},
	}, vars))
}

func TestEvaluate_UnknownOp(t *testing.T) {
	vars := mapResolver{"x": "1"}
	// Unknown op returns false.
	assert.False(t, game.Evaluate(game.Condition{Var: "x", Op: "contains", Value: "1"}, vars))
}

// --- EvaluateWhen ---

func TestEvaluateWhen_Nil(t *testing.T) {
	assert.True(t, game.EvaluateWhen(nil, mapResolver{}), "nil when = visible")
}

func TestEvaluateWhen_EmptyClause(t *testing.T) {
	assert.True(t, game.EvaluateWhen(&game.WhenClause{}, mapResolver{}), "empty clause = visible")
}

func TestEvaluateWhen_AllOf(t *testing.T) {
	vars := mapResolver{"a": "true", "b": "true", "c": "false"}

	// All true → visible
	when := &game.WhenClause{AllOf: []game.Condition{
		{Var: "a"},
		{Var: "b"},
	}}
	assert.True(t, game.EvaluateWhen(when, vars))

	// One false → hidden
	when2 := &game.WhenClause{AllOf: []game.Condition{
		{Var: "a"},
		{Var: "c"},
	}}
	assert.False(t, game.EvaluateWhen(when2, vars))
}

func TestEvaluateWhen_AnyOf(t *testing.T) {
	vars := mapResolver{"a": "false", "b": "true"}

	// One true → visible
	when := &game.WhenClause{AnyOf: []game.Condition{
		{Var: "a"},
		{Var: "b"},
	}}
	assert.True(t, game.EvaluateWhen(when, vars))

	// None true → hidden
	vars2 := mapResolver{"a": "false", "b": "false"}
	assert.False(t, game.EvaluateWhen(when, vars2))
}

func TestEvaluateWhen_Combined(t *testing.T) {
	vars := mapResolver{"points": "15", "took_bergamot": "true", "weather": "raining"}

	when := &game.WhenClause{
		AllOf: []game.Condition{
			{Var: "points", Op: "gte", Value: float64(10)},
			{Var: "took_bergamot"},
		},
		AnyOf: []game.Condition{
			{Var: "weather", Op: "in", Value: []any{"raining", "storm"}},
		},
	}
	assert.True(t, game.EvaluateWhen(when, vars))

	// AllOf fails
	vars2 := mapResolver{"points": "5", "took_bergamot": "true", "weather": "raining"}
	assert.False(t, game.EvaluateWhen(when, vars2))

	// AnyOf fails
	vars3 := mapResolver{"points": "15", "took_bergamot": "true", "weather": "sunny"}
	assert.False(t, game.EvaluateWhen(when, vars3))
}

func TestEvaluateWhen_NotNegation(t *testing.T) {
	vars := mapResolver{"found_code": "true"}

	// Disappearing hint pattern: visible until found_code is true
	when := &game.WhenClause{AllOf: []game.Condition{
		{Var: "found_code", Not: true},
	}}
	assert.False(t, game.EvaluateWhen(when, vars), "hidden when found_code is true")

	vars2 := mapResolver{}
	assert.True(t, game.EvaluateWhen(when, vars2), "visible when found_code unset")
}
