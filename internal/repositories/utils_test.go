package repositories_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v7/internal/db"
	"github.com/nathanhollows/Rapua/v7/internal/migrations"
	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

// testParents holds IDs for a full FK-valid parent chain: user → instance → marker → location.
type testParents struct {
	UserID     string
	QuestID    string
	MarkerCode string
	LocationID string
}

// createTestParents inserts a user, instance, marker, and location, returning their IDs.
// Use this whenever a test needs valid FK references for child rows.
func createTestParents(t *testing.T, dbc *bun.DB) testParents {
	t.Helper()
	ctx := context.Background()

	user := &models.User{ID: gofakeit.UUID(), Name: gofakeit.Name(), Email: gofakeit.Email()}
	_, err := dbc.NewInsert().Model(user).Exec(ctx)
	if err != nil {
		t.Fatalf("createTestParents: insert user: %v", err)
	}

	inst := &models.Quest{ID: gofakeit.UUID(), UserID: user.ID, Name: "test-instance"}
	_, err = dbc.NewInsert().Model(inst).Exec(ctx)
	if err != nil {
		t.Fatalf("createTestParents: insert instance: %v", err)
	}

	markerCode := gofakeit.LetterN(5)
	marker := &models.Marker{Code: markerCode}
	_, err = dbc.NewInsert().Model(marker).Exec(ctx)
	if err != nil {
		t.Fatalf("createTestParents: insert marker: %v", err)
	}

	loc := &models.Location{ID: gofakeit.UUID(), QuestID: inst.ID, MarkerID: markerCode, Name: "test-location"}
	_, err = dbc.NewInsert().Model(loc).Exec(ctx)
	if err != nil {
		t.Fatalf("createTestParents: insert location: %v", err)
	}

	return testParents{
		UserID:     user.ID,
		QuestID:    inst.ID,
		MarkerCode: markerCode,
		LocationID: loc.ID,
	}
}

// createTestTeam inserts a run for the given questID and returns its code.
// Codes are upper-cased because GetByCode upper-cases before querying.
func createTestTeam(t *testing.T, dbc *bun.DB, questID string) string {
	t.Helper()
	code := strings.ToUpper(gofakeit.LetterN(6))
	team := &models.Run{
		ID:      gofakeit.UUID(),
		Code:    code,
		Name:    gofakeit.Name(),
		QuestID: questID,
	}
	_, err := dbc.NewInsert().Model(team).Exec(context.Background())
	if err != nil {
		t.Fatalf("createTestTeam: %v", err)
	}
	return code
}

// createTestBlock inserts a block with the given ownerID and returns its ID.
// owner_id has no FK constraint so any non-empty string is valid.
func createTestBlock(t *testing.T, dbc *bun.DB, ownerID string) string {
	t.Helper()
	block := &models.Block{
		ID:      gofakeit.UUID(),
		OwnerID: ownerID,
		Type:    "text",
		Context: "location_content",
	}
	_, err := dbc.NewInsert().Model(block).Exec(context.Background())
	if err != nil {
		t.Fatalf("createTestBlock: %v", err)
	}
	return block.ID
}

// insertUserWithID inserts a user with a specific ID, ignoring conflicts (shared-cache DB).
func insertUserWithID(t *testing.T, dbc *bun.DB, userID string) {
	t.Helper()
	user := &models.User{ID: userID, Name: gofakeit.Name(), Email: gofakeit.Email()}
	_, err := dbc.NewInsert().Model(user).
		On("CONFLICT (\"id\") DO NOTHING").
		Exec(context.Background())
	if err != nil {
		t.Fatalf("insertUserWithID: %v", err)
	}
}

func newTLogger(t *testing.T) *slog.Logger {
	handler := slog.NewTextHandler(testWriter{t}, nil)
	return slog.New(handler)
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Logf("%s", p)
	return len(p), nil
}

func setupDB(t *testing.T) (*bun.DB, func()) {
	t.Helper()
	t.Setenv("DB_CONNECTION", "file::memory:?cache=shared")
	t.Setenv("DB_TYPE", "sqlite3")
	db := db.MustOpen(newTLogger(t))
	ctx := context.Background()

	migrator := migrate.NewMigrator(db, migrations.Migrations)
	if err := migrator.Init(ctx); err != nil {
		t.Fatal(err)
	}

	if err := migrator.Lock(ctx); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := migrator.Unlock(ctx); err != nil {
			t.Fatal(err)
		}
	}()

	_, err := migrator.Migrate(ctx)
	if err != nil {
		t.Fatal(err)
	}

	return db, func() {
		db.Close()
	}
}
