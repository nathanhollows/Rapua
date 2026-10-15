package templates_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runViewFixture is a small run with a pace worth drawing: a start, views,
// two submissions to one block, a completion, a message, a section close and
// the finish.
func runViewFixture() templates.TeamOverviewData {
	base := time.Date(2026, 10, 11, 18, 4, 0, 0, time.UTC)
	at := func(minutes int) time.Time { return base.Add(time.Duration(minutes) * time.Minute) }
	payload := func(value any) json.RawMessage {
		data, _ := json.Marshal(value)
		return data
	}

	objectives := []models.Objective{
		{ID: "root", Slug: "the-quest", Title: "The quest"},
		{ID: "campus", Slug: "campus", Title: "Campus", ParentID: "root"},
		{ID: "o1", Slug: "the-library", Title: "The library", ParentID: "campus"},
		{ID: "cafe", Slug: "the-cafe", Title: "The cafe", ParentID: "campus"},
	}

	events := []models.RunEvent{
		{Type: models.EventRunStarted, CreatedAt: at(0)},
		{
			Type: models.EventRunNamed, CreatedAt: at(1),
			Data: payload(models.RunNamedData{Name: "Leeroy Jenkins"}),
		},
		{Type: models.EventObjectiveViewed, ObjectiveID: "o1", CreatedAt: at(4)},
		{
			Type: models.EventBlockAnswered, ObjectiveID: "o1", BlockID: "b1", CreatedAt: at(6),
			Data: payload(models.BlockAnsweredData{BlockType: "quiz"}),
		},
		{
			Type: models.EventBlockAnswered, ObjectiveID: "o1", BlockID: "b1", CreatedAt: at(8),
			Data:   payload(models.BlockAnsweredData{BlockType: "quiz", Completed: true, PointsAwarded: 10}),
			Points: 10,
		},
		{
			Type: models.EventObjectiveContextCompleted, ObjectiveID: "o1", CreatedAt: at(8),
			Data: payload(models.ObjectiveContextCompletedData{Context: game.ContextObjectiveProof, Points: 10}),
		},
		{
			Type: models.EventNotificationSent, CreatedAt: at(20),
			Data: payload(models.NotificationData{NotificationID: "n1", Content: "Head to the quad"}),
		},
		{Type: models.EventSectionFinished, ObjectiveID: "campus", CreatedAt: at(30)},
		{Type: models.EventRunCompleted, CreatedAt: at(40)},
	}

	return templates.TeamOverviewData{
		Quest: models.Quest{
			ID:         "q1",
			Objectives: objectives,
			Settings:   models.QuestSettings{EnablePoints: true},
		},
		Run: models.Run{
			Code: "DWMD", Name: "Leeroy Jenkins", QuestID: "q1",
			HasStarted: true, StartedAt: at(0), Points: 10,
		},
		RunLog: services.BuildRunLog(
			events, objectives, 4*time.Minute, at(45),
		),
		Progress: services.BuildRunProgress(
			objectives,
			map[string]bool{"o1": true},
			services.RunActivity{
				Touched:  map[string]bool{"o1": true},
				Closed:   map[string]bool{"campus": true},
				Finished: true,
			},
			nil,
		),
		Uploads: []*models.Upload{},
	}
}

func render(t *testing.T, component templ.Component) string {
	t.Helper()
	var page bytes.Buffer
	require.NoError(t, component.Render(context.Background(), &page))
	return page.String()
}

// The page draws the run from its stream: the strip, the log rows and the
// filters all render from the events and nothing else.
func TestTeamOverview_DrawsTheRunFromItsStream(t *testing.T) {
	rendered := render(t, templates.TeamOverview(runViewFixture()))

	for _, want := range []string{
		"DWMD",
		"Leeroy Jenkins",
		"run-heat",
		"Timeline",
		"The quest",
		"Drafts are not shown: players never meet them.",
		"?show=progress",
		"?show=messages",
		"?show=navigation",
		"Run started",
		"Finished the run",
		"Finished section",
		"Message received",
		"Head to the quad",
		"Quiz",
		"attempt 2",
		"+10",
		"not completed",
		"entries, the whole run",
		"Complete",
		"The run is complete.",
		"The cafe",
		"1 ∕ 1",
		"Between actions · quest avg 4m",
		"Stalled for 22m",
		"Idle for 10m",
	} {
		assert.Contains(t, rendered, want)
	}

	// The strip classifies against the run's own three-minute pace and stops
	// at the finish: the early waits play, the twenty-two-minute one is
	// stalled, and the ten-minute one idle.
	for _, band := range []string{
		`title="playing, 8m"`,
		`title="stalled, 22m"`,
		`title="idle, 10m"`,
	} {
		assert.Contains(t, rendered, band)
	}
}

