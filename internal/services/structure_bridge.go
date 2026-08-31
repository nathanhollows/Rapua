package services

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
)

// The document is a recursive tree of objectives; storage is still a group blob
// on quests.game_structure with objective rows hanging off it. Everything in
// this file translates between the two.
//
// One thing the document expresses does not survive the round trip, because
// storage keeps objectives and subgroups in separate arrays rather than in one
// ordered list: interleaving. [leaf, section, leaf] comes back as
// [leaf, leaf, section]. Under ordered routing that is a change in behaviour,
// not just in presentation.
//
// A section's own proof and reveal blocks have nowhere to go either, since
// blocks are owned by objective rows and a group is not one. Lint rejects that
// document outright (SECTION_BLOCKS_NOT_STORED) rather than letting it import
// without the gate the author wrote.
//
// Everything else round-trips exactly, including the completion band, which is
// stored verbatim rather than being narrowed to the engine's older vocabulary.

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

// bandFromGroup reads a stored group's completion band.
//
// A group written by this bridge carries the band verbatim. One written before
// the band existed carries only the older trio, which is read as:
//
//	completion=all                         -> both bounds omitted ([n, n])
//	completion=minimum k, auto_advance      -> [k, k], auto-completing at k
//	completion=minimum k, no auto_advance   -> [k, omitted], the player finishes
func bandFromGroup(gs models.GameStructure) (*int, *int) {
	if gs.ChildrenMin != nil || gs.ChildrenMax != nil {
		return gs.ChildrenMin, gs.ChildrenMax
	}
	if gs.CompletionType != models.CompletionMinimum {
		return nil, nil
	}
	minChildren := gs.MinimumRequired
	if gs.AutoAdvance {
		maxChildren := minChildren
		return &minChildren, &maxChildren
	}
	return &minChildren, nil
}

// groupCompletionFromBand derives the navigation engine's completion trio from
// a band. The band itself is stored alongside, so this is a narrowing for the
// engine's benefit rather than the record of what the author wrote: a genuine
// range becomes its minimum with auto-advance off, which keeps the half the
// engine can act on (the player still chooses when to finish).
func groupCompletionFromBand(obj game.ObjectiveDoc) (models.CompletionType, int, bool) {
	if obj.ChildrenMin == nil && obj.ChildrenMax == nil {
		return models.CompletionAll, 0, true
	}
	band := obj.Band()
	return models.CompletionMinimum, band.Min, band.AutoCompletes()
}

// newSectionGroup builds the stored group for an objective that has children.
// The band is written verbatim and the engine's completion trio derived from
// it, so the author's range survives even though the engine cannot act on it.
func newSectionGroup(id string, obj game.ObjectiveDoc) models.GameStructure {
	gs := models.GameStructure{
		ID:           id,
		Slug:         obj.Slug,
		Name:         obj.Title,
		Color:        obj.Color,
		Routing:      obj.Routing,
		MaxNext:      obj.MaxNext,
		ChildrenMin:  obj.ChildrenMin,
		ChildrenMax:  obj.ChildrenMax,
		FinishLabel:  obj.FinishLabel,
		Depends:      obj.Depends,
		ObjectiveIDs: []string{},
		SubGroups:    []models.GameStructure{},
	}
	gs.CompletionType, gs.MinimumRequired, gs.AutoAdvance = groupCompletionFromBand(obj)
	return gs
}

// sectionSlug prefers the group's stored slug, falling back to one minted from
// its name. Groups predate slugs, so a name is all an older row carries; a
// minted slug is stable only while the name is, which is why a slug written by
// this bridge is kept and reused thereafter.
func sectionSlug(gs models.GameStructure) string {
	if gs.Slug != "" {
		return gs.Slug
	}
	return slugify(gs.Name)
}

// sectionTitle guarantees a non-empty title. A group may legally have a blank
// name; an objective may not have a blank title, so exporting one verbatim
// would produce a document that fails its own import.
func sectionTitle(gs models.GameStructure) string {
	if gs.Name != "" {
		return gs.Name
	}
	return "Untitled section"
}

// slugify renders a group name as a slug candidate.
func slugify(name string) string {
	slug := nonSlugChars.ReplaceAllString(strings.ToLower(name), "-")
	return strings.Trim(slug, "-")
}

// uniqueSlug returns a slug not already in taken, and records it. Slugs must be
// unique across the whole document, and two groups may share a name.
func uniqueSlug(candidate string, taken map[string]bool) string {
	if candidate == "" {
		candidate = "section"
	}
	slug := candidate
	for n := 2; taken[slug]; n++ {
		slug = fmt.Sprintf("%s-%d", candidate, n)
	}
	taken[slug] = true
	return slug
}
