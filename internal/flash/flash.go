package flash

import (
	"encoding/gob"
	"net/http"

	"github.com/nathanhollows/Rapua/v6/internal/sessions"
)

func init() {
	gob.Register(Message{})
}

// Message is a struct containing each flashed message.
//
//nolint:recvcheck // Save uses value receiver for immutability, builder methods use pointer receiver
type Message struct {
	Title   string
	Message string
	Style   Style
}

// Style represents the visual style of a flash message.
type Style string

const (
	Default Style = ""
	Success Style = "success"
	Error   Style = "error"
	Warning Style = "warning"
	Info    Style = "info"
)

// New adds a new message into the cookie storage.
func New(w http.ResponseWriter, r *http.Request, title string, message string, style Style) error {
	flash := Message{Title: title, Message: message, Style: style}
	return flash.Save(w, r)
}

// Save adds a new message into the cookie storage.
func (m Message) Save(w http.ResponseWriter, r *http.Request) error {
	session, _ := sessions.Get(r, "scanscout")
	session.Options.HttpOnly = true
	session.Options.Secure = true
	session.Options.SameSite = http.SameSiteLaxMode
	session.AddFlash(m)
	return session.Save(r, w)
}

// SetTitle sets the title of the message.
func (m *Message) SetTitle(title string) Message {
	m.Title = title
	return *m
}

// SetMessage sets the message content.
func (m *Message) SetMessage(message string) Message {
	m.Message = message
	return *m
}

// Get flash messages from the cookie storage.
func Get(w http.ResponseWriter, r *http.Request) []interface{} {
	session, err := sessions.Get(r, "scanscout")
	if err == nil {
		messages := session.Flashes()
		if len(messages) > 0 {
			saveErr := session.Save(r, w)
			if saveErr != nil {
				return nil
			}
		}
		return messages
	}
	return nil
}

// NewDefault adds a new default message into the cookie storage.
func NewDefault(message string) *Message {
	return &Message{Title: "", Message: message, Style: Default}
}

// NewSuccess adds a new success message into the cookie storage.
func NewSuccess(message string) *Message {
	return &Message{Title: "", Message: message, Style: Success}
}

// NewError adds a new error message into the cookie storage.
func NewError(message string) *Message {
	return &Message{Title: "", Message: message, Style: Error}
}

// NewWarning adds a new warning message into the cookie storage.
func NewWarning(message string) *Message {
	return &Message{Title: "", Message: message, Style: Warning}
}

// NewInfo adds a new info message into the cookie storage.
func NewInfo(message string) *Message {
	return &Message{Title: "", Message: message, Style: Info}
}
