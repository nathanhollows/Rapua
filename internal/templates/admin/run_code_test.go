package templates

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// An admin reads a code aloud and copies it, so it is shown in two halves.
func TestTeamCode_IsShownInTwoHalves(t *testing.T) {
	html := renderComponent(t, teamCode("ABCDEF"))

	assert.Contains(t, html, "ABC-DEF")
	assert.NotContains(t, html, ">ABCDEF<")
}

func TestTeamCode_LeavesAnOlderCodeAlone(t *testing.T) {
	assert.Contains(t, renderComponent(t, teamCode("ABCD")), ">ABCD<")
}
