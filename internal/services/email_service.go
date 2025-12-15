package services

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"html"
	"mime/multipart"
	"net/smtp"
	"net/textproto"
	"os"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	templates "github.com/nathanhollows/Rapua/v6/internal/templates/emails"
	"github.com/nathanhollows/Rapua/v6/models"
)

type EmailService struct{}

func NewEmailService() *EmailService {
	return &EmailService{}
}

// emailHeader represents a single email header for ordered output.
type emailHeader struct {
	key   string
	value string
}

// emailMessage holds all the parameters needed to build an email.
type emailMessage struct {
	from        string
	fromName    string
	to          string
	toName      string
	subject     string
	plainText   string
	htmlContent string
	replyTo     string
}

// buildEmailMessage constructs the raw email message bytes.
// This is separated from sendEmail to enable testing without SMTP.
func buildEmailMessage(msg emailMessage, smtpHost string) ([]byte, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Generate Message-ID for proper email threading
	messageID := fmt.Sprintf("<%s@%s>", uuid.New().String(), smtpHost)

	// Build headers in deterministic order
	headers := []emailHeader{
		{"From", fmt.Sprintf("%s <%s>", msg.fromName, msg.from)},
		{"To", fmt.Sprintf("%s <%s>", msg.toName, msg.to)},
		{"Subject", msg.subject},
		{"Date", time.Now().Format(time.RFC1123Z)},
		{"Message-ID", messageID},
		{"MIME-Version", "1.0"},
		{"Content-Type", fmt.Sprintf("multipart/alternative; boundary=%s", writer.Boundary())},
	}

	// Add Reply-To header if provided
	if msg.replyTo != "" {
		headers = append(headers[:5], append([]emailHeader{{"Reply-To", msg.replyTo}}, headers[5:]...)...)
	}

	// Write headers in order
	for _, h := range headers {
		buf.WriteString(fmt.Sprintf("%s: %s\r\n", h.key, h.value))
	}
	buf.WriteString("\r\n")

	// Write plain text part
	if msg.plainText != "" {
		part, err := writer.CreatePart(textproto.MIMEHeader{
			"Content-Type": []string{"text/plain; charset=UTF-8"},
		})
		if err != nil {
			return nil, fmt.Errorf("creating plain text part: %w", err)
		}
		if _, writeErr := part.Write([]byte(msg.plainText)); writeErr != nil {
			return nil, fmt.Errorf("writing plain text: %w", writeErr)
		}
	}

	// Write HTML part
	if msg.htmlContent != "" {
		part, err := writer.CreatePart(textproto.MIMEHeader{
			"Content-Type": []string{"text/html; charset=UTF-8"},
		})
		if err != nil {
			return nil, fmt.Errorf("creating HTML part: %w", err)
		}
		if _, writeErr := part.Write([]byte(msg.htmlContent)); writeErr != nil {
			return nil, fmt.Errorf("writing HTML: %w", writeErr)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}
	return buf.Bytes(), nil
}

// sendEmail sends an email using SMTP with support for both plain text and HTML.
// replyTo is optional - pass empty string to omit.
func (s EmailService) sendEmail(from, fromName, to, toName, subject, plainText, htmlContent, replyTo string) error {
	// Get SMTP configuration from environment
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")

	if smtpHost == "" || smtpPort == "" || smtpUser == "" || smtpPass == "" {
		return errors.New(
			"SMTP configuration incomplete: ensure SMTP_HOST, SMTP_PORT, SMTP_USER, and SMTP_PASS are set",
		)
	}

	msg := emailMessage{
		from:        from,
		fromName:    fromName,
		to:          to,
		toName:      toName,
		subject:     subject,
		plainText:   plainText,
		htmlContent: htmlContent,
		replyTo:     replyTo,
	}

	messageBytes, err := buildEmailMessage(msg, smtpHost)
	if err != nil {
		return err
	}

	// Connect to SMTP server with TLS
	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)

	// Create TLS config with minimum TLS version
	tlsConfig := &tls.Config{
		ServerName: smtpHost,
		MinVersion: tls.VersionTLS12,
	}

	// Connect with STARTTLS (port 587)
	conn, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("connecting to SMTP server: %w", err)
	}
	defer conn.Close()

	// Start TLS
	if tlsErr := conn.StartTLS(tlsConfig); tlsErr != nil {
		return fmt.Errorf("starting TLS: %w", tlsErr)
	}

	// Authenticate
	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	if authErr := conn.Auth(auth); authErr != nil {
		return fmt.Errorf("authenticating: %w", authErr)
	}

	// Set sender
	if mailErr := conn.Mail(from); mailErr != nil {
		return fmt.Errorf("setting sender: %w", mailErr)
	}

	// Set recipient
	if rcptErr := conn.Rcpt(to); rcptErr != nil {
		return fmt.Errorf("setting recipient: %w", rcptErr)
	}

	// Send message
	w, dataErr := conn.Data()
	if dataErr != nil {
		return fmt.Errorf("opening data connection: %w", dataErr)
	}

	if _, writeErr := w.Write(messageBytes); writeErr != nil {
		_ = w.Close()
		return fmt.Errorf("writing message: %w", writeErr)
	}
	if closeErr := w.Close(); closeErr != nil {
		return fmt.Errorf("closing data writer: %w", closeErr)
	}

	// Properly close SMTP connection
	return conn.Quit()
}

func (s EmailService) SendContactEmail(_ context.Context, name, contactEmail, content string) error {
	contactAddr := os.Getenv("CONTACT_EMAIL")
	subject := "New message from Rapua contact form"

	// Escape user input to prevent XSS
	safeName := html.EscapeString(name)
	safeEmail := html.EscapeString(contactEmail)
	safeContent := html.EscapeString(content)

	htmlContent := fmt.Sprintf(`
	<p><strong>Name:</strong> %s</p>
	<p><strong>Email:</strong> %s</p>
	<p>%s</p>
	`, safeName, safeEmail, strings.ReplaceAll(safeContent, "\n", "<br>"))

	plainText := fmt.Sprintf("Name: %s\nEmail: %s\n\n%s", name, contactEmail, content)

	return s.sendEmail(
		contactAddr,          // from
		"Rapua Contact Form", // fromName
		contactAddr,          // to
		"Rapua",              // toName
		subject,
		plainText,
		htmlContent,
		contactEmail, // replyTo - so replies go to the person who submitted
	)
}

func (s EmailService) SendVerificationEmail(ctx context.Context, user models.User) error {
	fromEmail := os.Getenv("SMTP_FROM_EMAIL")
	fromName := os.Getenv("SMTP_FROM_NAME")
	subject := "Please verify your email"

	url := templ.URL(os.Getenv("SITE_URL") + "/verify-email/" + user.EmailToken)

	plainTextContent := fmt.Sprintf(
		`Tap the link below to finish verifying your account with Rapua. `+
			`If you didn't register, you can safely ignore this email and your email `+
			`will be automatically deleted from our system.

Verify your email: %s

Cheers,
Nathan`,
		url,
	)

	// Render the HTML email template
	w := new(bytes.Buffer)
	c := templates.VerifyEmail(url)
	err := c.Render(ctx, w)
	if err != nil {
		return fmt.Errorf("rendering email template: %w", err)
	}

	return s.sendEmail(
		fromEmail,
		fromName,
		user.Email,
		user.Name,
		subject,
		plainTextContent,
		w.String(),
		"", // no reply-to for verification emails
	)
}
