package templates

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func at(spec string) time.Time {
	parsed, err := time.ParseInLocation("2006-01-02 15:04", spec, time.Local)
	if err != nil {
		panic(err)
	}
	return parsed
}

// A weekday is only unambiguous inside a week, so wider gaps get a date.
func TestStartsAtFrom_SaysOnlyAsMuchAsItMust(t *testing.T) {
	now := at("2026-10-09 14:00")

	for _, c := range []struct{ name, start, want string }{
		{"later today", "2026-10-09 21:43", "Starts 9:43pm"},
		{"on the hour drops the minutes", "2026-10-09 21:00", "Starts 9pm"},
		{"tomorrow is named, not dated", "2026-10-10 09:00", "Starts tomorrow, 9am"},
		{"inside the week, a weekday is enough", "2026-10-13 21:43", "Starts Tue, 9:43pm"},
		{"a week out, the weekday repeats, so date it", "2026-10-16 21:43", "Starts 16 Oct"},
		{"next month drops the clock: the day is the question", "2026-11-14 21:43", "Starts 14 Nov"},
		{"another year carries the year", "2027-11-14 21:43", "Starts 14 Nov 2027"},
	} {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, startsAtFrom(at(c.start), now))
		})
	}
}

// "Mon 9pm" for last Monday would read as the coming one.
func TestStartsAtFrom_NeverNamesAWeekdayInThePast(t *testing.T) {
	now := at("2026-10-09 14:00")

	assert.Equal(t, "Starts 6 Oct", startsAtFrom(at("2026-10-06 21:00"), now))
}

// The boundary is the calendar day, not 24 hours from now.
func TestStartsAtFrom_CountsCalendarDays(t *testing.T) {
	now := at("2026-10-09 23:30")

	assert.Equal(t, "Starts tomorrow, 12:15am", startsAtFrom(at("2026-10-10 00:15"), now))
}

// The browser gets the instant because the server only knows its own timezone.
func TestQuestStatus_ScheduledCarriesTheInstant(t *testing.T) {
	// time.Local, because the fallback text is written in the server's zone.
	start := time.Date(2026, 11, 14, 21, 43, 0, 0, time.Local)
	quest := models.Quest{StartTime: bun.NullTime{Time: start}}

	var out strings.Builder
	require.NoError(t, questStatusIndicator(quest, false).Render(context.Background(), &out))

	html := out.String()
	assert.Contains(t, html, `datetime="`+start.UTC().Format(time.RFC3339)+`"`,
		"the moment, not the wording")
	assert.Contains(t, html, "data-starts-at")
	assert.Contains(t, html, "Starts 14 Nov", "and the server's words until a script runs")
}

// An empty datetime would be read as 1970.
func TestQuestStatus_NoStartTimeCarriesNoInstant(t *testing.T) {
	var out strings.Builder
	require.NoError(t, questStatusIndicator(models.Quest{}, false).Render(context.Background(), &out))

	assert.NotContains(t, out.String(), "data-starts-at")
}

// A day is not always twenty-four hours. Across a daylight-saving change the
// difference between two midnights is 23 or 25, and truncating that lost a day
// at exactly the time of year a quest is most likely to be scheduled across.
func TestDaysApart_SurvivesDaylightSaving(t *testing.T) {
	// New Zealand puts the clocks forward on 27 September 2026.
	nz, err := time.LoadLocation("Pacific/Auckland")
	require.NoError(t, err)

	from := time.Date(2026, 9, 24, 9, 0, 0, 0, nz)
	to := time.Date(2026, 10, 1, 9, 0, 0, 0, nz)

	assert.Equal(t, 7, daysApart(from, to), "seven midnights apart, 167 hours of them")
}

// And the same going back, where the week is an hour long.
func TestDaysApart_SurvivesTheClocksGoingBack(t *testing.T) {
	nz, err := time.LoadLocation("Pacific/Auckland")
	require.NoError(t, err)

	from := time.Date(2026, 4, 2, 9, 0, 0, 0, nz)
	to := time.Date(2026, 4, 9, 9, 0, 0, 0, nz)

	assert.Equal(t, 7, daysApart(from, to))
}
