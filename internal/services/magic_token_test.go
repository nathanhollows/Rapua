package services

import (
	"errors"
	"testing"
	"time"
)

func TestMagicTokenService_GenerateAndValidate(t *testing.T) {
	service := NewMagicTokenService([]byte("test-secret-key-32-bytes-long!!"))

	userID := "user-123-uuid"
	duration := 1 * time.Minute

	// Generate token
	token, err := service.GenerateToken(userID, duration)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	if token == "" {
		t.Fatal("GenerateToken returned empty token")
	}

	// Validate token
	gotUserID, err := service.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if gotUserID != userID {
		t.Errorf("ValidateToken returned wrong userID: got %q, want %q", gotUserID, userID)
	}
}

func TestMagicTokenService_ExpiredToken(t *testing.T) {
	service := NewMagicTokenService([]byte("test-secret-key-32-bytes-long!!"))

	userID := "user-123-uuid"
	// Generate token that expires immediately
	duration := -1 * time.Second

	token, err := service.GenerateToken(userID, duration)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// Validate token should fail with expired error
	_, err = service.ValidateToken(token)
	if !errors.Is(err, ErrMagicTokenExpired) {
		t.Errorf("ValidateToken: got error %v, want %v", err, ErrMagicTokenExpired)
	}
}

func TestMagicTokenService_InvalidToken(t *testing.T) {
	service := NewMagicTokenService([]byte("test-secret-key-32-bytes-long!!"))

	testCases := []struct {
		name  string
		token string
	}{
		{"empty token", ""},
		{"invalid base64", "not-valid-base64!!!"},
		{"wrong format", "dXNlci1pZA"},                        // just "user-id" in base64
		{"tampered signature", "dXNlci1pZDoxNzA2MTIzNDU2OmFi"}, // wrong signature
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.ValidateToken(tc.token)
			if !errors.Is(err, ErrMagicTokenInvalid) {
				t.Errorf("ValidateToken(%q): got error %v, want %v", tc.token, err, ErrMagicTokenInvalid)
			}
		})
	}
}

func TestMagicTokenService_WrongSecretKey(t *testing.T) {
	service1 := NewMagicTokenService([]byte("secret-key-one-32-bytes-long!!!"))
	service2 := NewMagicTokenService([]byte("secret-key-two-32-bytes-long!!!"))

	userID := "user-123-uuid"
	duration := 1 * time.Minute

	// Generate token with service1
	token, err := service1.GenerateToken(userID, duration)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// Try to validate with service2 (different key)
	_, err = service2.ValidateToken(token)
	if !errors.Is(err, ErrMagicTokenInvalid) {
		t.Errorf("ValidateToken with wrong key: got error %v, want %v", err, ErrMagicTokenInvalid)
	}
}

func TestMagicTokenService_EmptyUserID(t *testing.T) {
	service := NewMagicTokenService([]byte("test-secret-key-32-bytes-long!!"))

	_, err := service.GenerateToken("", 1*time.Minute)
	if err == nil {
		t.Error("GenerateToken with empty userID should return error")
	}
}

func TestMagicTokenService_UserIDWithColon(t *testing.T) {
	service := NewMagicTokenService([]byte("test-secret-key-32-bytes-long!!"))

	_, err := service.GenerateToken("user:with:colons", 1*time.Minute)
	if err == nil {
		t.Error("GenerateToken with colon in userID should return error")
	}
}
