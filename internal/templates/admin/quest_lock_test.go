package templates

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func renderLocked(t *testing.T, quest models.Quest) string {
	t.Helper()
	var out strings.Builder
	require.NoError(t, LockedEditor(quest, templ.Raw("")).Render(context.Background(), &out))
	return out.String()
}

// running is a quest in progress, which is the only state the lock engages in.
func running() models.Quest {
	return models.Quest{
		ID:        "quest-1",
		StartTime: bun.NullTime{Time: time.Now().Add(-time.Hour)},
		EndTime:   bun.NullTime{Time: time.Now().Add(time.Hour)},
	}
}

// Unlocking one quest must not unlock another.
func TestQuestLock_UnlockIsScopedToTheQuest(t *testing.T) {
	html := renderLocked(t, running())

	assert.Contains(t, html, `data-quest-id="quest-1"`)
	assert.Contains(t, html, "'rapua.unlock.' + id")
}

// Session storage, not local: an unlock must not survive to a later day.
func TestQuestLock_UnlockDoesNotOutliveTheSession(t *testing.T) {
	html := renderLocked(t, running())

	assert.Contains(t, html, "sessionStorage.setItem")
	assert.Contains(t, html, "sessionStorage.getItem")
	// The word also appears in a comment, so assert on the calls.
	assert.NotContains(t, html, "localStorage.setItem")
	assert.NotContains(t, html, "localStorage.getItem")
}

// The id reaches the script as data, never spliced into source.
func TestQuestLock_QuestIDIsDataNotSource(t *testing.T) {
	hostile := running()
	hostile.ID = `"); alert(1); //`
	html := renderLocked(t, hostile)

	// Inert in the attribute; it must never reach a script body.
	for _, script := range scriptBodies(html) {
		assert.NotContains(t, script, "alert(1)")
	}
}

// scriptBodies is the contents of every script element on the page.
func scriptBodies(html string) []string {
	var bodies []string
	rest := html
	for {
		start := strings.Index(rest, "<script>")
		if start < 0 {
			return bodies
		}
		rest = rest[start+len("<script>"):]
		end := strings.Index(rest, "</script>")
		if end < 0 {
			return bodies
		}
		bodies = append(bodies, rest[:end])
		rest = rest[end:]
	}
}

// A restored unlock must show on the button, or the page looks locked while
// edits save.
func TestQuestLock_RestoredUnlockShowsOnTheButton(t *testing.T) {
	html := renderLocked(t, running())

	assert.Contains(t, html, "if (window.questLock.bypasses()) showUnlocked();")
	// Both states are rendered so the swap never builds an icon in script.
	assert.Contains(t, html, `data-lock-state="locked"`)
	assert.Contains(t, html, `data-lock-state="unlocked"`)
}

// The control lives in the bar, not the page header, so it survives navigation.
func TestQuestLock_ButtonGoesToTheNavSlot(t *testing.T) {
	html := renderLocked(t, running())

	assert.Contains(t, html, "getElementById('quest-unlock-slot')")
	assert.NotContains(t, html, "quest-title-group", "the page header is the page's own")
}

// A stopped quest drops its remembered unlock, so a restart asks again.
func TestQuestLock_StoppedQuestForgetsItsUnlock(t *testing.T) {
	html := renderLocked(t, models.Quest{ID: "quest-1"})

	assert.Contains(t, html, "window.questLock.set(")
	assert.Contains(t, html, "quest-lock-meta")
	assert.NotContains(t, html, "quest-unlock-confirm", "a stopped quest has nothing to unlock")
}

// The wording itself is tested in starts_at_test.go against a fixed now.
func TestQuestStatus_ScheduledSaysWhenItStarts(t *testing.T) {
	start := time.Now().Local().Add(3 * time.Hour)
	quest := models.Quest{ID: "q", StartTime: bun.NullTime{Time: start}}

	assert.Contains(t, startsAt(quest), "Starts ")
	assert.NotEqual(t, "Scheduled", startsAt(quest),
		"a quest with a start time says when, not that it has one")
}

func TestQuestStatus_NoStartTimeFallsBackToTheWord(t *testing.T) {
	assert.Equal(t, "Scheduled", startsAt(models.Quest{}))
}

// Rewriting one button's classes in script left it with neither state's styling.
func TestQuestLock_BothStatesAreRenderedOneHidden(t *testing.T) {
	html := renderLocked(t, running())

	assert.Contains(t, html, `data-lock-state="locked"`)
	assert.Contains(t, html, `data-lock-state="unlocked"`)
	assert.Contains(t, html, "btn btn-sm btn-warning btn-outline", "locked is a control")
	assert.Contains(t, html, "btn btn-sm btn-warning gap-1.5", "and so is unlocked")
	assert.NotContains(t, html, "button.className", "nothing is restyled in script")

	// Otherwise both words show before anything is pressed.
	at := strings.Index(html, `data-lock-state="unlocked"`)
	require.GreaterOrEqual(t, at, 0)
	open := strings.LastIndex(html[:at], "<")
	require.GreaterOrEqual(t, open, 0)
	tag := html[open:]
	end := strings.Index(tag, ">")
	require.GreaterOrEqual(t, end, 0)
	assert.Contains(t, tag[:end], "hidden")
}

// The warning colour does not say which way the switch goes.
func TestQuestLock_BothStatesSayWhatAClickDoes(t *testing.T) {
	html := renderLocked(t, running())

	assert.Contains(t, html, "Click to unlock.")
	assert.Contains(t, html, "Click to lock it again.")
	assert.Equal(t, 2, strings.Count(html, "tooltip tooltip-bottom"))
}

// Relocking must not need a page reload.
func TestQuestLock_CanBeLockedAgain(t *testing.T) {
	html := renderLocked(t, running())

	assert.Contains(t, html, `id="quest-relock"`)
	assert.Contains(t, html, "relock()")
	assert.Contains(t, html, "forget(state.questID)")
}

// renderNav renders the bar, where the status lives.
func renderNav(t *testing.T, quest models.Quest) string {
	t.Helper()
	var out strings.Builder
	user := models.User{CurrentQuestID: quest.ID, CurrentQuest: quest}
	require.NoError(t, nav(user, "Quest").Render(context.Background(), &out))
	return out.String()
}

// The dot is daisyUI's status component, not a hand-rolled div.
func TestQuestStatus_UsesTheStatusComponent(t *testing.T) {
	html := renderNav(t, running())

	assert.Contains(t, html, "status-success")
	assert.Contains(t, html, `class="status shrink-0 status-success"`)
	assert.NotContains(t, html, "rounded-full bg-success", "no hand-rolled dot")
}

// The control sits left of the status so the status does not shift.
func TestQuestStatus_ControlComesBeforeTheStatus(t *testing.T) {
	html := renderNav(t, running())

	// Scoped to the bar: the drawer's status card renders earlier.
	at := strings.Index(html, "navbar-end")
	require.GreaterOrEqual(t, at, 0)
	bar := html[at:]

	slot := strings.Index(bar, "quest-unlock-slot")
	status := strings.Index(bar, "status-success")
	require.GreaterOrEqual(t, slot, 0)
	require.GreaterOrEqual(t, status, 0)
	assert.Less(t, slot, status)
}
