package services

import "errors"

var (
	ErrAlreadyCheckedIn         = errors.New("player has already scanned in")
	ErrCheckOutAtWrongLocation  = errors.New("team is not at the correct location to check out")
	ErrInsufficientCredits      = errors.New("insufficient credits to start team")
	ErrInstanceSettingsNotFound = errors.New("instance settings not found")
	ErrLocationNotFound         = errors.New("location not found")
	ErrPermissionDenied         = errors.New("permission denied")
	ErrTeamNotFound             = errors.New("team not found")
	ErrUnecessaryCheckOut       = errors.New("player does not need to scan out")
	ErrUnfinishedCheckIn        = errors.New("unfinished check in")
	ErrUserNotAuthenticated     = errors.New("user not authenticated")
)
