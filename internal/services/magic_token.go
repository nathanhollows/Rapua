package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	ErrMagicTokenInvalid = errors.New("invalid magic token")
	ErrMagicTokenExpired = errors.New("magic token has expired")
)

// MagicTokenService handles generation and validation of magic login tokens.
// Tokens are stateless HMAC-signed strings containing userID and expiry.
type MagicTokenService struct {
	secretKey []byte
}

// NewMagicTokenService creates a new MagicTokenService with the given secret key.
func NewMagicTokenService(secretKey []byte) *MagicTokenService {
	return &MagicTokenService{
		secretKey: secretKey,
	}
}

// GenerateToken creates a time-limited magic login token for the given user ID.
// Format: base64url(userID:expiryUnix:signature).
func (s *MagicTokenService) GenerateToken(userID string, duration time.Duration) (string, error) {
	if userID == "" {
		return "", errors.New("userID cannot be empty")
	}
	if strings.Contains(userID, ":") {
		return "", errors.New("userID cannot contain ':'")
	}

	expiry := time.Now().Add(duration).Unix()
	payload := fmt.Sprintf("%s:%d", userID, expiry)

	signature := s.sign(payload)

	token := fmt.Sprintf("%s:%s", payload, signature)
	encoded := base64.RawURLEncoding.EncodeToString([]byte(token))

	return encoded, nil
}

// ValidateToken verifies a magic login token and returns the user ID if valid.
func (s *MagicTokenService) ValidateToken(token string) (string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", ErrMagicTokenInvalid
	}

	parts := strings.Split(string(decoded), ":")
	if len(parts) != 3 {
		return "", ErrMagicTokenInvalid
	}

	userID := parts[0]
	expiryStr := parts[1]
	providedSignature := parts[2]

	// Verify signature
	payload := fmt.Sprintf("%s:%s", userID, expiryStr)
	expectedSignature := s.sign(payload)

	if !hmac.Equal([]byte(providedSignature), []byte(expectedSignature)) {
		return "", ErrMagicTokenInvalid
	}

	// Check expiry
	expiry, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		return "", ErrMagicTokenInvalid
	}

	if time.Now().Unix() > expiry {
		return "", ErrMagicTokenExpired
	}

	return userID, nil
}

// sign creates an HMAC-SHA256 signature of the payload.
func (s *MagicTokenService) sign(payload string) string {
	h := hmac.New(sha256.New, s.secretKey)
	h.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
