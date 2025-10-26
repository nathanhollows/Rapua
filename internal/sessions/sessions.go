package sessions

import (
	"net/http"
	"os"

	"github.com/gorilla/sessions"
	gsessions "github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/google"
	"github.com/nathanhollows/Rapua/v5/models"
)

var store sessions.Store

const (
	adminSession  = "admin"
	playerSession = "scanscout"
)

func Start() {
	store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))

	authStore := gsessions.NewCookieStore([]byte(os.Getenv("SESSION_SECRET")))
	authStore.Options.SameSite = http.SameSiteLaxMode
	authStore.Options.HttpOnly = true
	authStore.Options.Secure = true
	//nolint:reassign
	gothic.Store = authStore
	goth.UseProviders(
		google.New(
			os.Getenv("GOOGLE_CLIENT_ID"),
			os.Getenv("GOOGLE_SECRET_ID"),
			os.Getenv("SITE_URL")+"/auth/google/callback",
			"email",
			"profile",
		),
	)
}

// Get returns a session for the given request.
func GetAdmin(r *http.Request) (*sessions.Session, error) {
	return store.Get(r, adminSession)
}

// Get returns a session for the given request.
func GetPlayer(r *http.Request) (*sessions.Session, error) {
	return store.Get(r, playerSession)
}

// Get returns a session for the given request.
func Get(r *http.Request, name string) (*sessions.Session, error) {
	return store.Get(r, name)
}

// NewFromTeam session for the given request and team.
func NewFromTeam(r *http.Request, team models.Team) (*sessions.Session, error) {
	session, err := store.Get(r, playerSession)
	if err != nil {
		return nil, err
	}

	session.Values["team"] = team.Code
	session.Options.Secure = true
	session.Options.SameSite = http.SameSiteLaxMode

	return session, nil
}

// NewFromUser session for the given request and user.
func NewFromUser(r *http.Request, user models.User) (*sessions.Session, error) {
	session, err := store.Get(r, adminSession)
	if err != nil {
		return nil, err
	}

	session.Values["user_id"] = user.ID
	session.Options.Secure = true
	session.Options.SameSite = http.SameSiteLaxMode

	return session, nil
}
