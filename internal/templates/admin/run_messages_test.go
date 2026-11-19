package templates_test

import (
	"strings"
	"testing"

	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

func TestAlertsList_ListsNewestFirstAndSaysWhatBecameOfEach(t *testing.T) {
	old := models.Notification{Content: "Welcome", Dismissed: true}
	recent := models.Notification{Content: "Head to the library", Dismissed: false}
	html := render(t, templates.AlertsList([]models.Notification{old, recent}))

	assert.Less(t, strings.Index(html, "Head to the library"), strings.Index(html, "Welcome"))
	assert.Contains(t, html, "Not dismissed yet")
	assert.Contains(t, html, "Dismissed")
	assert.Contains(t, html, "message-done", "a dismissed message is quieter than one still showing")
}

func TestAlertsList_SaysWhenNothingWasSent(t *testing.T) {
	assert.Contains(t, render(t, templates.AlertsList(nil)), "No messages sent yet")
}

// The column holds 255 characters, so the form stops there too.
func TestMessagesCard_ExplainsHowPlayersSeeItAndCapsTheLength(t *testing.T) {
	html := render(t, templates.MessagesCard(models.Run{Code: "ABCD"}, nil))

	assert.Contains(t, html, "alerts at the top of every page")
	assert.Contains(t, html, "They cannot reply")
	assert.Contains(t, html, `maxlength="255"`)
	assert.Contains(t, html, `name="runCode" value="ABCD"`, "the handler reads runCode, so the form has to send exactly that")
	assert.Contains(t, html, `hx-target="#alerts-list"`)
}
