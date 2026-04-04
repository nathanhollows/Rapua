// Package game defines the core vocabulary for the Rapua game format.
// This package has zero project imports — it is the leaf node of the dependency graph.
// All other packages that need these types import game/.
package game

// BlockContext represents where a block can be used.
type BlockContext string

const (
	ContextLocationContent BlockContext = "location_content" // Regular location content blocks
	ContextLocationClues   BlockContext = "location_clues"   // Clues
	ContextTask            BlockContext = "task"             // Task a la scavenger hunts
	ContextCheckpoint      BlockContext = "checkpoint"       // Verify a player is at a location
	ContextStart           BlockContext = "start"            // Start pages - introductions, rules, set team name
	ContextFinish          BlockContext = "finish"           // Finish/end pages
)
