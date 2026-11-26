package services

import (
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func names(teams []StationTeam) []string {
	out := make([]string, 0, len(teams))
	for _, team := range teams {
		out = append(out, team.Name)
	}
	return out
}

func started(code, name string, status TeamStatus, seen time.Time) stationRun {
	return stationRun{
		Run:     models.Run{Code: code, Name: name, HasStarted: true},
		Summary: RunSummary{Status: status, LastSeen: seen},
	}
}

// Who a station is waiting for is decided from each team's own position: where it
// has been, where it can go now, where it cannot go yet, and where its route
// does not go at all. Teams that have stopped or finished are left out of every
// count that would make a facilitator wait.
func TestBuildStationReport_FilesEachTeamWhereItBelongs(t *testing.T) {
	now := time.Date(2026, 11, 21, 14, 0, 0, 0, time.UTC)
	root := models.Objective{ID: "root", Title: "Everton Induction"}
	section := models.Objective{ID: "sec", Title: "Scavenger hunt", ParentID: "root"}
	a := models.Objective{ID: "a", Title: "Mailboxes", ParentID: "sec"}
	b := models.Objective{ID: "b", Title: "BBQ Area", ParentID: "sec"}
	secret := models.Objective{ID: "s", Title: "Hidden", ParentID: "sec", Secret: true}
	inPlay := []models.Objective{root, section, a, b, secret}

	progress := func(done, locked, ruledOut []string) RunProgress {
		set := func(ids []string, id string) bool {
			for _, candidate := range ids {
				if candidate == id {
					return true
				}
			}
			return false
		}
		var nodes []RunProgressNode
		for _, objective := range []models.Objective{section, a, b} {
			nodes = append(nodes, RunProgressNode{
				Objective: objective, Done: set(done, objective.ID),
				Locked: set(locked, objective.ID), RuledOut: set(ruledOut, objective.ID),
			})
		}
		return RunProgress{Nodes: nodes}
	}

	// Has been to Mailboxes, can visit BBQ now.
	foxes := started("P1", "Red Foxes", StatusPlaying, now.Add(-2*time.Minute))
	foxes.Progress = progress([]string{"a"}, nil, nil)
	// Opened Mailboxes three minutes ago and has not finished; BBQ is not reachable yet.
	tui := started("P2", "Blue Tui", StatusPlaying, now.Add(-time.Minute))
	tui.Progress = progress(nil, []string{"b"}, nil)
	tui.Viewed = map[string]time.Time{"a": now.Add(-3 * time.Minute)}
	// Opened it an hour ago, so it is not there now.
	kea := started("P3", "Quiet Kea", StatusIdle, now.Add(-30*time.Minute))
	kea.Progress = progress(nil, nil, nil)
	kea.Viewed = map[string]time.Time{"a": now.Add(-time.Hour)}
	// Route cuts BBQ out.
	moas := started("P4", "Mighty Moas", StatusPlaying, now.Add(-5*time.Minute))
	moas.Progress = progress([]string{"a"}, nil, []string{"b"})
	// Stopped after Mailboxes; finished after BBQ; never started.
	owls := started("P5", "Night Owls", StatusAbandoned, now.Add(-3*time.Hour))
	owls.Completed = map[string]bool{"a": true}
	hens := started("P6", "Hungry Hens", StatusFinished, now.Add(-time.Hour))
	hens.Completed = map[string]bool{"b": true}
	fresh := stationRun{Run: models.Run{Code: "P7"}, Summary: RunSummary{Status: StatusNotStarted}}

	report := buildStationReport(inPlay, []stationRun{foxes, tui, kea, moas, owls, hens, fresh}, now)

	assert.Equal(t, 4, report.Playing, "playing teams only")
	assert.Equal(t, 1, report.NotStarted)
	assert.Equal(t, 1, report.Stopped)
	require.Len(t, report.Stations, 3, "neither a secret nor the quest's own root is a station")
	assert.Equal(t, "Scavenger hunt", report.Stations[0].Objective.Title)
	assert.True(t, report.Stations[0].Section)
	assert.Empty(t, report.Stations[0].Parent, "directly under the quest, so nothing to say about where it sits")
	assert.Zero(t, report.Stations[0].Depth)

	mail := report.Stations[1]
	assert.Equal(t, "Scavenger hunt", mail.Parent)
	assert.Equal(t, 1, mail.Depth)
	assert.Equal(t, []string{"Mighty Moas", "Night Owls", "Red Foxes"}, names(mail.Been),
		"cleared it, even those who then stopped")
	assert.Equal(t, []string{"Blue Tui"}, names(mail.Here))
	assert.Equal(t, now.Add(-3*time.Minute), mail.Here[0].Arrived)
	assert.Equal(t, []string{"Quiet Kea"}, names(mail.ToCome), "looked an hour ago, so on the way, not here")
	assert.True(t, mail.ToCome[0].Quiet, "unseen for half an hour")
	assert.Equal(t, 1, mail.Finished, "finished the game without visiting")
	assert.Zero(t, mail.Stopped)

	bbq := report.Stations[2]
	assert.Equal(t, []string{"Red Foxes", "Quiet Kea"}, names(bbq.ToCome), "open to both, most recently seen first")
	assert.Equal(t, []string{"Blue Tui"}, names(bbq.Later), "locked, but its route still includes it")
	assert.Equal(t, []string{"Hungry Hens"}, names(bbq.Been))
	assert.Equal(t, 1, bbq.Skipped, "its route skips this stop")
	assert.Equal(t, 1, bbq.Stopped, "stopped before getting here")
}

// An unnamed team is still told apart on the list.
func TestTeamLabel_FallsBackToTheCode(t *testing.T) {
	assert.Equal(t, "Team ABCD", teamLabel(models.Run{Code: "ABCD"}))
	assert.Equal(t, "Team ABCD", teamLabel(models.Run{Code: "ABCD", Name: "  "}))
	assert.Equal(t, "Red Foxes", teamLabel(models.Run{Code: "ABCD", Name: "Red Foxes"}))
}

func TestBuildGameClock(t *testing.T) {
	now := time.Date(2026, 11, 21, 14, 0, 0, 0, time.UTC)
	at := func(h int) bun.NullTime { return bun.NullTime{Time: time.Date(2026, 11, 21, h, 0, 0, 0, time.UTC)} }

	cases := []struct {
		name   string
		quest  models.Quest
		kind   ClockKind
		remain time.Duration
		pct    int
	}{
		{"never opened", models.Quest{}, ClockClosed, 0, 0},
		{"opens later", models.Quest{StartTime: at(16)}, ClockSoon, 0, 0},
		{"ended", models.Quest{StartTime: at(9), EndTime: at(12)}, ClockOver, 0, 0},
		{"open with a closing time", models.Quest{StartTime: at(12), EndTime: at(16)}, ClockOpen, 2 * time.Hour, 50},
		{"open with none", models.Quest{StartTime: at(12)}, ClockOpen, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clock := BuildGameClock(c.quest, now)
			assert.Equal(t, c.kind, clock.Kind)
			assert.Equal(t, c.remain, clock.Remaining)
			assert.Equal(t, c.pct, clock.Percent)
		})
	}
	assert.False(t, BuildGameClock(models.Quest{StartTime: at(12)}, now).HasEnd())
}

