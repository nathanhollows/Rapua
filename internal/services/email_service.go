package services

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/a-h/templ"
	templates "github.com/nathanhollows/Rapua/v3/internal/templates/emails"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/sendgrid/rest"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

type EmailService interface {
	// SendVerificationEmail sends a verification email to the user to complete their registration
	SendVerificationEmail(ctx context.Context, user models.User) (*rest.Response, error)
	// SendContactEmail sends an email to the site owner from the contact form
	SendContactEmail(ctx context.Context, name, contactEmail, content string) (*rest.Response, error)
}

type emailService struct{}

func NewEmailService() EmailService {
	return &emailService{}
}

func (s emailService) SendContactEmail(ctx context.Context, name, contactEmail, content string) (*rest.Response, error) {
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
	response, err := client.Send(message)
	return response, err
}

func (s emailService) SendVerificationEmail(ctx context.Context, user models.User) (*rest.Response, error) {
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
		return nil, fmt.Errorf("rendering email template: %w", err)
	}

	message := mail.NewSingleEmail(from, subject, to, plainTextContent, w.String())
	client := sendgrid.NewSendClient(os.Getenv("SENDGRID_API_KEY"))
	response, err := client.Send(message)
	return response, err
}
