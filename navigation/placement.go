package navigation

import "github.com/nathanhollows/Rapua/v8/models"

// MaxDrawnDepth is how many sections deep the player's screen ever draws.
//
// Past this a container passes through: it still routes, bands and gates, it
// simply has no place on the screen and its rows join the nearest section that
// does. The author's tree has no depth limit; the screen has a fixed one, so
// nesting for a band across four sections costs the player nothing.
const MaxDrawnDepth = 2

// SectionKind says how the player's screen treats a section.
type SectionKind int

const (
	// SectionDrawn is a place the player stands in.
	SectionDrawn SectionKind = iota
	// SectionPassThrough is a section the author marked so. It does not count
	// toward MaxDrawnDepth, so it lets a deeper level be drawn.
	SectionPassThrough
	// SectionForced is a section that would be drawn but sits past
	// MaxDrawnDepth, so it passes through whether the author meant it or not.
	SectionForced
)

// Hidden reports whether the player never stands in a section of this kind.
func (k SectionKind) Hidden() bool { return k != SectionDrawn }

// ancestorsOf returns the sections above an objective, outermost first, without
// the root: the root is the quest rather than a place in it.
//
// A missing parent is an orphan and a repeat is a cycle. Both are shapes
// storage can hold and lint reports; neither is a reason to hide work the run
// can still do, so the walk stops there.
func ancestorsOf(byID map[string]models.Objective, objective models.Objective) []models.Objective {
	var chain []models.Objective
	seen := map[string]bool{objective.ID: true}
	for id := objective.ParentID; id != ""; {
		parent, ok := byID[id]
		if !ok || seen[parent.ID] {
			break
		}
		seen[parent.ID] = true
		if parent.ParentID == "" {
			break
		}
		chain = append([]models.Objective{parent}, chain...)
		id = parent.ParentID
	}
	return chain
}

// drawnChain keeps the sections of a chain that are drawn: the ones the author
// did not pass through, up to MaxDrawnDepth counting from the top. Counting
// from the top is what an author thinks in terms of, and it keeps the cut from
// moving when a row is added beneath it.
func drawnChain(chain []models.Objective) []models.Objective {
	drawn := make([]models.Objective, 0, MaxDrawnDepth)
	for _, section := range chain {
		if section.PassThrough {
			continue
		}
		if len(drawn) == MaxDrawnDepth {
			break
		}
		drawn = append(drawn, section)
	}
	return drawn
}

// PlaceFor finds the section a row is shown under, and the drawn sections above
// it. The zero Objective means the row stands alone, under no section.
func PlaceFor(byID map[string]models.Objective, objective models.Objective) (models.Objective, []models.Objective) {
	drawn := drawnChain(ancestorsOf(byID, objective))
	if len(drawn) == 0 {
		return models.Objective{}, nil
	}
	return drawn[len(drawn)-1], drawn[:len(drawn)-1]
}

// FinishPlaceFor finds the drawn container a finishable section's press belongs
// on: the section itself when it is drawn, and the nearest drawn ancestor when
// it is not, which is also where its children group. A finishable root lands in
// the loose group.
func FinishPlaceFor(
	byID map[string]models.Objective, section models.Objective,
) (models.Objective, []models.Objective) {
	if section.ParentID == "" {
		return models.Objective{}, nil
	}
	drawn := drawnChain(append(ancestorsOf(byID, section), section))
	if len(drawn) == 0 {
		return models.Objective{}, nil
	}
	return drawn[len(drawn)-1], drawn[:len(drawn)-1]
}

// KindOf says how the player's screen treats a section, so the editor can show
// an author the same cut the player gets.
func KindOf(byID map[string]models.Objective, section models.Objective) SectionKind {
	if section.PassThrough {
		return SectionPassThrough
	}
	for _, drawn := range drawnChain(append(ancestorsOf(byID, section), section)) {
		if drawn.ID == section.ID {
			return SectionDrawn
		}
	}
	return SectionForced
}
