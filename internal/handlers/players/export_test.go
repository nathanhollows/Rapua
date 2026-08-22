package players

import "log/slog"

// TestHandlerOption sets one field on a handler built by NewTestPlayerHandler.
type TestHandlerOption func(*PlayerHandler)

// NewTestPlayerHandler builds a PlayerHandler for tests in package players_test,
// which cannot reach unexported fields directly.
func NewTestPlayerHandler(opts ...TestHandlerOption) *PlayerHandler {
	h := &PlayerHandler{logger: slog.Default()}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

func WithRunService(s RunService) TestHandlerOption {
	return func(h *PlayerHandler) { h.runService = s }
}

func WithUploadService(s UploadService) TestHandlerOption {
	return func(h *PlayerHandler) { h.uploadService = s }
}