// The verdict is the answer to "do I have to keep waiting".
func TestStationReading_Verdict(t *testing.T) {
	open := GameClock{Kind: ClockOpen}
	team := []StationTeam{{Name: "x"}}

	cases := []struct {
		name    string
		reading StationReading
		clock   GameClock
		playing int
		want    StationVerdict
	}{
		{"teams on the way", StationReading{ToCome: team}, open, 5, VerdictStay},
		{"on the way and one here", StationReading{ToCome: team, Here: team}, open, 5, VerdictStay},
		{"only teams here", StationReading{Here: team}, open, 5, VerdictHere},
		{"here, with more behind", StationReading{Here: team, Later: team}, open, 5, VerdictStay},
		{"nobody yet, but later", StationReading{Later: team}, open, 5, VerdictLater},
		{"everyone has been", StationReading{Been: team}, open, 5, VerdictMaybe},
		{"no one playing", StationReading{}, open, 0, VerdictWait},
		{"game ended", StationReading{ToCome: team}, GameClock{Kind: ClockOver}, 0, VerdictDone},
		{"not open yet", StationReading{ToCome: team}, GameClock{Kind: ClockSoon}, 0, VerdictWait},
		{"never opened", StationReading{}, GameClock{Kind: ClockClosed}, 0, VerdictWait},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.reading.Verdict(c.clock, c.playing))
		})
	}
	assert.True(t, VerdictStay.Waiting())
	assert.True(t, VerdictLater.Waiting())
	assert.False(t, VerdictMaybe.Waiting())
	assert.False(t, VerdictDone.Waiting())
}

// A team unseen for long enough is not coming, whatever the game's own reading of
// abandonment says: that reading needs a pace to compare against, which a team
// with little history never has. Nobody waits at a station for a team nobody has
// seen since last year.
func TestBuildStationReport_GivesUpOnATeamNobodyHasSeen(t *testing.T) {
	now := time.Date(2026, 11, 21, 14, 0, 0, 0, time.UTC)
	root := models.Objective{ID: "root", Title: "Quest"}
	a := models.Objective{ID: "a", Title: "Mailboxes", ParentID: "root"}
	inPlay := []models.Objective{root, a}
	open := RunProgress{Nodes: []RunProgressNode{{Objective: a}}}

	// Started, thin history so the game never calls it abandoned, unseen for 17 months.
	ghost := started("P1", "Old Timers", StatusStarted, now.Add(-12224*time.Hour))
	ghost.Progress = open
	// Same history, but opened something four minutes ago without answering anything.
	looking := started("P2", "Window Shoppers", StatusStarted, now.Add(-12224*time.Hour))
	looking.Progress = open
	looking.Recent = now.Add(-4 * time.Minute)
	looking.Viewed = map[string]time.Time{"b": now.Add(-4 * time.Minute)}

	report := buildStationReport(inPlay, []stationRun{ghost, looking}, now)

	assert.Equal(t, 1, report.Playing, "only the team seen lately")
	assert.Equal(t, 1, report.Stopped, "the other is given up on")
	require.Len(t, report.Stations, 1)
	station := report.Stations[0]
	assert.Equal(t, []string{"Window Shoppers"}, names(station.ToCome), "never a team from last year")
	assert.Equal(t, 1, station.Stopped, "and it says where it went")
	assert.False(t, station.ToCome[0].Quiet, "recent activity counts as being seen")
}
