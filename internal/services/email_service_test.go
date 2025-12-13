package services_test

import (
	"context"
	"html"
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v6/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildEmailMessage_AllFields(t *testing.T) {
	msg := services.EmailMessage{
		From:        "sender@example.com",
		FromName:    "Sender Name",
		To:          "recipient@example.com",
		ToName:      "Recipient Name",
		Subject:     "Test Subject",
		PlainText:   "Plain text content",
		HTMLContent: "<p>HTML content</p>",
		ReplyTo:     "",
	}

	result, err := services.BuildEmailMessage(msg, "mail.example.com")

	require.NoError(t, err)
	resultStr := string(result)

	// Check headers are present
	assert.Contains(t, resultStr, "From: Sender Name <sender@example.com>")
	assert.Contains(t, resultStr, "To: Recipient Name <recipient@example.com>")
	assert.Contains(t, resultStr, "Subject: Test Subject")
	assert.Contains(t, resultStr, "MIME-Version: 1.0")
	assert.Contains(t, resultStr, "Message-ID: <")
	assert.Contains(t, resultStr, "@mail.example.com>")
	assert.Contains(t, resultStr, "Date:")

	// Check content is present
	assert.Contains(t, resultStr, "Plain text content")
	assert.Contains(t, resultStr, "<p>HTML content</p>")

	// Check no Reply-To when empty
	assert.NotContains(t, resultStr, "Reply-To:")
}

func TestBuildEmailMessage_ReplyTo(t *testing.T) {
	msg := services.EmailMessage{
		From:        "sender@example.com",
		FromName:    "Sender",
		To:          "recipient@example.com",
		ToName:      "Recipient",
		Subject:     "Test",
		PlainText:   "content",
		HTMLContent: "",
		ReplyTo:     "reply@example.com",
	}

	result, err := services.BuildEmailMessage(msg, "mail.example.com")

	require.NoError(t, err)
	assert.Contains(t, string(result), "Reply-To: reply@example.com")
}

func TestBuildEmailMessage_DeterministicHeaderOrder(t *testing.T) {
	msg := services.EmailMessage{
		From:        "sender@example.com",
		FromName:    "Sender",
		To:          "recipient@example.com",
		ToName:      "Recipient",
		Subject:     "Test",
		PlainText:   "content",
		HTMLContent: "",
		ReplyTo:     "",
	}

	// Build multiple times and verify order is consistent
	result1, err := services.BuildEmailMessage(msg, "mail.example.com")
	require.NoError(t, err)

	result2, err := services.BuildEmailMessage(msg, "mail.example.com")
	require.NoError(t, err)

	// Extract just the header portion (before the first boundary)
	getHeaderOrder := func(result []byte) []string {
		lines := strings.Split(string(result), "\r\n")
		var headers []string
		for _, line := range lines {
			if line == "" {
				break
			}
			if idx := strings.Index(line, ":"); idx > 0 {
				headers = append(headers, line[:idx])
			}
		}
		return headers
	}

	headers1 := getHeaderOrder(result1)
	headers2 := getHeaderOrder(result2)

	assert.Equal(t, headers1, headers2, "header order should be deterministic")

	// Verify expected order
	expectedOrder := []string{"From", "To", "Subject", "Date", "Message-ID", "MIME-Version", "Content-Type"}
	assert.Equal(t, expectedOrder, headers1)
}

func TestBuildEmailMessage_HeaderOrderWithReplyTo(t *testing.T) {
	msg := services.EmailMessage{
		From:        "sender@example.com",
		FromName:    "Sender",
		To:          "recipient@example.com",
		ToName:      "Recipient",
		Subject:     "Test",
		PlainText:   "content",
		HTMLContent: "",
		ReplyTo:     "reply@example.com",
	}

	result, err := services.BuildEmailMessage(msg, "mail.example.com")
	require.NoError(t, err)

	lines := strings.Split(string(result), "\r\n")
	var headers []string
	for _, line := range lines {
		if line == "" {
			break
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			headers = append(headers, line[:idx])
		}
	}

	// Reply-To should be inserted after Message-ID
	expectedOrder := []string{
		"From", "To", "Subject", "Date", "Message-ID", "Reply-To", "MIME-Version", "Content-Type",
	}
	assert.Equal(t, expectedOrder, headers)
}

func TestBuildEmailMessage_PlainTextOnly(t *testing.T) {
	msg := services.EmailMessage{
		From:        "sender@example.com",
		FromName:    "Sender",
		To:          "recipient@example.com",
		ToName:      "Recipient",
		Subject:     "Test",
		PlainText:   "Plain text only",
		HTMLContent: "",
		ReplyTo:     "",
	}

	result, err := services.BuildEmailMessage(msg, "mail.example.com")

	require.NoError(t, err)
	assert.Contains(t, string(result), "Plain text only")
	assert.Contains(t, string(result), "text/plain")
}

func TestBuildEmailMessage_HTMLOnly(t *testing.T) {
	msg := services.EmailMessage{
		From:        "sender@example.com",
		FromName:    "Sender",
		To:          "recipient@example.com",
		ToName:      "Recipient",
		Subject:     "Test",
		PlainText:   "",
		HTMLContent: "<p>HTML only</p>",
		ReplyTo:     "",
	}

	result, err := services.BuildEmailMessage(msg, "mail.example.com")

	require.NoError(t, err)
	assert.Contains(t, string(result), "<p>HTML only</p>")
	assert.Contains(t, string(result), "text/html")
}

func TestBuildEmailMessage_UniqueMessageID(t *testing.T) {
	msg := services.EmailMessage{
		From:      "sender@example.com",
		FromName:  "Sender",
		To:        "recipient@example.com",
		ToName:    "Recipient",
		Subject:   "Test",
		PlainText: "content",
	}

	result1, _ := services.BuildEmailMessage(msg, "mail.example.com")
	result2, _ := services.BuildEmailMessage(msg, "mail.example.com")

	// Extract Message-IDs
	extractMessageID := func(result []byte) string {
		lines := strings.SplitSeq(string(result), "\r\n")
		for line := range lines {
			if after, ok := strings.CutPrefix(line, "Message-ID:"); ok {
				return strings.TrimSpace(after)
			}
		}
		return ""
	}

	id1 := extractMessageID(result1)
	id2 := extractMessageID(result2)

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2, "each message should have a unique Message-ID")
}

