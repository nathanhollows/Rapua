package services

// This file exports internal functions for testing only.
// It's only compiled during tests and allows the test package to use services_test.

// EmailMessage is a test helper to create email messages.
type EmailMessage struct {
	From        string
	FromName    string
	To          string
	ToName      string
	Subject     string
	PlainText   string
	HTMLContent string
	ReplyTo     string
}

// BuildEmailMessage exports buildEmailMessage for testing with exported struct.
func BuildEmailMessage(msg EmailMessage, smtpHost string) ([]byte, error) {
	internal := emailMessage{
		from:        msg.From,
		fromName:    msg.FromName,
		to:          msg.To,
		toName:      msg.ToName,
		subject:     msg.Subject,
		plainText:   msg.PlainText,
		htmlContent: msg.HTMLContent,
		replyTo:     msg.ReplyTo,
	}
	return buildEmailMessage(internal, smtpHost)
}

// SendEmail exports sendEmail for testing.
func (s EmailService) SendEmail(from, fromName, to, toName, subject, plainText, htmlContent, replyTo string) error {
	return s.sendEmail(from, fromName, to, toName, subject, plainText, htmlContent, replyTo)
}
