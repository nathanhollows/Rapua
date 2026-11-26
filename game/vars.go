package game

import (
	"regexp"
	"slices"
)

// MaxVarNameLength bounds a variable name so it stays readable in a badge.
const MaxVarNameLength = 64

var varNamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// DependsField lists the variables that must all be set before an objective
// opens or a content block shows. Every name is a truthy check: there is no
// negation, so a gate that has opened never closes again.
type DependsField []string

// ValidVarName reports whether name can be set or depended on.
func ValidVarName(name string) bool {
	return len(name) <= MaxVarNameLength && varNamePattern.MatchString(name)
}

// DependsMet reports whether every name in deps is set. An empty list is met.
//
// An entry that names nothing fails closed. Lint rejects one at import
// (DEPENDS_EMPTY_NAME), so this only fires on state that bypassed import, and
// a gate nobody can read should stay shut rather than swing open.
func DependsMet(deps DependsField, vars map[string]bool) bool {
	for _, name := range deps {
		if name == "" || !vars[name] {
			return false
		}
	}
	return true
}

// ChosenVarsReporter is implemented by blocks whose player picks which
// variables completing them sets, such as a choice.
type ChosenVarsReporter interface {
	ChosenVars(state PlayerState) []string
}

// VarDeclarer is implemented by blocks that can set more than their own sets
// field, so lint and the editor can offer every name the block might write.
type VarDeclarer interface {
	DeclaredVars() []string
}

// CompletedOutsideCheckIn is implemented by an interactive block whose
// completion its own handler records rather than check-in, so there is no
// moment at which it could set a variable.
type CompletedOutsideCheckIn interface {
	CompletedOutsideCheckIn()
}

// CanSetVars reports whether a block can carry its own sets field. Content
// cannot be solved, so it cannot be the thing that earned a gate; a chooser
// sets its options' variables instead.
func CanSetVars(block Block) bool {
	if !block.RequiresValidation() {
		return false
	}
	if _, chooses := block.(ChosenVarsReporter); chooses {
		return false
	}
	_, elsewhere := block.(CompletedOutsideCheckIn)
	return !elsewhere
}

// VarsSetBy returns the variables a block writes for this state, which only a
// completed interactive block does.
func VarsSetBy(block Block, state PlayerState) []string {
	if !block.RequiresValidation() || state == nil || !state.IsComplete() {
		return nil
	}
	if chooser, ok := block.(ChosenVarsReporter); ok {
		return chooser.ChosenVars(state)
	}
	if sets := block.GetSets(); sets != "" && CanSetVars(block) {
		return []string{sets}
	}
	return nil
}

// DeclaredVars returns every variable the block could write, whatever the
// player does.
func DeclaredVars(block Block) []string {
	if !block.RequiresValidation() {
		return nil
	}
	if declarer, ok := block.(VarDeclarer); ok {
		return declarer.DeclaredVars()
	}
	if sets := block.GetSets(); sets != "" && CanSetVars(block) {
		return []string{sets}
	}
	return nil
}

// NormaliseDepends drops empty and repeated names, keeping the author's order.
func NormaliseDepends(deps []string) DependsField {
	out := make(DependsField, 0, len(deps))
	for _, name := range deps {
		if name == "" || slices.Contains(out, name) {
			continue
		}
		out = append(out, name)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
