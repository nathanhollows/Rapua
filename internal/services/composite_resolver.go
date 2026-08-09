package services

import (
	"strconv"
	"strings"
	"time"

	"github.com/nathanhollows/Rapua/v8/models"
)

// PlayerVarResolver implements game.VarResolver by composing two state layers:
//  1. Built-in state (player.points, run.started_at, game.team_count,
//     objective.<slug>)
//  2. Creator-defined vars (from run_var_states, set by block `sets` triggers)
//
// Priority: built-in > creator. Unknown vars return ("", false).
type PlayerVarResolver struct {
	team        *models.Run
	creatorVars map[string]string
	runCount    int
}

// NewPlayerVarResolver creates a resolver for the given team state.
// creatorVars is typically team.VarStates.
func NewPlayerVarResolver(team *models.Run, creatorVars map[string]string) *PlayerVarResolver {
	return &PlayerVarResolver{
		team:        team,
		creatorVars: creatorVars,
	}
}

// WithRunCount enriches the resolver with the number of teams in the instance.
// Enables the game.team_count built-in variable. Call before evaluating conditions.
func (r *PlayerVarResolver) WithRunCount(n int) *PlayerVarResolver {
	r.runCount = n
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
	case "player.points", "points": // "points" is the pre-respine spelling
		return strconv.Itoa(r.team.Points), true
	case "run.started_at":
		if r.team.StartedAt.IsZero() {
			return "", true
		}
		return r.team.StartedAt.Format(time.RFC3339), true
	case "game.team_count":
		return strconv.Itoa(r.runCount), true
	}
	// Claimed by the built-in namespace so a creator var cannot shadow it.
	if after, ok := strings.CutPrefix(name, "objective."); ok && len(after) > 0 {
		return "", true
	}
	return "", false
}
