package game

import "strings"

// notPrefix negates a single depends name.
const notPrefix = "not "

// DependsField is a flat list of variable names gating an objective's
// reachability. Names are implicitly ANDed; a "not " prefix negates one.
// There are no comparison operators: every name is a truthy check.
type DependsField []string

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

// ParseDependsName splits a depends entry into its variable name and whether
// the entry is negated. Surrounding whitespace is ignored so that "not  x" and
// " x " both name the variable x.
//
// Only leading whitespace is stripped before the prefix test, so a dangling
// "not " reports an empty name rather than a variable literally called "not".
// An author who typed the negation and left the name off gets a diagnostic
// (DEPENDS_EMPTY_NAME) instead of a gate that silently never opens. A bare
// "not" with no trailing space is still an ordinary variable name.
func ParseDependsName(entry string) (string, bool) {
	leading := strings.TrimLeft(entry, " \t\n\r")
	if rest, ok := strings.CutPrefix(leading, notPrefix); ok {
		return strings.TrimSpace(rest), true
	}
	return strings.TrimSpace(leading), false
}

// EvaluateDepends reports whether every name in deps evaluates truthy.
// An empty list is vacuously true: an objective with no depends is reachable.
//
// An entry that names nothing fails closed. Lint rejects such an entry at
// import (DEPENDS_EMPTY_NAME), so this only fires on state that bypassed
// import, and a gate nobody can read should stay shut rather than swing open.
func EvaluateDepends(deps DependsField, resolver VarResolver) bool {
	for _, entry := range deps {
		name, negated := ParseDependsName(entry)
		if name == "" {
			return false
		}
		met := isTruthy(resolver.ResolveVar(name))
		if negated {
			met = !met
		}
		if !met {
			return false
		}
	}
	return true
}
