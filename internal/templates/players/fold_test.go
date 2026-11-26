package templates

import (
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

// A place is not foldable: it is where the run is standing, so there is nothing
// to collapse it into. The nesting it replaced is carried by the path instead,
// which costs no width and so needs no escape hatch.
func TestQuestPlace_DoesNotFold(t *testing.T) {
	rows := tree(
		obj("root", "", "Quest"),
		obj("wing", "root", "Meet the people who keep Te Aro running"),
		obj("a", "wing", "Ask at the fale"), obj("b", "wing", "Find the maps"),
	)

	html := render(t, Objectives(ObjectivesParams{
		View: viewOf([]models.Objective{rows[2], rows[3]}, rows),
	}))

	assert.NotContains(t, html, "<details")
	assert.NotContains(t, html, "objective-fold")
	assert.Contains(t, html, "Meet the people who keep Te Aro running")
	assert.Equal(t, 1, strings.Count(html, "Ask at the fale"))
}
