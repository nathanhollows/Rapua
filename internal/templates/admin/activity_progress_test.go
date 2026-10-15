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

// teamRow builds one row of the runs table with the two figures the page shows:
// how far the run has come, and how it is moving.
func teamRow(code string, done, total int, status services.TeamStatus) []services.LeaderBoardTeamData {
	return []services.LeaderBoardTeamData{{
		ID: code, Code: code, Name: "Team " + code, Points: 10,
		LastSeen: time.Date(2026, 10, 12, 9, 30, 0, 0, time.UTC),
		Progress: done, Total: total, Status: status, Rank: 1, HasStarted: true,
	}}
}

func teammatesTable(t *testing.T, rows []services.LeaderBoardTeamData) string {
	t.Helper()
	return render(t, templates.ActivityTeamsTable(
		models.QuestSettings{EnablePoints: true}, 3, rows, "rank", "asc",
	))
}

// The runs page measures a run, it does not count it: two runs on one quest
// need not be the same length, so the cell holds a share and never the raw
// figures behind it.
func TestActivityTeamsTable_ShowsProgressAsAShare(t *testing.T) {
	rendered := teammatesTable(t, teamRow("P001", 2, 3, services.StatusPlaying))

	// Two of three truncates to 66: a bar that rounds up would read complete
	// while a leaf is still owed.
	assert.Contains(t, rendered, "66%")
	assert.Contains(t, rendered, `class="progress w-24 progress-primary"`)
	assert.NotContains(t, rendered, "2 / 3", "the page shows the share, not the counts")
}

// A run that has done everything it owes reads complete, and the bar says so in
// its own colour rather than by sitting at full width.
func TestActivityTeamsTable_FinishedRunReadsComplete(t *testing.T) {
	rendered := teammatesTable(t, teamRow("P001", 3, 3, services.StatusFinished))

	assert.Contains(t, rendered, "100%")
	assert.Contains(t, rendered, `class="progress w-24 progress-success"`)
	assert.Contains(t, rendered, "Complete")
}

// The status column carries the cadence reading, which is the same vocabulary
// the run page uses: a run that has gone quiet says which kind of quiet.
func TestActivityTeamsTable_StatusColumn(t *testing.T) {
	cases := []struct {
		status services.TeamStatus
		want   string
	}{
		{services.StatusPlaying, "Playing"},
		{services.StatusIdle, "Idle"},
		{services.StatusStalled, "Stalled"},
		{services.StatusAbandoned, "Abandoned"},
		{services.StatusStarted, "Started"},
	}

	for _, tc := range cases {
		t.Run(string(tc.status), func(t *testing.T) {
			rendered := teammatesTable(t, teamRow("P001", 1, 3, tc.status))
			assert.Contains(t, rendered, tc.want)
		})
	}
}

// A quest with no objectives has no share to show, and a zero total must not
// divide by it.
func TestActivityTeamsTable_NoObjectives(t *testing.T) {
	rendered := render(t, templates.ActivityTeamsTable(
		models.QuestSettings{}, 0, teamRow("P001", 0, 0, services.StatusStarted), "rank", "asc",
	))

	assert.Contains(t, rendered, "No objectives")
	assert.NotContains(t, rendered, "100%")
}

// TestActivityTeamsPreview writes a browsable copy of the table when
// RAPUA_PREVIEW names a path, so the bar can be looked at rather than only
// asserted on.
func TestActivityTeamsPreview(t *testing.T) {
	path := os.Getenv("RAPUA_PREVIEW")
	if path == "" {
		t.Skip("set RAPUA_PREVIEW to write a browsable copy of the page")
	}

	rows := append(
		teamRow("P001", 2, 3, services.StatusPlaying),
		teamRow("P002", 3, 3, services.StatusFinished)...,
	)
	rows = append(rows, teamRow("P003", 0, 3, services.StatusIdle)...)
	rows = append(rows, teamRow("P004", 1, 3, services.StatusStalled)...)
	rows = append(rows, teamRow("P005", 1, 3, services.StatusAbandoned)...)
	for i := range rows {
		rows[i].Rank = i + 1
	}

	page := render(t, templates.ActivityTeamsTable(
		models.QuestSettings{EnablePoints: true}, 3, rows, "rank", "asc",
	))

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
