package templates

import (
	"fmt"
	"strings"
	"time"

	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
)

// FacilitatorDashboardData is everything the facilitator's page reads. Stations
// holds what the link covers, in the quest's order; Watching is the ones this
// facilitator chose to follow.
type FacilitatorDashboardData struct {
	QuestName string
	Now       time.Time
	Clock     services.GameClock
	Report    services.StationReport
	Watching  map[string]bool
}

// Watched is the covered stations the facilitator is following, in order.
func (d FacilitatorDashboardData) Watched() []services.StationReading {
	var out []services.StationReading
	for _, station := range d.Report.Stations {
		if d.Watching[station.Objective.ID] {
			out = append(out, station)
		}
	}
	return out
}

// WatchedKeys is the choice as the picker keeps it: short keys, not whole IDs.
func (d FacilitatorDashboardData) WatchedKeys() string {
	var keys []string
	for _, station := range d.Watched() {
		keys = append(keys, services.StationKey(station.Objective.ID))
	}
	return strings.Join(keys, ",")
}

// inside lists, by key, what is nested under the station at index i, so a
// section can be ticked as a whole.
func (d FacilitatorDashboardData) inside(i int) string {
	stations := d.Report.Stations
	var keys []string
	for _, next := range stations[i+1:] {
		if next.Depth <= stations[i].Depth {
			break
		}
		keys = append(keys, services.StationKey(next.Objective.ID))
	}
	return strings.Join(keys, ",")
}

// teams is "1 team", "3 teams".
func teams(n int) string {
	if n == 1 {
		return "1 team"
	}
	return fmt.Sprintf("%d teams", n)
}

func teamsHave(n int) string {
	if n == 1 {
		return "1 team has"
	}
	return fmt.Sprintf("%d teams have", n)
}

func teamsAre(n int) string {
	if n == 1 {
		return "1 team is"
	}
	return fmt.Sprintf("%d teams are", n)
}

const day = 24 * time.Hour

// ago is how long since, to the nearest sensible unit, for a glance. It stops
// counting in hours at a day: nobody needs "12224 hours".
func ago(when, now time.Time) string {
	if when.IsZero() {
		return ""
	}
	d := now.Sub(when)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 2*time.Hour:
		return "1 hour ago"
	case d < day:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	case d < 2*day:
		return "yesterday"
	case d < 14*day:
		return fmt.Sprintf("%d days ago", int(d/day))
	}
	return "over 2 weeks ago"
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// stationCopy is what a station says about itself, in plain words.
type stationCopy struct {
	Class string
	Icon  string
	// Pill is the short status on the collapsed row.
	Pill string
	// Tag, Title and Text are the verdict once opened.
	Tag, Title, Text string
}

// stationCopyFor words a station's verdict. Everything a facilitator reads here
// is meant to be understood by someone who knows nothing of how the game is built.
func stationCopyFor(
	reading services.StationReading, clock services.GameClock, report services.StationReport,
) stationCopy {
	verdict := reading.Verdict(clock, report.Playing)
	toCome, here, later := len(reading.ToCome), len(reading.Here), len(reading.Later)
	switch verdict {
	case services.VerdictStay:
		text := "None are with you yet."
		if here > 0 {
			text = teamsAre(here) + " with you now."
		}
		if later > 0 {
			text += " " + teams(later) + " more could still reach you."
		}
		return stationCopy{
			Class: "stay", Icon: "hourglass", Pill: teams(toCome+here) + " to come",
			Tag: "Keep waiting", Title: teams(toCome+here) + " still to come", Text: text,
		}
	case services.VerdictHere:
		return stationCopy{
			Class: "here", Icon: "users", Pill: teams(here) + " with you",
			Tag: "Teams are with you", Title: teamsAre(here) + " here now", Text: "No one else is due after this.",
		}
	case services.VerdictLater:
		return stationCopy{
			Class: "stay", Icon: "hourglass", Pill: teams(later) + " later",
			Tag: "Keep waiting", Title: "Nobody is due yet",
			Text: teams(later) + " haven't reached you but their route does include you.",
		}
	case services.VerdictMaybe:
		return stationCopy{
			Class: "maybe", Icon: "clock", Pill: "Probably done",
			Tag: "Probably done", Title: "Everyone who can visit has been", Text: maybeText(clock, report),
		}
	case services.VerdictDone:
		return stationCopy{
			Class: "done", Icon: "circle-check", Pill: "All done",
			Tag: "You can go", Title: "You’re all done here",
			Text: "The game has ended and no one else can visit. Thanks for helping!",
		}
	case services.VerdictWait:
	}
	return waitCopy(clock)
}

