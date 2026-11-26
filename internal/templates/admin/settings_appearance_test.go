package templates_test

import (
	"testing"

	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

// data-theme="system" matches no theme: the page fell back to light and kept it when boosted.
func TestSettingsAppearance_ResolvesSystemInsteadOfWritingIt(t *testing.T) {
	html := render(t, templates.SettingsAppearance(models.User{}))

	assert.Contains(t, html, "prefers-color-scheme: dark")
	assert.Contains(t, html, "setAttribute('data-theme', resolved)")
	assert.NotContains(t, html, "setAttribute('data-theme', currentTheme)")
}

// show_email is retired; the handler no longer sends it, so a stored value is untouched.
func TestSettingsProfile_HasNoEmailSharingSwitch(t *testing.T) {
	html := render(t, templates.SettingsProfile(models.User{ShareEmail: true}))

	assert.NotContains(t, html, "show_email")
	assert.NotContains(t, html, "Show your email")
}

// Labels are for reading; stored values stay as saved.
func TestSettingsAppearance_LabelsThemesLightDarkAndFollowSystem(t *testing.T) {
	html := render(t, templates.SettingsAppearance(models.User{}))

	for _, label := range []string{"Light", "Dark", "Follow system theme"} {
		assert.Contains(t, html, `<span class="flex-grow text-sm">`+label+"</span>")
	}
	for _, value := range []string{"cupcake", "dracula", "system"} {
		assert.Contains(t, html, `data-theme-value="`+value+`"`)
	}
	assert.NotContains(t, html, "Cupcake")
	assert.NotContains(t, html, "Dracula")
}
