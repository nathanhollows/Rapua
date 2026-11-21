package templates

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/internal/templates/brand"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderComponent(t *testing.T, c templ.Component) string {
	t.Helper()
	var out strings.Builder
	require.NoError(t, c.Render(context.Background(), &out))
	return out.String()
}

func station(id, title string, depth int, section bool) services.StationReading {
	return services.StationReading{Objective: models.Objective{ID: id, Title: title}, Depth: depth, Section: section}
}

func facilitatorData(
	clock services.GameClock, stations []services.StationReading, watching ...string,
) FacilitatorDashboardData {
	watch := map[string]bool{}
	for _, id := range watching {
		watch[id] = true
	}
	return FacilitatorDashboardData{
		QuestName: "Everton Induction",
		Now:       time.Date(2026, 11, 21, 14, 0, 0, 0, time.UTC),
		Clock:     clock,
		Report:    services.StationReport{Playing: 21, NotStarted: 3, Stopped: 2, Stations: stations},
		Watching:  watch,
	}
}

var openUntil = services.GameClock{
	Kind: services.ClockOpen, Opens: time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC),
	Closes: time.Date(2026, 11, 21, 15, 30, 0, 0, time.UTC), Remaining: 90 * time.Minute, Percent: 52,
}

// A facilitator is told, in order, how long is left, whether the stations they
// look after still have teams coming, and why.
func TestFacilitatorDashboard_AnswersTheThreeQuestions(t *testing.T) {
	waiting := station("a", "Mailboxes", 1, false)
	waiting.Parent = "Scavenger hunt"
	waiting.ToCome = []services.StationTeam{
		{Name: "Red Foxes", LastSeen: time.Date(2026, 11, 21, 13, 58, 0, 0, time.UTC)},
		{Name: "Team Marmite", LastSeen: time.Date(2026, 11, 21, 13, 30, 0, 0, time.UTC), Quiet: true},
	}
	waiting.Here = []services.StationTeam{{Name: "Blue Tui", Arrived: time.Date(2026, 11, 21, 13, 57, 0, 0, time.UTC)}}
	waiting.Been = []services.StationTeam{{Name: "Hungry Hens"}}
	waiting.Finished = 4
	waiting.Skipped = 1
	finished := station("b", "BBQ Area", 1, false)
	finished.Been = []services.StationTeam{{Name: "Hungry Hens"}}
	unwatched := station("c", "Laundry Room", 1, false)

	html := renderComponent(t, FacilitatorDashboard(
		facilitatorData(openUntil, []services.StationReading{waiting, finished, unwatched}, "a", "b")))

	// How long have I got: the activity page's banner, in its words, with a live count.
	assert.Contains(t, html, "The game is open")
	assert.Contains(t, html, `It closes at <time datetime="2026-11-21T15:30:00Z" data-local>`)
	assert.NotContains(t, html, "@localDate", "a component call written into text is printed, not run")
	assert.Contains(t, html, "Closes in")
	assert.Contains(t, html, `data-countdown="2026-11-21T15:30:00Z"`)
	assert.Contains(t, html, "game-chip", "the same status chip as the activity page")
	assert.Contains(t, html, "21 teams playing, 3 not started yet.")
	assert.Contains(t, html, "2 teams have stopped playing and aren’t counted below.")
	assert.NotContains(t, html, "Game open", "the state is said once")

	// Do I have to keep waiting.
	assert.Contains(t, html, "Watching 2 stations")
	assert.Contains(t, html, "1 of your 2 stations still has teams coming.")
	assert.Contains(t, html, "3 teams to come", "the two on the way and the one here")
	assert.Contains(t, html, "Keep waiting")
	assert.Contains(t, html, "Probably done", "BBQ has had everyone, but the game is open")
	assert.Contains(t, html, "3 teams have not started yet and could still arrive before the game closes.")

	// Why: who, and who is left out.
	assert.Contains(t, html, "Red Foxes")
	assert.Contains(t, html, "may have stopped", "a team unseen for a while is not waited on blindly")
	assert.Contains(t, html, "arrived 3 min ago")
	assert.Contains(t, html, "Not counted: 4 teams have finished the game, 1 team whose route skips this stop.")

	// Only what is followed gets a row. The rest is only in the picker.
	assert.Contains(t, html, `data-station="a"`)
	assert.NotContains(t, html, `data-station="c"`)
	assert.Contains(t, html, `<h1 class="text-4xl font-black">Your stations</h1>`)
}

func TestFacilitatorDashboard_AsksWhichStationsOnTheFirstVisit(t *testing.T) {
	html := renderComponent(t, FacilitatorDashboard(
		facilitatorData(openUntil, []services.StationReading{station("a", "Mailboxes", 1, false)})))

	assert.Contains(t, html, "Which stations are you looking after?")
	assert.NotContains(t, html, "data-station=", "no row until one is chosen")
	assert.NotContains(t, html, "Watching")
}

