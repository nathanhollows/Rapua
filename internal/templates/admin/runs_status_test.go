package templates_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v8/internal/services"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writePreview wraps a rendered component in a page carrying the built
// stylesheet, so a file titled by RAPUA_PREVIEW can be opened in a browser.
func writePreview(t *testing.T, path, page string) {
	t.Helper()
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
<body class="bg-base-100 min-h-screen p-8">%s</body>
</html>`, css, page)
	require.NoError(t, os.WriteFile(path, []byte(doc), 0o600))
}

func listRun(code string, started bool) models.Run {
	return models.Run{
		ID: code, Code: code, Name: "Team " + code, QuestID: "q1",
		HasStarted: started, StartedAt: time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC),
	}
}

// The runs list reads each run's state from the quest's summaries: the same
// progress share and the same cadence words the activity table shows, so the
// two pages cannot tell an admin different stories about one run.
func TestTeamsTable_ShowsSummary(t *testing.T) {
	rendered := render(t, templates.TeamsTable(
		[]models.Run{listRun("P001", true), listRun("P002", true), listRun("P003", false)},
		map[string]services.RunSummary{
			"P001": {Done: 1, Total: 4, Status: services.StatusStalled},
			"P002": {Done: 4, Total: 4, Status: services.StatusFinished},
		},
	))

	assert.Contains(t, rendered, "25%")
	assert.Contains(t, rendered, "Stalled")
	assert.Contains(t, rendered, "100%")
	assert.Contains(t, rendered, "Complete")
	assert.Contains(t, rendered, "Not started")
}

// A summary is built from the stream, so a run that has begun but has nothing
// readable yet says started rather than claiming a pace it has not shown.
func TestTeamsTable_StartedRunWithoutASummary(t *testing.T) {
	rendered := render(t, templates.TeamsTable([]models.Run{listRun("P001", true)}, nil))

	assert.Contains(t, rendered, "Started")
	assert.NotContains(t, rendered, "Playing")
	assert.NotContains(t, rendered, "progress")
}

// Tests read the page without a database, so summaries are absent more often
// than they are empty; a run that never started still reads not started.
func TestTeamsTable_UnstartedRunWithoutASummary(t *testing.T) {
	rendered := render(t, templates.TeamsList([]models.Run{listRun("P001", false)}, nil))

	assert.Contains(t, rendered, "Not started")
	assert.NotContains(t, rendered, "progress")
}

// TestTeamsPreview writes a browsable copy of the list when RAPUA_PREVIEW
// names a path, so the row can be looked at rather than only asserted on.
func TestTeamsPreview(t *testing.T) {
	path := os.Getenv("RAPUA_PREVIEW")
	if path == "" {
		t.Skip("set RAPUA_PREVIEW to write a browsable copy of the page")
	}

	runs := []models.Run{
		listRun("P001", true), listRun("P002", true),
		listRun("P003", true), listRun("P004", false),
	}
	rows := render(t, templates.TeamsTable(runs, map[string]services.RunSummary{
		"P001": {Done: 1, Total: 4, Status: services.StatusPlaying},
		"P002": {Done: 4, Total: 4, Status: services.StatusFinished},
		"P003": {Done: 2, Total: 4, Status: services.StatusStalled},
	}))

	writePreview(t, path, rows)
}
