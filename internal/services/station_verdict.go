package services

import (
	"time"

	"github.com/nathanhollows/Rapua/v8/models"
)

// ClockKind is whether the game is open, from the point of view of someone
// deciding whether to keep waiting.
type ClockKind string

const (
	// ClockOpen can still have teams arrive. It may or may not have a closing time.
	ClockOpen ClockKind = "open"
	// ClockSoon is scheduled to open.
	ClockSoon ClockKind = "soon"
	// ClockOver has passed its closing time.
	ClockOver ClockKind = "over"
	// ClockClosed has never been opened, or no time was ever set.
	ClockClosed ClockKind = "closed"
)

// GameClock is the answer to "how long have I got left".
type GameClock struct {
	Kind   ClockKind
	Opens  time.Time
	Closes time.Time
	// Remaining is how long until it closes, when it has a closing time.
	Remaining time.Duration
	// Percent is how much of the open period has passed, when both ends are set.
	Percent int
}

// HasEnd is whether the game has a closing time to count down to.
func (c GameClock) HasEnd() bool { return !c.Closes.IsZero() }

// BuildGameClock reads a quest's schedule at a moment, given as a parameter so
// the reading can be checked at any hour.
func BuildGameClock(quest models.Quest, now time.Time) GameClock {
	start, end := quest.StartTime.Time, quest.EndTime.Time
	switch {
	case start.IsZero():
		return GameClock{Kind: ClockClosed}
	case start.After(now):
		return GameClock{Kind: ClockSoon, Opens: start, Closes: end}
	case !end.IsZero() && end.Before(now):
		return GameClock{Kind: ClockOver, Opens: start, Closes: end}
	}

	const wholeGame = 100
	clock := GameClock{Kind: ClockOpen, Opens: start, Closes: end}
	if !end.IsZero() {
		clock.Remaining = end.Sub(now)
		if total := end.Sub(start); total > 0 {
			clock.Percent = max(0, min(wholeGame, int(now.Sub(start)*wholeGame/total)))
		}
	}
	return clock
}

// StationVerdict is the plain answer for one station.
type StationVerdict string

const (
	// VerdictWait is nothing to read yet: not open, or no team has started.
	VerdictWait StationVerdict = "wait"
	// VerdictStay is teams still to come.
	VerdictStay StationVerdict = "stay"
	// VerdictLater is nobody due yet, but teams whose route includes it.
	VerdictLater StationVerdict = "later"
	// VerdictHere is teams with the facilitator and none behind them.
	VerdictHere StationVerdict = "here"
	// VerdictMaybe is nobody due, but the game is open and more teams could start.
	VerdictMaybe StationVerdict = "maybe"
	// VerdictDone is the game over: nobody else can visit.
	VerdictDone StationVerdict = "done"
)

// Verdict says whether a facilitator has to keep waiting. playing is how many
// teams are in the game at all, so a station with no visits is told apart from a
// game with no players.
func (r StationReading) Verdict(clock GameClock, playing int) StationVerdict {
	switch clock.Kind {
	case ClockSoon, ClockClosed:
		return VerdictWait
	case ClockOver:
		return VerdictDone
	case ClockOpen:
	}

	switch {
	case len(r.ToCome) > 0:
		return VerdictStay
	case len(r.Here) > 0 && len(r.Later) == 0:
		return VerdictHere
	case len(r.Here) > 0:
		return VerdictStay
	case len(r.Later) > 0:
		return VerdictLater
	case playing == 0 && len(r.Been) == 0:
		return VerdictWait
	}
	return VerdictMaybe
}

// Waiting is whether anyone is due: the station still needs its facilitator.
func (v StationVerdict) Waiting() bool {
	return v == VerdictStay || v == VerdictLater || v == VerdictHere
}
