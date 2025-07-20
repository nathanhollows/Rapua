package services

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/a-h/templ"
	templates "github.com/nathanhollows/Rapua/v4/internal/templates/emails"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

type EmailService struct{}

func NewEmailService() *EmailService {
	return &EmailService{}
}

func (s EmailService) SendContactEmail(ctx context.Context, name, contactEmail, content string) error {
	sentFrom := mail.NewEmail("Rapua Contact Form", os.Getenv("CONTACT_EMAIL"))
	sentTo := mail.NewEmail("Rapua", os.Getenv("CONTACT_EMAIL"))
	subject := "New message from Rapua contact form"

	htmlTemplate := `
	<p><strong>Name:</strong> %v</p>
	<p><strong>Email:</strong> %v</p>
	<p>%v</p>
	`

	message := mail.NewSingleEmail(sentFrom, subject, sentTo, content, fmt.Sprintf(htmlTemplate, name, contactEmail, content))

	client := sendgrid.NewSendClient(os.Getenv("SENDGRID_API_KEY"))
	_, err := client.Send(message)
	return err
}

func (s EmailService) SendVerificationEmail(ctx context.Context, user models.User) error {
	from := mail.NewEmail(os.Getenv("SENDGRID_FROM_NAME"), os.Getenv("SENDGRID_FROM_EMAIL"))
	to := mail.NewEmail(user.Name, user.Email)
	subject := "Please verify your email"

	url := templ.URL(os.Getenv("SITE_URL") + "/verify-email/" + user.EmailToken)

	plainTextContent := `Tap the link below to finish verifying your account with Rapua. If you didn't register, you can safely ignore this email and your email will be automatically deleted from our system.
Verify your email

	%v

Cheers,
Nathan`
	plainTextContent = fmt.Sprintf(plainTextContent, url)

	// Render the html email template
	w := new(bytes.Buffer)
	c := templates.VerifyEmail(url)
	err := c.Render(ctx, w)
	if err != nil {
		return fmt.Errorf("rendering email template: %w", err)
	}

	message := mail.NewSingleEmail(from, subject, to, plainTextContent, w.String())
	client := sendgrid.NewSendClient(os.Getenv("SENDGRID_API_KEY"))
	_, err = client.Send(message)
	return err
}
