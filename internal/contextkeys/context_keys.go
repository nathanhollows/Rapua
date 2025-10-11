package contextkeys

import "context"

// ContextKey defines typed context keys to prevent key collisions.
type ContextKey string

// Predefined context keys for the application.
const (
	UserKey    ContextKey = "user"
	TeamKey    ContextKey = "team"
	PreviewKey ContextKey = "preview"
	StatusKey  ContextKey = "status"
)

// UserStatus represents the current status of the application.
type UserStatus struct {
	IsAdminLoggedIn bool `json:"is_admin_logged_in"`
}

// DefaultuserStatus returns a new ApplicationStatus with default (false) values.
func DefaultuserStatus() UserStatus {
	return UserStatus{
		IsAdminLoggedIn: false,
	}
}

// GetUserStatus retrieves the application status from the context
// If no status is found, it returns a default status.
func GetUserStatus(ctx context.Context) UserStatus {
	if ctx == nil {
		return DefaultuserStatus()
	}

	status, ok := ctx.Value(StatusKey).(UserStatus)
	if !ok {
		return DefaultuserStatus()
	}

	return status
}

// WithUserStatus adds application status to the context
// If the provided context is nil, it creates a new context.
func WithUserStatus(ctx context.Context, status UserStatus) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, StatusKey, status)
}
