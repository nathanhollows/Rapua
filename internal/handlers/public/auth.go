package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/markbates/goth/gothic"
	"github.com/nathanhollows/Rapua/helpers"
	"github.com/nathanhollows/Rapua/internal/flash"
	"github.com/nathanhollows/Rapua/internal/services"
	"github.com/nathanhollows/Rapua/internal/sessions"
	templates "github.com/nathanhollows/Rapua/internal/templates/public"
	"github.com/nathanhollows/Rapua/models"
)

// LoginHandler is the handler for the admin login page
func (h *PublicHandler) Login(w http.ResponseWriter, r *http.Request) {
	user, err := h.AuthService.GetAuthenticatedUser(r)
	if err == nil || user != nil {
		// User is already authenticated, redirect to the admin page
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	c := templates.Login(h.AuthService.AllowGoogleLogin())
	err = templates.AuthLayout(c, "Login").Render(r.Context(), w)

	if err != nil {
		h.Logger.Error("Error rendering login page", "err", err)
	}
}

// LoginPost handles the login form submission
func (h *PublicHandler) LoginPost(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	email := r.Form.Get("email")
	password := r.Form.Get("password")

	// Try to authenticate the user
	user, err := h.AuthService.AuthenticateUser(r.Context(), email, password)

	if err != nil {
		h.Logger.Error("authenticating user", "err", err)
		w.WriteHeader(http.StatusUnauthorized)
		c := templates.LoginError("Invalid email or password.")
		c.Render(r.Context(), w)
		return
	}

	session, err := sessions.NewFromUser(r, *user)
	if err != nil {
		h.Logger.Error("creating session", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		c := templates.LoginError("An error occurred while trying to log in. Please try again.")
		c.Render(r.Context(), w)
		return
	}
	session.Save(r, w)
	w.Header().Add("hx-redirect", "/admin")
}

// Logout destroys the user session
func (h *PublicHandler) Logout(w http.ResponseWriter, r *http.Request) {
	session, err := sessions.Get(r, "admin")
	if err != nil {
		h.Logger.Error("getting session for logout", "err", err)
		flash.NewError("An error occurred while trying to log out. Please try again.").Save(w, r)
		// Redirect to the login page
		http.Redirect(w, r, helpers.URL("/login"), http.StatusSeeOther)
		return
	}
	session.Options.MaxAge = -1
	session.Save(r, w)
	http.Redirect(w, r, helpers.URL("/login"), http.StatusSeeOther)
}

// RegisterHandler is the handler for the admin register page
func (h *PublicHandler) Register(w http.ResponseWriter, r *http.Request) {
	user, err := h.AuthService.GetAuthenticatedUser(r)
	if err == nil || user != nil {
		// User is already authenticated, redirect to the admin page
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	c := templates.Register(h.AuthService.AllowGoogleLogin())
	err = templates.AuthLayout(c, "Register").Render(r.Context(), w)

	if err != nil {
		h.Logger.Error("rendering register page", "err", err)
	}
}

// RegisterPostHandler handles the form submission for creating a new user
func (h *PublicHandler) RegisterPost(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	var user models.User
	user.Name = r.Form.Get("name")
	user.Email = r.Form.Get("email")
	user.Password = r.Form.Get("password")

	confirmPassword := r.Form.Get("password-confirm")

	// Create the user
	err := h.UserService.CreateUser(r.Context(), &user, confirmPassword)
	if err != nil {
		h.Logger.Error("creating user", "err", err)
		w.WriteHeader(http.StatusUnauthorized)
		if errors.Is(err, services.ErrPasswordsDoNotMatch) {
			c := templates.RegisterError("Passwords do not match.")
			c.Render(r.Context(), w)
			return
		}
		c := templates.RegisterError("Something went wrong! Please try again.")
		c.Render(r.Context(), w)
		return
	}

	// Send the email verification
	err = h.AuthService.SendEmailVerification(r.Context(), &user)
	if err != nil {
		if !errors.Is(err, services.ErrUserAlreadyVerified) {
			h.UserService.DeleteUser(r.Context(), user.ID)
			h.Logger.Error("sending email verification", "err", err)
			c := templates.RegisterError("Your account was created, but an error occurred while trying to send the email verification. Please try again.")
			c.Render(r.Context(), w)
			return
		}
	}

	c := templates.RegisterSuccess()
	c.Render(r.Context(), w)
}

// ForgotPasswordHandler is the handler for the forgot password page
func (h *PublicHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	user, err := h.AuthService.GetAuthenticatedUser(r)
	if err == nil || user != nil {
		// User is already authenticated, redirect to the admin page
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	c := templates.ForgotPassword()
	err = templates.AuthLayout(c, "Forgot Password").Render(r.Context(), w)

	if err != nil {
		h.Logger.Error("rendering forgot password page", "err", err)
	}
}

// ForgotPasswordPostHandler handles the form submission for the forgot password page
func (h *PublicHandler) ForgotPasswordPost(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement

	c := templates.ForgotMessage(
		*flash.NewInfo("If an account with that email exists, an email will be sent with instructions on how to reset your password."),
	)
	c.Render(r.Context(), w)
}

var userTemplate = `
<p><a href="/logout/{{.Provider}}">logout</a></p>
<p>Name: {{.Name}} [{{.LastName}}, {{.FirstName}}]</p>
<p>Email: {{.Email}}</p>
<p>NickName: {{.NickName}}</p>
<p>Location: {{.Location}}</p>
<p>AvatarURL: {{.AvatarURL}} <img src="{{.AvatarURL}}"></p>
<p>Description: {{.Description}}</p>
<p>UserID: {{.UserID}}</p>
<p>AccessToken: {{.AccessToken}}</p>
<p>ExpiresAt: {{.ExpiresAt}}</p>
<p>RefreshToken: {{.RefreshToken}}</p>
<p>Provider: {{.Provider}}</p>
`

// Auth redirects the user to the Google OAuth page
func (h *PublicHandler) Auth(w http.ResponseWriter, r *http.Request) {
	// Include the provider to the query string
	// since Chi doesn't do this automatically
	provider := chi.URLParam(r, "provider")
	r.URL.RawQuery = fmt.Sprintf("%s&provider=%s", r.URL.RawQuery, provider)

	_, err := h.AuthService.CompleteUserAuth(w, r)
	if err == nil {
		// User is authenticated, redirect to the admin page
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	} else {
		// Redirect user to authentication handler
		gothic.BeginAuthHandler(w, r)
	}
}

// AuthCallback handles the callback from Google OAuth
func (h *PublicHandler) AuthCallback(w http.ResponseWriter, r *http.Request) {
	// Include the provider to the query string
	// since Chi doesn't do this automatically
	provider := chi.URLParam(r, "provider")
	r.URL.RawQuery = fmt.Sprintf("%s&provider=%s", r.URL.RawQuery, provider)

	user, err := h.AuthService.CompleteUserAuth(w, r)
	if err != nil {
		h.Logger.Error("completing auth", "error", err)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if user == nil {
		h.Logger.Error("completing auth", "error", "user is nil")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	session, err := sessions.NewFromUser(r, *user)
	if err != nil {
		h.Logger.Error("creating session", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		c := templates.LoginError("An error occurred while trying to log in. Please try again.")
		c.Render(r.Context(), w)
		return
	}
	session.Save(r, w)

	w.Write([]byte(`
<!DOCTYPE html>
<html>
<head><meta http-equiv="refresh" content="0; url='/admin'"></head>
<body></body>
</html>
		`))
}

// VerifyEmail shows the user the verify email page, the first step in the email verification process
func (h *PublicHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	// If the user is authenticated without error, we will redirect them to the admin page
	user, err := h.AuthService.GetAuthenticatedUser(r)
	if err != nil && user != nil && user.EmailVerified {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	c := templates.VerifyEmail(*user)
	err = templates.AuthLayout(c, "Verify Email").Render(r.Context(), w)

	if err != nil {
		h.Logger.Error("rendering verify email page", "err", err)
	}
}

// VerifyEmailWithToken verifies the user's email address and redirects upon error or success
func (h *PublicHandler) VerifyEmailWithToken(w http.ResponseWriter, r *http.Request) {
	// If the user is authenticated without error, we will redirect them to the admin page
	user, err := h.AuthService.GetAuthenticatedUser(r)
	if err == nil && user != nil && user.EmailVerified {
		flash.NewInfo("Your email is already verified.").Save(w, r)
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	token := chi.URLParam(r, "token")

	err = h.AuthService.VerifyEmail(r.Context(), token)
	if err != nil {
		if errors.Is(err, services.ErrInvalidToken) {
			http.Redirect(w, r, "/verify-email", http.StatusSeeOther)
			return
		}
		if errors.Is(err, services.ErrTokenExpired) {
			http.Redirect(w, r, "/verify-email", http.StatusSeeOther)
			return
		}
		h.Logger.Error("verifying email", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		http.Redirect(w, r, "/verify-email", http.StatusSeeOther)
		return
	}

	// Send a meta refresh to the verify email page
	w.Write([]byte(`<html><head><meta http-equiv="refresh" content="2; url='/admin'"></head><body></body></html>`))
}

// VerifyEmailStatus checks the status of the email verification and redirects accordingly
func (h *PublicHandler) VerifyEmailStatus(w http.ResponseWriter, r *http.Request) {
	user, err := h.AuthService.GetAuthenticatedUser(r)
	if err != nil || user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if user.EmailVerified {
		w.Header().Add("HX-Redirect", "/admin")
		return
	}

	// Not verified yet
	w.WriteHeader(http.StatusUnauthorized)
}

// ResendEmailVerification resends the email verification email
func (h *PublicHandler) ResendEmailVerification(w http.ResponseWriter, r *http.Request) {
	user, err := h.AuthService.GetAuthenticatedUser(r)
	if err != nil || user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	err = h.AuthService.SendEmailVerification(r.Context(), user)
	if err != nil {
		if errors.Is(err, services.ErrUserAlreadyVerified) {
			w.WriteHeader(http.StatusUnauthorized)
			c := templates.Toast(
				*flash.NewError("Your email is already verified."),
			)
			c.Render(r.Context(), w)
			return
		}

		h.Logger.Error("sending email verification", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		c := templates.Toast(
			*flash.NewError("An error occurred while trying to send the email. Please try again."),
		)
		c.Render(r.Context(), w)
		return
	}

	w.WriteHeader(http.StatusOK)
	c := templates.Toast(
		*flash.NewSuccess("Email sent! Please check your inbox."),
	)
	c.Render(r.Context(), w)
}
