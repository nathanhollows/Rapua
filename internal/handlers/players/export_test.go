package players

import "log/slog"

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

func WithCheckInService(s CheckInService) TestHandlerOption {
	return func(h *PlayerHandler) { h.checkInService = s }
}

func WithBlockService(s BlockService) TestHandlerOption {
	return func(h *PlayerHandler) { h.blockService = s }
}

func WithNavigationService(s NavigationService) TestHandlerOption {
	return func(h *PlayerHandler) { h.navigationService = s }
}
