package admin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func station(id string) services.StationReading {
	return services.StationReading{Objective: models.Objective{ID: id}}
}

func watchRequest(value string, set bool) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/facilitator/dashboard", nil)
	if set {
		req.AddCookie(&http.Cookie{Name: facilitatorWatchCookie, Value: value})
	}
	return req
}

// A link made with no objectives covers everything; one made with some covers
// only those, in the quest's order.
func TestStationsForToken(t *testing.T) {
	all := []services.StationReading{station("a"), station("b"), station("c")}

	assert.Len(t, stationsForToken(all, &models.FacilitatorToken{}), 3)

	scoped := stationsForToken(all, &models.FacilitatorToken{Objectives: []string{"c", "a", "gone"}})
	assert.Equal(t, "a", scoped[0].Objective.ID, "the quest's order, not the link's")
	assert.Equal(t, "c", scoped[1].Objective.ID)
	assert.Len(t, scoped, 2, "an objective no longer in play is dropped")
}

// What is followed is the cookie, kept to what the link covers, so a hand-made
// cookie cannot reach another station.
func TestWatchedStations(t *testing.T) {
	covered := []services.StationReading{station("a"), station("b")}
	open := &models.FacilitatorToken{}
	scoped := &models.FacilitatorToken{Objectives: []string{"a", "b"}}

	t.Run("a link for the whole game follows nothing until asked", func(t *testing.T) {
		assert.Empty(t, watchedStations(watchRequest("", false), covered, open))
	})
	t.Run("a link made for particular stations follows them all", func(t *testing.T) {
		got := watchedStations(watchRequest("", false), covered, scoped)
		assert.Equal(t, map[string]bool{"a": true, "b": true}, got)
	})
	t.Run("a choice is the choice", func(t *testing.T) {
		assert.Equal(t, map[string]bool{"b": true},
			watchedStations(watchRequest(services.StationKey("b"), true), covered, scoped))
	})
	t.Run("choosing nothing stays nothing, even on a scoped link", func(t *testing.T) {
		assert.Empty(t, watchedStations(watchRequest("", true), covered, scoped))
	})
	t.Run("the browser's percent-encoded commas read the same", func(t *testing.T) {
		encoded := services.StationKey("a") + "%2C" + services.StationKey("b")
		got := watchedStations(watchRequest(encoded, true), covered, open)
		assert.Equal(t, map[string]bool{"a": true, "b": true}, got)
	})
	t.Run("a station the link does not cover is ignored", func(t *testing.T) {
		cookie := services.StationKey("a") + "," + services.StationKey("other")
		got := watchedStations(watchRequest(cookie, true), covered, open)
		assert.Equal(t, map[string]bool{"a": true}, got)
	})
}

// facilitatorPage is a quest with two stations and one started team, a handler
// able to serve its facilitator page, and the cookies a facilitator would send.
type facilitatorPage struct {
	h          *Handler
	questID    string
	mail, bbq  models.Objective
	teamCode   string
	teamName   string
	linkScoped string
	linkOpen   string
}

func setupFacilitatorPage(t *testing.T) facilitatorPage {
	t.Helper()
	dbc, cleanup := setupObjectiveTestDB(t)
	t.Cleanup(cleanup)
	ctx := context.Background()

	h := newObjectiveTestHandler(t, dbc)
	transactor := db.NewTransactor(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	objectiveRepo := repositories.NewObjectiveRepository(dbc)
	h.facilitatorService = services.NewFacilitatorService(repositories.NewFacilitatorTokenRepo(dbc))
	h.runService = services.NewRunService(
		transactor, repositories.NewRunRepository(dbc), nil, blockStateRepo, objectiveRepo,
		repositories.NewObjectiveContextCompletionRepository(dbc), repositories.NewSectionFinishRepository(dbc),
		repositories.NewBlockRepository(dbc, blockStateRepo), repositories.NewRunEventRepository(dbc),
		repositories.NewRunVariableRepository(dbc),
	)

	user := objectiveTestQuest(t, dbc)
	root, err := objectiveRepo.FindRoot(ctx, user.CurrentQuestID)
	require.NoError(t, err)
	station := func(slug, title string, position int) models.Objective {
		objective := models.Objective{
			ID: gofakeit.UUID(), QuestID: user.CurrentQuestID, ParentID: root.ID, Slug: slug, Title: title,
			Routing: models.RouteStrategyFreeRoam, Position: position,
		}
		_, insertErr := dbc.NewInsert().Model(&objective).Exec(ctx)
		require.NoError(t, insertErr)
		return objective
	}
	mail, bbq := station("mail", "Mailboxes", 0), station("bbq", "BBQ Area", 1)

	// Started a minute ago, so it is playing and can visit both.
	team := &models.Run{
		ID: gofakeit.UUID(), Code: "ZZ99", Name: "Red Foxes", QuestID: user.CurrentQuestID,
		HasStarted: true, StartedAt: time.Now().Add(-time.Minute),
	}
	_, err = dbc.NewInsert().Model(team).Exec(ctx)
	require.NoError(t, err)

	scoped, err := h.facilitatorService.CreateFacilitatorToken(ctx, user.CurrentQuestID, []string{mail.ID}, time.Hour)
	require.NoError(t, err)
	open, err := h.facilitatorService.CreateFacilitatorToken(ctx, user.CurrentQuestID, nil, time.Hour)
	require.NoError(t, err)

	return facilitatorPage{
		h: h, questID: user.CurrentQuestID, mail: mail, bbq: bbq,
		teamCode: team.Code, teamName: team.Name, linkScoped: scoped, linkOpen: open,
	}
}

func (f facilitatorPage) get(token string, watching ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/facilitator/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: facilitatorSessionCookie, Value: token})
	if len(watching) > 0 {
		keys := make([]string, 0, len(watching))
		for _, id := range watching {
			keys = append(keys, services.StationKey(id))
		}
		req.AddCookie(&http.Cookie{Name: facilitatorWatchCookie, Value: strings.Join(keys, ",")})
	}
	w := httptest.NewRecorder()
	f.h.FacilitatorDashboard(w, req)
	return w
}

