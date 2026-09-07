package models

// WalkAncestors calls visit for each row on the parent chain above startID,
// starting with startID itself, and stops early if visit returns false.
//
// The walk is bounded by revisiting: stored rows can hold a parent cycle, and
// every plain `for id := x; id != ""; id = parentOf[id]` over one runs
// forever. Lint reports that state and the editor draws it for repair, which
// means code all over the app meets it while it is being fixed rather than
// after.
func WalkAncestors(startID string, parentOf map[string]string, visit func(id string) bool) {
	seen := make(map[string]bool)
	for id := startID; id != ""; id = parentOf[id] {
		if seen[id] {
			return
		}
		seen[id] = true
		if !visit(id) {
			return
		}
	}
}

// HasAncestor reports whether ancestorID is startID or sits above it.
func HasAncestor(startID, ancestorID string, parentOf map[string]string) bool {
	found := false
	WalkAncestors(startID, parentOf, func(id string) bool {
		if id == ancestorID {
			found = true
			return false
		}
		return true
	})
	return found
}
