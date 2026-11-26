package services

import "errors"

var (
	ErrBlockNotValidForContext = errors.New("block type not valid for context")
	// ErrBlockNotMovable is a move between contexts a block cannot make: only
	// the proof and reveal of an objective trade blocks.
	ErrBlockNotMovable      = errors.New("block cannot be moved there")
	ErrInsufficientCredits  = errors.New("insufficient credits to start team")
	ErrPermissionDenied     = errors.New("permission denied")
	ErrTeamNotFound         = errors.New("team not found")
	ErrUserNotAuthenticated = errors.New("user not authenticated")
)
