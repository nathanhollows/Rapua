package templates_test

import (
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v8/internal/flash"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun/schema"
)

func gameQuest(start, end time.Time) models.Quest {
	return models.Quest{StartTime: schema.NullTime{Time: start}, EndTime: schema.NullTime{Time: end}}
}

func TestGameControl_ClosedOffersStartAndSchedule(t *testing.T) {
	html := render(t, templates.GameControl(gameQuest(time.Time{}, time.Time{})))

	assert.Contains(t, html, `id="schedule-status"`)
	assert.Contains(t, html, "The game is closed")
	assert.Contains(t, html, `hx-get="/admin/schedule/start"`)
	assert.NotContains(t, html, `hx-get="/admin/schedule/stop"`)
	assert.NotContains(t, html, `data-countdown="`)
}

func TestGameControl_ScheduledCountsDownToTheStart(t *testing.T) {
	start := time.Now().Add(48 * time.Hour)
	html := render(t, templates.GameControl(gameQuest(start, start.Add(8*time.Hour))))

	assert.Contains(t, html, "Opens in")
	assert.Contains(t, html, `data-countdown="`)
	assert.Contains(t, html, "data-starts-at")
	assert.Contains(t, html, "Start now")
	assert.Contains(t, html, "Edit schedule")
	assert.Contains(t, html, "It closes at")
}

func TestGameControl_ActiveWithoutAnEndOffersStopAndSchedule(t *testing.T) {
	html := render(t, templates.GameControl(gameQuest(time.Now().Add(-time.Hour), time.Time{})))

	assert.Contains(t, html, "The game is open")
	assert.Contains(t, html, `hx-get="/admin/schedule/stop"`)
	assert.NotContains(t, html, `data-countdown="`)
	assert.NotContains(t, html, "Edit schedule")
}

func TestGameControl_ActiveWithAnEndCountsDownToIt(t *testing.T) {
	html := render(t, templates.GameControl(gameQuest(time.Now().Add(-time.Hour), time.Now().Add(3*time.Hour))))

	assert.Contains(t, html, "Closes in")
	assert.Contains(t, html, `data-countdown="`)
	assert.Contains(t, html, "Edit schedule")
}

// Saving posts the UTC fields the script fills in, into the card it replaces.
func TestGameControl_ScheduleFormPostsIntoTheCard(t *testing.T) {
	html := render(t, templates.GameControl(gameQuest(time.Time{}, time.Time{})))

	assert.Contains(t, html, `hx-post="/admin/schedule/"`)
	assert.Contains(t, html, `hx-target="#schedule-status"`)
	for _, name := range []string{"utc_start_date", "utc_start_time", "utc_end_date", "utc_end_time", "set_start", "set_end"} {
		assert.Contains(t, html, `name="`+name+`"`)
	}
}

// The feed's slot is always in the page so Start can swap it in, but holds
// nothing unless the game is live.
func TestJustNowSlot_OnlyHoldsTheFeedWhileTheGameIsLive(t *testing.T) {
	live := render(t, templates.JustNowSlot(gameQuest(time.Now().Add(-time.Hour), time.Time{}), false))
	closed := render(t, templates.JustNowSlot(gameQuest(time.Time{}, time.Time{}), false))
	scheduled := render(t, templates.JustNowSlot(gameQuest(time.Now().Add(time.Hour), time.Time{}), false))

	assert.Contains(t, live, `hx-get="/admin/activity/recent"`)
	assert.Contains(t, live, "every 10s")
	for _, html := range []string{closed, scheduled} {
		assert.Contains(t, html, `id="just-now"`)
		assert.NotContains(t, html, "recent")
		assert.NotContains(t, html, "Just now")
	}
}

func TestJustNowSlot_SwapsOutOfBandWhenAsked(t *testing.T) {
	assert.Contains(t, render(t, templates.JustNowSlot(gameQuest(time.Time{}, time.Time{}), true)), `hx-swap-oob="true"`)
	assert.NotContains(t, render(t, templates.JustNowSlot(gameQuest(time.Time{}, time.Time{}), false)), "hx-swap-oob")
}

func TestRecentActivity_NamesTheTeamAndLinksToItsRun(t *testing.T) {
	entries := []services.ActivityEntry{
		{RunCode: "HKAH", RunName: "Haka Hustlers", RunLogEntry: services.RunLogEntry{Kind: services.RunLogAnswer, BlockName: "Scan", Title: "Resource Centre", Points: 30, Time: time.Now()}},
		{RunCode: "PQRS", RunLogEntry: services.RunLogEntry{Kind: services.RunLogStart, Time: time.Now()}},
	}
	html := render(t, templates.RecentActivity(entries, true))

	assert.Contains(t, html, "Haka Hustlers")
	assert.Contains(t, html, "Scan in Resource Centre")
	assert.Contains(t, html, "+30")
	assert.Contains(t, html, `href="/admin/runs/HKAH"`)
	assert.Contains(t, html, `hx-boost="true"`, "the page is not boosted as a whole, so the list opts in")
	assert.Contains(t, html, "data-ago", "the age is ticked by script between fetches")
	assert.Contains(t, html, "PQRS", "an unnamed team shows its code")
	assert.Contains(t, html, "Started playing")
}

func TestRecentActivity_HidesPointsWhenTheQuestDoesNotScore(t *testing.T) {
	entries := []services.ActivityEntry{{RunCode: "HKAH", RunLogEntry: services.RunLogEntry{Kind: services.RunLogAnswer, Points: 30, Time: time.Now()}}}

	assert.NotContains(t, render(t, templates.RecentActivity(entries, false)), "+30")
}

func TestRecentActivity_SaysWhenThereIsNothingYet(t *testing.T) {
	assert.Contains(t, render(t, templates.RecentActivity(nil, true)), "Nothing yet")
}

func TestGameControl_CarriesTheFlashMessage(t *testing.T) {
	html := render(t, templates.GameControl(gameQuest(time.Time{}, time.Time{}), *flash.NewSuccess("Game started!")))

	assert.Contains(t, html, "Game started!")
}
