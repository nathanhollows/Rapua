package game_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mapResolver implements game.VarResolver using a plain map.
type mapResolver map[string]string

func (m mapResolver) ResolveVar(name string) (string, bool) {
	v, ok := m[name]
	return v, ok
}

// --- isTruthy (tested via a single-name depends list) ---

func TestEvaluateDepends_Truthiness(t *testing.T) {
	tests := []struct {
		name     string
		vars     mapResolver
		expected bool
	}{
		{"unset var is falsy", mapResolver{}, false},
		{"empty string is falsy", mapResolver{"x": ""}, false},
		{"'false' is falsy", mapResolver{"x": "false"}, false},
		{"'0' is falsy", mapResolver{"x": "0"}, false},
		{"'true' is truthy", mapResolver{"x": "true"}, true},
		{"'1' is truthy", mapResolver{"x": "1"}, true},
		{"any other string is truthy", mapResolver{"x": "sunny"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, game.EvaluateDepends(game.DependsField{"x"}, tt.vars))
		})
	}
}

func TestEvaluateDepends_EmptyIsVacuouslyTrue(t *testing.T) {
	assert.True(t, game.EvaluateDepends(nil, mapResolver{}))
	assert.True(t, game.EvaluateDepends(game.DependsField{}, mapResolver{}))
}

func TestEvaluateDepends_NamesAreANDed(t *testing.T) {
	deps := game.DependsField{"a", "b"}
	assert.True(t, game.EvaluateDepends(deps, mapResolver{"a": "true", "b": "true"}))
	assert.False(t, game.EvaluateDepends(deps, mapResolver{"a": "true"}))
	assert.False(t, game.EvaluateDepends(deps, mapResolver{}))
}

func TestEvaluateDepends_NotPrefixNegates(t *testing.T) {
	deps := game.DependsField{"not a"}
	assert.True(t, game.EvaluateDepends(deps, mapResolver{}))
	assert.True(t, game.EvaluateDepends(deps, mapResolver{"a": "false"}))
	assert.False(t, game.EvaluateDepends(deps, mapResolver{"a": "true"}))
}

// A name that merely starts with "not" is an ordinary variable: only the
// "not " prefix, space included, negates.
func TestEvaluateDepends_NotIsPrefixNotSubstring(t *testing.T) {
	assert.True(t, game.EvaluateDepends(game.DependsField{"nothing"}, mapResolver{"nothing": "true"}))
	assert.False(t, game.EvaluateDepends(game.DependsField{"nothing"}, mapResolver{}))
}

// An entry naming nothing is a gate nobody can read, so it stays shut. Lint
// rejects such an entry at import; this is the runtime's half of that contract.
func TestEvaluateDepends_BlankEntryFailsClosed(t *testing.T) {
	assert.False(t, game.EvaluateDepends(game.DependsField{""}, mapResolver{}))
	assert.False(t, game.EvaluateDepends(game.DependsField{"  "}, mapResolver{}))
	assert.False(t, game.EvaluateDepends(game.DependsField{"not "}, mapResolver{}))
	// Still false even when every other entry is satisfied.
	assert.False(t, game.EvaluateDepends(game.DependsField{"a", ""}, mapResolver{"a": "true"}))
}

func TestParseDependsName(t *testing.T) {
	tests := []struct {
		entry       string
		wantName    string
		wantNegated bool
	}{
		{"a", "a", false},
		{"  a  ", "a", false},
		{"not a", "a", true},
		{"not  a", "a", true},
		{"  not a  ", "a", true},
		{"nothing", "nothing", false},
		{"not", "not", false},
		{"not ", "", true},
		{"not\tx", "not\tx", false},
		{"objective.find-maisie", "objective.find-maisie", false},
	}
	for _, tt := range tests {
		t.Run(tt.entry, func(t *testing.T) {
			name, negated := game.ParseDependsName(tt.entry)
			assert.Equal(t, tt.wantName, name)
			assert.Equal(t, tt.wantNegated, negated)
		})
	}
}

func TestDependsField_JSONRoundTrip(t *testing.T) {
	var deps game.DependsField
	require.NoError(t, json.Unmarshal([]byte(`["top", "not heart"]`), &deps))
	assert.Equal(t, game.DependsField{"top", "not heart"}, deps)

	data, err := json.Marshal(deps)
	require.NoError(t, err)
	assert.JSONEq(t, `["top","not heart"]`, string(data))
}
