package flash

import (
	"encoding/gob"
	"net/http"

	"github.com/nathanhollows/Rapua/v4/internal/sessions"
)

func init() {
	gob.Register(Message{})
}

// Message is a struct containing each flashed message.
type Message struct {
	Title   string
	Message string
	Style   FlashStyle
}

type FlashStyle string

const (
	Default FlashStyle = ""
	Success FlashStyle = "success"
	Error   FlashStyle = "error"
	Warning FlashStyle = "warning"
	Info    FlashStyle = "info"
)

// New adds a new message into the cookie storage.
func New(w http.ResponseWriter, r *http.Request, title string, message string, style FlashStyle) error {
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

// Set the title of the message.
func (m *Message) SetTitle(title string) Message {
	m.Title = title
	return *m
}

// Set the message of the message.
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
			err := session.Save(r, w)
			if err != nil {
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