func TestEmailService_SendEmail_ConfigValidation(t *testing.T) {
	service := services.NewEmailService()

	t.Run("returns error when SMTP_HOST is missing", func(t *testing.T) {
		t.Setenv("SMTP_HOST", "")
		t.Setenv("SMTP_PORT", "587")
		t.Setenv("SMTP_USER", "user")
		t.Setenv("SMTP_PASS", "pass")

		err := service.SendEmail("from@test.com", "From", "to@test.com", "To", "Subject", "text", "html", "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "SMTP configuration incomplete")
	})

	t.Run("returns error when SMTP_PORT is missing", func(t *testing.T) {
		t.Setenv("SMTP_HOST", "smtp.example.com")
		t.Setenv("SMTP_PORT", "")
		t.Setenv("SMTP_USER", "user")
		t.Setenv("SMTP_PASS", "pass")

		err := service.SendEmail("from@test.com", "From", "to@test.com", "To", "Subject", "text", "html", "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "SMTP configuration incomplete")
	})

	t.Run("returns error when SMTP_USER is missing", func(t *testing.T) {
		t.Setenv("SMTP_HOST", "smtp.example.com")
		t.Setenv("SMTP_PORT", "587")
		t.Setenv("SMTP_USER", "")
		t.Setenv("SMTP_PASS", "pass")

		err := service.SendEmail("from@test.com", "From", "to@test.com", "To", "Subject", "text", "html", "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "SMTP configuration incomplete")
	})

	t.Run("returns error when SMTP_PASS is missing", func(t *testing.T) {
		t.Setenv("SMTP_HOST", "smtp.example.com")
		t.Setenv("SMTP_PORT", "587")
		t.Setenv("SMTP_USER", "user")
		t.Setenv("SMTP_PASS", "")

		err := service.SendEmail("from@test.com", "From", "to@test.com", "To", "Subject", "text", "html", "")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "SMTP configuration incomplete")
	})
}

func TestContactEmailXSSPrevention(t *testing.T) {
	// These tests verify that the XSS prevention logic works correctly
	// by testing the html.EscapeString behavior used in SendContactEmail

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "script tag is escaped",
			input:    "<script>alert('xss')</script>",
			expected: "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
		},
		{
			name:     "HTML tags are escaped",
			input:    "<img src=x onerror=alert(1)>",
			expected: "&lt;img src=x onerror=alert(1)&gt;",
		},
		{
			name:     "ampersand is escaped",
			input:    "Tom & Jerry",
			expected: "Tom &amp; Jerry",
		},
		{
			name:     "quotes are escaped",
			input:    `He said "hello"`,
			expected: "He said &#34;hello&#34;",
		},
		{
			name:     "normal text unchanged",
			input:    "Hello, this is a normal message.",
			expected: "Hello, this is a normal message.",
		},
		{
			name:     "unicode preserved",
			input:    "Hello 世界 🌍",
			expected: "Hello 世界 🌍",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := html.EscapeString(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSendContactEmail_FailsWithoutConfig(t *testing.T) {
	service := services.NewEmailService()

	// Clear all SMTP config to ensure it fails at config validation
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_PORT", "")
	t.Setenv("SMTP_USER", "")
	t.Setenv("SMTP_PASS", "")
	t.Setenv("CONTACT_EMAIL", "contact@example.com")

	err := service.SendContactEmail(context.Background(), "Test User", "user@example.com", "Test message")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "SMTP configuration incomplete")
}

func TestSendVerificationEmail_FailsWithoutConfig(t *testing.T) {
	service := services.NewEmailService()

	// Clear all SMTP config
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_PORT", "")
	t.Setenv("SMTP_USER", "")
	t.Setenv("SMTP_PASS", "")
	t.Setenv("SMTP_FROM_EMAIL", "noreply@example.com")
	t.Setenv("SMTP_FROM_NAME", "Rapua")
	t.Setenv("SITE_URL", "http://localhost:8080")

	// Test the sendEmail path to verify config validation
	err := service.SendEmail(
		"noreply@example.com",
		"Rapua",
		"user@example.com",
		"Test User",
		"Please verify your email",
		"Verify here: http://localhost:8080/verify-email/test-token-123",
		"<p>Verify here</p>",
		"",
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "SMTP configuration incomplete")
}
