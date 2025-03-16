package contextkeys

type contextKey string

const (
	UserKey    contextKey = "user"
	TeamKey    contextKey = "team"
	PreviewKey contextKey = "preview"
)
