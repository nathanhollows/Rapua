package services

import "errors"

var (
	ErrPermissionDenied     = errors.New("permission denied")
	ErrUserNotAuthenticated = errors.New("user not authenticated")
)