func maybeText(clock services.GameClock, report services.StationReport) string {
	text := "No team can visit you right now."
	if report.NotStarted > 0 {
		text += " " + teamsHave(report.NotStarted) + " not started yet and could still arrive"
		if clock.HasEnd() {
			text += " before the game closes"
		}
		text += "."
	}
	return text
}

func waitCopy(clock services.GameClock) stationCopy {
	words := stationCopy{Class: "wait", Icon: "moon", Tag: "Waiting"}
	switch clock.Kind {
	case services.ClockSoon:
		words.Pill, words.Title, words.Text = "Not open yet", "Not open yet", "This page updates itself when the game opens."
	case services.ClockClosed:
		words.Pill, words.Title, words.Text = "Not open", "The game isn’t open", "Nobody can play until it is opened."
	case services.ClockOpen, services.ClockOver:
		words.Pill, words.Title, words.Text = "No teams yet", "No teams yet",
			"Nobody has started playing. Teams will show up here as they begin."
	}
	return words
}

// bannerIcon is the status icon the activity page uses for the same states.
func bannerIcon(kind services.ClockKind) string {
	status := models.Closed
	switch kind {
	case services.ClockOpen:
		status = models.Active
	case services.ClockSoon:
		status = models.Scheduled
	case services.ClockOver, services.ClockClosed:
	}
	name, _ := statusIconLook(status)
	return name
}

// countdownFor is what the banner counts down to, as the activity page does:
// the opening of a game not yet open, the closing of one that has an end.
func countdownFor(clock services.GameClock) (time.Time, string) {
	switch clock.Kind {
	case services.ClockSoon:
		return clock.Opens, "Opens in"
	case services.ClockOpen:
		if clock.HasEnd() {
			return clock.Closes, "Closes in"
		}
	case services.ClockOver, services.ClockClosed:
	}
	return time.Time{}, ""
}

// teamsPlaying is "1 team playing", "21 teams playing".
func teamsPlaying(n int) string {
	return teams(n) + " playing"
}

// The classes summaryLine returns, for the sentence's colour.
const (
	summaryNeed = "need"
	summaryOK   = "ok"
)

// summaryLine is the one sentence for everything this facilitator is watching.
func summaryLine(data FacilitatorDashboardData) (string, string) {
	watched := data.Watched()
	total := len(watched)
	if data.Clock.Kind == services.ClockSoon || data.Clock.Kind == services.ClockClosed {
		return "Waiting for the game to open.", ""
	}
	need := 0
	for _, station := range watched {
		if station.Verdict(data.Clock, data.Report.Playing).Waiting() {
			need++
		}
	}
	switch {
	case need > 0 && total == 1:
		return "Teams are still due at your station.", summaryNeed
	case need == 1:
		return fmt.Sprintf("1 of your %d stations still has teams coming.", total), summaryNeed
	case need > 1:
		return fmt.Sprintf("%d of your %d stations still have teams coming.", need, total), summaryNeed
	case data.Clock.Kind == services.ClockOver && total == 1:
		return "Nothing more is due. You can go.", summaryOK
	case data.Clock.Kind == services.ClockOver:
		return "Nothing more is due at any of your stations. You can go.", summaryOK
	case total == 1:
		return "Nobody is due at your station right now.", summaryOK
	}
	return "Nobody is due at any of your stations right now.", summaryOK
}
