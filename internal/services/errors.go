package services

import "errors"

var (
	ErrBlockNotValidForContext = errors.New("block type not valid for context")
	ErrInsufficientCredits     = errors.New("insufficient credits to start team")
	ErrPermissionDenied        = errors.New("permission denied")
	ErrTeamNotFound            = errors.New("team not found")
	ErrUserNotAuthenticated    = errors.New("user not authenticated")
)