// The security-relevant path: a link made for one station shows only that one,
// whatever the cookie says. A cookie is the facilitator's to write, so a station
// the link does not cover must not appear in the page or in its picker.
func TestFacilitatorDashboard_ALinkForOneStationShowsOnlyThatStation(t *testing.T) {
	f := setupFacilitatorPage(t)

	w := f.get(f.linkScoped, f.mail.ID, f.bbq.ID)

	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Mailboxes")
	assert.NotContains(t, body, "BBQ Area", "a station the link does not cover is nowhere on the page")
	assert.NotContains(t, body, f.bbq.ID, "nor in the picker")
	assert.NotContains(t, body, services.StationKey(f.bbq.ID), "nor its key")
	assert.Contains(t, body, f.teamName, "a team is shown by its name")
}

// A run's code gives full access to that team, so the page must never show one.
func TestFacilitatorDashboard_NeverShowsATeamsCode(t *testing.T) {
	f := setupFacilitatorPage(t)

	body := f.get(f.linkOpen, f.mail.ID, f.bbq.ID).Body.String()

	assert.Contains(t, body, f.teamName)
	assert.NotContains(t, body, f.teamCode)
}

func TestFacilitatorDashboard_ALinkForTheWholeGameFollowsWhatIsAsked(t *testing.T) {
	f := setupFacilitatorPage(t)

	both := f.get(f.linkOpen, f.mail.ID, f.bbq.ID).Body.String()
	assert.Contains(t, both, `data-station="`+f.mail.ID+`"`)
	assert.Contains(t, both, `data-station="`+f.bbq.ID+`"`)

	none := f.get(f.linkOpen).Body.String()
	assert.Contains(t, none, "Which stations are you looking after?", "nothing is followed until it is asked")
	assert.NotContains(t, none, "data-station=")
}

func TestFacilitatorDashboard_RefusesAnInvalidLink(t *testing.T) {
	f := setupFacilitatorPage(t)

	assert.Equal(t, http.StatusUnauthorized, f.get("not-a-token").Code)
}

// The cookie speaks in short keys. A whole objective ID, which the cookie once
// held, selects nothing: only the key of a covered station does.
func TestWatchedStations_AWholeIDIsNotAKey(t *testing.T) {
	covered := []services.StationReading{station("a")}

	got := watchedStations(watchRequest("a", true), covered, &models.FacilitatorToken{})

	assert.Empty(t, got)
}

// Eight characters a station, so a long choice still fits a cookie that a
// browser would otherwise drop without a word.
func TestStationKey_IsShortStableAndDistinct(t *testing.T) {
	id := "4f0b3c8e-2d1a-4c55-9a77-0d1e5f6a7b8c"

	assert.Len(t, services.StationKey(id), 8)
	again := services.StationKey(id)
	assert.Equal(t, services.StationKey(id), again, "the same station is always the same key")
	assert.NotEqual(t, services.StationKey(id), services.StationKey(id+"x"))

	seen := map[string]bool{}
	for i := range 5000 {
		key := services.StationKey(fmt.Sprintf("objective-%d", i))
		assert.False(t, seen[key], "no two of five thousand stations share a key")
		seen[key] = true
	}
	// 400 stations, comma separated, stays well inside a 4KB cookie.
	assert.Less(t, 400*(len(services.StationKey(id))+1), 3800)
}
