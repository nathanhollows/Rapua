package contextkeys_test

import (
	"context"
	"testing"

	"github.com/nathanhollows/Rapua/v5/internal/contextkeys"
)

func TestDefaultUserStatus(t *testing.T) {
	defaultStatus := contextkeys.DefaultuserStatus()

	if defaultStatus.IsAdminLoggedIn != false {
		t.Errorf("Expected IsAdminLoggedIn to be false, got %v", defaultStatus.IsAdminLoggedIn)
	}
}

func TestGetUserStatus(t *testing.T) {
	testCases := []struct {
		name           string
		ctx            context.Context
		expectedResult contextkeys.UserStatus
	}{
		{
			name:           "Nil context",
			ctx:            nil,
			expectedResult: contextkeys.UserStatus{IsAdminLoggedIn: false},
		},
		{
			name:           "Empty context",
			ctx:            context.Background(),
			expectedResult: contextkeys.UserStatus{IsAdminLoggedIn: false},
		},
		{
			name: "Context with user status",
			ctx: contextkeys.WithUserStatus(context.Background(), contextkeys.UserStatus{
				IsAdminLoggedIn: true,
			}),
			expectedResult: contextkeys.UserStatus{IsAdminLoggedIn: true},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := contextkeys.GetUserStatus(tc.ctx)

			if result.IsAdminLoggedIn != tc.expectedResult.IsAdminLoggedIn {
				t.Errorf("Expected IsAdminLoggedIn to be %v, got %v",
					tc.expectedResult.IsAdminLoggedIn,
					result.IsAdminLoggedIn)
			}
		})
	}
}

func TestWithUserStatus(t *testing.T) {
	testCases := []struct {
		name           string
		inputCtx       context.Context
		status         contextkeys.UserStatus
		expectedResult bool
	}{
		{
			name:           "Nil context",
			inputCtx:       nil,
			status:         contextkeys.UserStatus{IsAdminLoggedIn: true},
			expectedResult: true,
		},
		{
			name:           "Background context",
			inputCtx:       context.Background(),
			status:         contextkeys.UserStatus{IsAdminLoggedIn: true},
			expectedResult: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := contextkeys.WithUserStatus(tc.inputCtx, tc.status)

			// Retrieve the status from the context
			retrievedStatus := contextkeys.GetUserStatus(ctx)

			if retrievedStatus.IsAdminLoggedIn != tc.expectedResult {
				t.Errorf("Expected IsAdminLoggedIn to be %v, got %v",
					tc.expectedResult,
					retrievedStatus.IsAdminLoggedIn)
			}
		})
	}
}

func TestContextKeyUniqueness(t *testing.T) {
	keys := []contextkeys.ContextKey{
		contextkeys.UserKey,
		contextkeys.TeamKey,
		contextkeys.PreviewKey,
		contextkeys.StatusKey,
	}

	// Check for duplicates
	keyMap := make(map[contextkeys.ContextKey]bool)
	for _, key := range keys {
		if keyMap[key] {
			t.Errorf("Duplicate context key found: %v", key)
		}
		keyMap[key] = true
	}
}