// The game's own state decides what every station says, whoever is due.
func TestFacilitatorDashboard_SaysWhenTheGameIsOverOrNotOpen(t *testing.T) {
	waiting := station("a", "Mailboxes", 1, false)
	waiting.ToCome = []services.StationTeam{{Name: "Red Foxes"}}

	over := renderComponent(t, FacilitatorDashboard(facilitatorData(
		services.GameClock{Kind: services.ClockOver, Closes: time.Date(2026, 11, 21, 13, 48, 0, 0, time.UTC)},
		[]services.StationReading{waiting}, "a")))
	assert.Contains(t, over, "The game is closed")
	assert.Contains(t, over, "It closed 12 min ago. Nobody else can play.")
	assert.NotContains(t, over, `role="timer"`, "nothing left to count down to")
	assert.Contains(t, over, "You can go")
	assert.Contains(t, over, "Nothing more is due. You can go.")

	soon := renderComponent(t, FacilitatorDashboard(facilitatorData(
		services.GameClock{Kind: services.ClockSoon, Opens: time.Date(2026, 11, 21, 14, 25, 0, 0, time.UTC)},
		[]services.StationReading{waiting}, "a")))
	assert.Contains(t, soon, "Not open yet")
	assert.Contains(t, soon, `It opens at <time datetime="2026-11-21T14:25:00Z" data-local>`)
	assert.Contains(t, soon, "Opens in")
	assert.Contains(t, soon, "game-scheduled")
	assert.Contains(t, soon, "Waiting for the game to open.")
	assert.NotContains(t, soon, "Keep waiting")

	noEnd := renderComponent(t, FacilitatorDashboard(facilitatorData(
		services.GameClock{Kind: services.ClockOpen}, []services.StationReading{waiting}, "a")))
	assert.Contains(t, noEnd, "The game is open")
	assert.Contains(t, noEnd, "There is no closing time set. Ask the organiser when it finishes.")
	assert.NotContains(t, noEnd, `role="timer"`)
	assert.NotContains(t, noEnd, "Open, with no closing time", "not said twice")
}

// The picker shows the quest as it is nested, and can tick a section's contents.
func TestFacilitatorDashboard_PickerShowsTheQuestsNesting(t *testing.T) {
	html := renderComponent(t, FacilitatorDashboard(facilitatorData(openUntil, []services.StationReading{
		station("sec", "Scavenger hunt", 0, true),
		station("a", "Mailboxes", 1, false),
		station("b", "BBQ Area", 1, false),
		station("other", "Welcome", 0, false),
	}, "a")))

	assert.Contains(t, html, `id="fv-picker"`)
	assert.Contains(t, html, `data-watching="`+services.StationKey("a")+`"`)
	assert.Contains(t, html, `padding-left:0.80rem`, "top level")
	assert.Contains(t, html, `padding-left:2.20rem`, "one level in")
	assert.Equal(t, 1, strings.Count(html, `class="fv-all"`), "only a section has an inside")
	assert.Contains(t, html, `data-inside="`+services.StationKey("a")+","+services.StationKey("b")+`"`,
		"everything under the section, and no more")
	assert.Contains(t, html, `value="`+services.StationKey("a")+`" checked`, "what is followed is ticked")
	assert.NotContains(t, html, `value="`+services.StationKey("b")+`" checked`)
	assert.NotContains(t, html, `value="a"`, "the picker speaks in keys, not whole IDs")
	assert.Contains(t, html, "data-pick-limit", "a choice too long for a cookie is said aloud")
	assert.Contains(t, html, "Search stations")
	assert.Contains(t, html, "rapua_watch", "the choice is kept where the server reads it")
}

// A facilitator is not an admin, so the page is framed without the admin bar,
// and only an admin who opens the link is offered the way back.
func TestFacilitatorLayout_OffersAnAdminTheWayBack(t *testing.T) {
	body := templ.NopComponent

	visitor := renderComponent(t, FacilitatorLayout(body, "Facilitator Dashboard", false))
	assert.Contains(t, visitor, `href="/login"`)
	assert.NotContains(t, visitor, `href="/admin"`)
	assert.NotContains(t, visitor, "topnav-tabs", "no admin navigation")

	admin := renderComponent(t, FacilitatorLayout(body, "Facilitator Dashboard", true))
	assert.Contains(t, admin, `href="/admin" class="btn btn-sm btn-primary"`)
	assert.Contains(t, admin, "admin.css", "the same stylesheet as the admin pages")
}

// The header's logo is the admin bar's own, mark and name, and it hops when
// pressed. It is a button: a link would leave the page before the hop was seen.
func TestFacilitatorLayout_HasTheAdminBarsLogoAndItsHop(t *testing.T) {
	html := renderComponent(t, FacilitatorLayout(templ.NopComponent, "Facilitator Dashboard", false))

	assert.Contains(t, html, renderComponent(t, brand.Mark()), "the very same markup as the admin bar")
	assert.Contains(t, html, `<button type="button" class="logo" aria-label="Rapua">`)
	assert.NotContains(t, html, `class="logo" aria-label="Back to Rapua"`, "no link to navigate away mid-hop")
	assert.Contains(t, html, "window.__logoHop", "the script that plays the hop on a click")
}

// The link modal says what the link opens, and the copy modal keeps the link
// readable but not editable.
func TestFacilitatorModals(t *testing.T) {
	link := renderComponent(t, FacilitatorLinkModal())
	assert.Contains(t, link, "how many teams have started", "the copy matches what the page shows")
	assert.Equal(t, 1, strings.Count(link, `name="duration"`), "one name, not two")

	copyModal := renderComponent(t, FacilitatorLinkCopyModal("https://example.test/facilitator/login/abc"))
	assert.Contains(t, copyModal, `value="https://example.test/facilitator/login/abc"`)
	assert.Contains(t, copyModal, "readonly")
}

// A time is never counted in thousands of hours.
func TestAgo(t *testing.T) {
	now := time.Date(2026, 11, 21, 14, 0, 0, 0, time.UTC)
	cases := map[string]time.Duration{
		"just now":         20 * time.Second,
		"4 min ago":        4 * time.Minute,
		"1 hour ago":       90 * time.Minute,
		"5 hours ago":      5 * time.Hour,
		"yesterday":        30 * time.Hour,
		"3 days ago":       80 * time.Hour,
		"over 2 weeks ago": 12224 * time.Hour,
	}
	for want, d := range cases {
		assert.Equal(t, want, ago(now.Add(-d), now), want)
	}
	assert.Empty(t, ago(time.Time{}, now), "no time, no words")
}
