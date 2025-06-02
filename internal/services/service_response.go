package services

import (
	"github.com/nathanhollows/Rapua/v3/internal/flash"
)

type ServiceResponse struct {
	FlashMessages []flash.Message
	Error         error
	Data          map[string]interface{}
}

// Add a flash message to the response.
func (r *ServiceResponse) AddFlashMessage(message flash.Message) {
	r.FlashMessages = append(r.FlashMessages, message)
}

// Check if there is an error.
func (r *ServiceResponse) HasError() bool {
	return r.Error != nil
}

// Check if there is a flash message.
func (r *ServiceResponse) HasFlash() bool {
	return len(r.FlashMessages) > 0
}
