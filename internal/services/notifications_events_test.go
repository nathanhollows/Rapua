package services_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The words ride the event as well as the row. The notifications table filters
// dismissed messages out of every read, so the log is the only copy that outlives
// a dismissal, and the only one reporting and the journal can read from.
func TestNotificationService_SentEventCarriesTheMessage(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	user := &models.User{ID: gofakeit.UUID(), Name: gofakeit.Name(), Email: gofakeit.Email()}
	_, err := dbc.NewInsert().Model(user).Exec(ctx)
	require.NoError(t, err)
	quest := &models.Quest{ID: gofakeit.UUID(), UserID: user.ID, Name: "Test Quest"}
	_, err = dbc.NewInsert().Model(quest).Exec(ctx)
	require.NoError(t, err)
	run := &models.Run{ID: gofakeit.UUID(), Code: "AAAAA", QuestID: quest.ID, HasStarted: true}
	_, err = dbc.NewInsert().Model(run).Exec(ctx)
	require.NoError(t, err)

	svc := services.NewNotificationService(
		db.NewTransactor(dbc),
		repositories.NewNotificationRepository(dbc),
		repositories.NewRunRepository(dbc),
		repositories.NewRunEventRepository(dbc),
	)

	sent, err := svc.SendNotification(ctx, run.Code, "Head to the quad, we're regrouping")
	require.NoError(t, err)

	stored, err := repositories.NewRunEventRepository(dbc).ListByRun(ctx, run.Code)
	require.NoError(t, err)
	require.Len(t, stored, 1)

	var payload models.NotificationData
	require.NoError(t, json.Unmarshal(stored[0].Data, &payload))
	assert.Equal(t, sent.ID, payload.NotificationID)
	assert.Equal(t, "Head to the quad, we're regrouping", payload.Content)
}