// A run that crosses midnight wears the day beside its clocks, and one that
// crosses a Monday wears the date, because "Mon" alone no longer says which
// week it was.
func TestRunLogCard_ClocksWearDaysWhenTheRunOutgrowsADay(t *testing.T) {
	base := time.Date(2026, 10, 12, 18, 4, 0, 0, time.UTC)

	events := func(last time.Time) []models.RunEvent {
		return []models.RunEvent{
			{Type: models.EventRunStarted, CreatedAt: base},
			{Type: models.EventObjectivesListed, CreatedAt: last},
		}
	}
	card := func(last time.Time, now time.Time) string {
		log := services.BuildRunLog(events(last), nil, 0, now)
		return render(t, templates.RunLogCard(log, "", "DWMD"))
	}

	day := base.Add(2 * time.Hour)
	assert.Contains(t, card(day, day),
		day.Local().Format("3:04 PM"), "one day runs on bare clocks")
	assert.NotContains(t, card(day, day), day.Local().Format("Mon 3:04 PM"))

	second := base.Add(48 * time.Hour)
	assert.Contains(t, card(second, second.Add(time.Hour)),
		second.Local().Format("Mon 3:04 PM"), "a second day rides with the clock")

	next := base.Add(8 * 24 * time.Hour)
	assert.Contains(t, card(next, next.Add(time.Hour)),
		next.Local().Format("2 Jan 3:04 PM"), "past a Monday, the date")
}

// The open quiet's foot is now, which is always past an unfinished run's
// capped axis, so its row has to clamp each end on its own. Clamping the end
// and taking the start from it would fold the clip into the start and load the
// strip with a window over everything.
func TestRunLogCard_OpenQuietCoversItsOwnStretch(t *testing.T) {
	base := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	var events []models.RunEvent
	for i := range 5 {
		events = append(events, models.RunEvent{
			Type: models.EventObjectivesListed, CreatedAt: base.Add(time.Duration(i) * time.Minute),
		})
	}

	log := services.BuildRunLog(events, nil, 0, base.Add(4*time.Minute+2*time.Hour))
	card := render(t, templates.RunLogCard(log, "", "DWMD"))

	// Four minutes played, then two hours quiet: the row starts where the
	// quiet did, four minutes in, and stops at the axis, which the tail's
	// drawn width ends.
	assert.Contains(t, card, `data-t0="240"`)
	assert.Contains(t, card, `data-t1="480"`)
	assert.Contains(t, card, "Abandoned for 2h")
}

// The filters narrow the log to one question.
func TestRunLogCard_FiltersTheLog(t *testing.T) {
	data := runViewFixture()

	progress := render(t, templates.RunLogCard(data.RunLog, "progress", "DWMD"))
	assert.Contains(t, progress, "Quiz")
	assert.Contains(t, progress, "Finished the run")
	assert.NotContains(t, progress, "Message received")

	messages := render(t, templates.RunLogCard(data.RunLog, "messages", "DWMD"))
	assert.Contains(t, messages, "Message received")
	assert.NotContains(t, messages, "Quiz")
	assert.NotContains(t, messages, "Finished the run")
}

// TestRenderRunViewPreview writes a browsable copy of the page when
// RAPUA_PREVIEW names a path, so the layout can be looked at rather than only
// asserted on.
func TestRenderRunViewPreview(t *testing.T) {
	path := os.Getenv("RAPUA_PREVIEW")
	if path == "" {
		t.Skip("set RAPUA_PREVIEW to write a browsable copy of the page")
	}

	page := render(t, templates.TeamOverview(runViewFixture()))

	wd, err := os.Getwd()
	require.NoError(t, err)
	css := "file://" + filepath.Join(wd, "..", "..", "..", "static", "css", "tailwind.css")

	doc := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" data-theme="dracula">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="%s">
</head>
<body class="bg-base-100 min-h-screen">%s</body>
</html>`, css, page)
	require.NoError(t, os.WriteFile(path, []byte(doc), 0o600))
}
