package services

import (
	"strconv"
	"strings"

	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/nathanhollows/Rapua/v7/navigation"
)

const (
	varTrue  = "true"
	varFalse = "false"
)

// PlayerVarResolver implements game.VarResolver by composing two state layers:
//  1. Built-in state (points, game.status, game.team_count, location.*.visited, location.*.checked_in, group.*.completed)
//  2. Creator-defined vars (from team_var_states, set by block `sets` triggers)
//
// Priority: built-in > creator. Unknown vars return ("", false).
type PlayerVarResolver struct {
	team        *models.Team
	creatorVars map[string]string
	teamCount   int
}

// NewPlayerVarResolver creates a resolver for the given team state.
// creatorVars is typically team.VarStates.
func NewPlayerVarResolver(team *models.Team, creatorVars map[string]string) *PlayerVarResolver {
	return &PlayerVarResolver{
		team:        team,
		creatorVars: creatorVars,
	}
}

// WithTeamCount enriches the resolver with the number of teams in the instance.
// Enables the game.team_count built-in variable. Call before evaluating conditions.
func (r *PlayerVarResolver) WithTeamCount(n int) *PlayerVarResolver {
	r.teamCount = n
	return r
}

// ResolveVar looks up a variable by name across both state layers.
func (r *PlayerVarResolver) ResolveVar(name string) (string, bool) {
	if val, ok := r.resolveBuiltIn(name); ok {
		return val, true
	}
	if val, ok := r.creatorVars[name]; ok {
		return val, true
	}
	return "", false
}

// resolveBuiltIn handles the built-in state namespace.
func (r *PlayerVarResolver) resolveBuiltIn(name string) (string, bool) {
	switch name {
	case "points":
		return strconv.Itoa(r.team.Points), true
	case "game.status":
		return strings.ToLower(r.team.Instance.GetStatus().String()), true
	case "game.team_count":
		return strconv.Itoa(r.teamCount), true
	}
	if after, ok := strings.CutPrefix(name, "location."); ok {
		return r.resolveLocationVar(after)
	}
	if after, ok := strings.CutPrefix(name, "group."); ok {
		return r.resolveGroupVar(after)
	}
	return "", false
}

// resolveLocationVar resolves "location.<slug>.visited" and "location.<slug>.checked_in".
func (r *PlayerVarResolver) resolveLocationVar(after string) (string, bool) {
	dot := strings.LastIndex(after, ".")
	if dot < 0 {
		return "", false
	}
	slug, attr := after[:dot], after[dot+1:]
	if attr != "visited" && attr != "checked_in" {
		return "", false
	}
	for _, ci := range r.team.CheckIns {
		if ci.Location.Slug != slug {
			continue
		}
		if attr == "visited" {
			return varTrue, true
		}
		if ci.BlocksCompleted {
			return varTrue, true
		}
		return varFalse, true
	}
	return varFalse, true
}

// resolveGroupVar resolves "group.<name>.completed".
func (r *PlayerVarResolver) resolveGroupVar(after string) (string, bool) {
	if !strings.HasSuffix(after, ".completed") {
		return "", false
	}
	groupName := strings.TrimSuffix(after, ".completed")
	group := navigation.FindGroupByName(&r.team.Instance.GameStructure, groupName)
	if group == nil {
		return varFalse, true
	}
	completedIDs := completedLocationIDs(r.team.CheckIns)
	if navigation.IsGroupCompleted(&r.team.Instance.GameStructure, group.ID, completedIDs) {
		return varTrue, true
	}
	return varFalse, true
}

// completedLocationIDs returns location IDs from checkIns where BlocksCompleted is true.
func completedLocationIDs(checkIns []models.CheckIn) []string {
	ids := make([]string, 0, len(checkIns))
	for _, ci := range checkIns {
		if ci.BlocksCompleted {
			ids = append(ids, ci.LocationID)
		}
	}
	return ids
}
