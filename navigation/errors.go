package navigation

import "errors"

// Validation errors.
var (
	ErrNoVisibleGroups       = errors.New("game structure must have at least one visible subgroup")
	ErrDuplicateGroupID      = errors.New("duplicate group ID found in game structure")
	ErrDuplicateLocationID   = errors.New("duplicate location ID found in game structure")
	ErrMissingGroupName      = errors.New("visible group must have a name")
	ErrMissingGroupColor     = errors.New("visible group must have a color")
	ErrInvalidCompletionType = errors.New("invalid completion type for group")
	ErrMultipleRootGroups    = errors.New("only one root group is allowed")
	ErrNonRootIsRoot         = errors.New("IsRoot can only be true for the top-level group")
	ErrInvalidMaxNext        = errors.New("MaxNext must be greater than 0 for random routing")
)

// Navigation errors.
var (
	ErrGroupNotFound       = errors.New("group not found in game structure")
	ErrAllLocationsVisited = errors.New("all locations in current group have been visited")
)
