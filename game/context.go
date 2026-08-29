// Package game defines the core vocabulary for the Rapua game format.
// It is the leaf of the dependency graph: zero project imports.
package game

type BlockContext string

const (
	ContextStart           BlockContext = "start"
	ContextFinish          BlockContext = "finish"
	ContextObjectiveProof  BlockContext = "objective_proof"
	ContextObjectiveReveal BlockContext = "objective_reveal" // Replaces the proof zone once proof clears.
)
