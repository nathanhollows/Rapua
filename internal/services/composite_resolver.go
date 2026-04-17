package services

import (
	"strconv"
	"strings"

	"github.com/nathanhollows/Rapua/v7/models"
)

// PlayerVarResolver implements game.VarResolver by composing three state layers:
//  1. Built-in state (points, location.*.visited, location.*.checked_in)
//  2. Creator-defined vars (from team_var_states, set by block `sets` triggers)
//  3. External vars (admin-set, from instance settings)
//
// Priority: built-in > creator > external. Unknown vars return ("", false).
type PlayerVarResolver struct {
	team         *models.Team
	creatorVars  map[string]string
	externalVars map[string]string
}

// NewPlayerVarResolver creates a resolver for the given team state.
// creatorVars is typically team.VarStates; externalVars comes from instance settings.
// Either map may be nil.
func NewPlayerVarResolver(team *models.Team, creatorVars, externalVars map[string]string) *PlayerVarResolver {
	return &PlayerVarResolver{
		team:         team,
		creatorVars:  creatorVars,
		externalVars: externalVars,
	}
}

// ResolveVar looks up a variable by name across all three state layers.
func (r *PlayerVarResolver) ResolveVar(name string) (string, bool) {
	if val, ok := r.resolveBuiltIn(name); ok {
		return val, true
	}
	if val, ok := r.creatorVars[name]; ok {
		return val, true
	}
	if val, ok := r.externalVars[name]; ok {
		return val, true
	}
	return "", false
}

// resolveBuiltIn handles the built-in state namespace.
func (r *PlayerVarResolver) resolveBuiltIn(name string) (string, bool) {
	if name == "points" {
		return strconv.Itoa(r.team.Points), true
	}

	if after, ok := strings.CutPrefix(name, "location."); ok {
		// Expect "location.<slug>.visited" or "location.<slug>.checked_in"
		dot := strings.LastIndex(after, ".")
		if dot < 0 {
			return "", false
		}
		slug, attr := after[:dot], after[dot+1:]
		if attr != "visited" && attr != "checked_in" {
			return "", false
		}

		for _, ci := range r.team.CheckIns {
			if ci.Location.Slug == slug {
				if attr == "visited" {
					return "true", true
				}
				// checked_in: all required blocks completed
				if ci.BlocksCompleted {
					return "true", true
				}
				return "false", true
			}
		}
		// Location not visited at all
		return "false", true
	}

	return "", false
}
