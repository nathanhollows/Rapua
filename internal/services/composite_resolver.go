package services

import (
	"strings"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
)

// objectiveVarPrefix is the runtime-owned namespace: objective.<slug> reports
// whether that objective has been completed on this run.
const objectiveVarPrefix = "objective."

// PlayerVarResolver implements game.VarResolver by composing two state layers:
//  1. Built-in state (objective.<slug>)
//  2. Creator-defined vars (from run_var_states, set by block `sets` triggers)
//
// Priority: built-in > creator. Unknown vars return ("", false).
type PlayerVarResolver struct {
	completedSlugs map[string]bool
	creatorVars    map[string]string
}

// NewPlayerVarResolver creates a resolver for the given run state.
// creatorVars is typically team.VarStates; completedSlugs holds the slug of
// every objective whose reveal context this run has completed.
func NewPlayerVarResolver(creatorVars map[string]string, completedSlugs map[string]bool) *PlayerVarResolver {
	return &PlayerVarResolver{
		completedSlugs: completedSlugs,
		creatorVars:    creatorVars,
	}
}

// ResolveVar looks up a variable by name across both state layers.
func (r *PlayerVarResolver) ResolveVar(name string) (string, bool) {
	if slug, ok := strings.CutPrefix(name, objectiveVarPrefix); ok && slug != "" {
		// The whole namespace resolves whether or not the slug names a real
		// objective, so a creator var can never shadow it. An unknown or
		// incomplete objective is falsy, not absent.
		if r.completedSlugs[slug] {
			return objectiveVarDone, true
		}
		return "", true
	}
	if val, ok := r.creatorVars[name]; ok {
		return val, true
	}
	return "", false
}

// objectiveVarDone is the truthy value objective.<slug> takes once complete.
// Any non-falsy string would do; this one is what the published spec names.
const objectiveVarDone = "done"

// completedObjectiveSlugs maps completed objective IDs to their slugs, which is
// the form objective.<slug> resolution needs.
func completedObjectiveSlugs(objectives []models.Objective, completedIDs []string) map[string]bool {
	slugByID := make(map[string]string, len(objectives))
	for _, obj := range objectives {
		slugByID[obj.ID] = obj.Slug
	}

	slugs := make(map[string]bool, len(completedIDs))
	for _, id := range completedIDs {
		if slug, ok := slugByID[id]; ok && slug != "" {
			slugs[slug] = true
		}
	}
	return slugs
}

var _ game.VarResolver = (*PlayerVarResolver)(nil)
