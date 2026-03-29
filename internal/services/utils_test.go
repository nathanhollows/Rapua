package services_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v7/internal/db"
	"github.com/nathanhollows/Rapua/v7/internal/migrations"
	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

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

// testParents holds IDs for a full FK-valid parent chain: user → instance → marker → location.
type testParents struct {
	UserID     string
	InstanceID string
	MarkerCode string
	LocationID string
}

// createTestParents inserts a user, instance, marker, and location, returning their IDs.
func createTestParents(t *testing.T, dbc *bun.DB) testParents {
	t.Helper()
	ctx := context.Background()

	user := &models.User{ID: gofakeit.UUID(), Name: gofakeit.Name(), Email: gofakeit.Email()}
	_, err := dbc.NewInsert().Model(user).Exec(ctx)
	if err != nil {
		t.Fatalf("createTestParents: insert user: %v", err)
	}

	inst := &models.Instance{ID: gofakeit.UUID(), UserID: user.ID, Name: "test-instance"}
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

	loc := &models.Location{ID: gofakeit.UUID(), InstanceID: inst.ID, MarkerID: markerCode, Name: "test-location"}
	_, err = dbc.NewInsert().Model(loc).Exec(ctx)
	if err != nil {
		t.Fatalf("createTestParents: insert location: %v", err)
	}

	return testParents{
		UserID:     user.ID,
		InstanceID: inst.ID,
		MarkerCode: markerCode,
		LocationID: loc.ID,
	}
}

// insertTestUser inserts a user with a specific ID, ignoring conflicts.
func insertTestUser(t *testing.T, dbc *bun.DB, userID string) {
	t.Helper()
	user := &models.User{ID: userID, Name: gofakeit.Name(), Email: gofakeit.Email()}
	_, err := dbc.NewInsert().Model(user).
		On("CONFLICT (\"id\") DO NOTHING").
		Exec(context.Background())
	if err != nil {
		t.Fatalf("insertTestUser: %v", err)
	}
}

// insertTestInstance inserts an instance with FK-valid user, returning the instance ID.
func insertTestInstance(t *testing.T, dbc *bun.DB, userID string) string {
	t.Helper()
	inst := &models.Instance{ID: gofakeit.UUID(), UserID: userID, Name: "test-instance"}
	_, err := dbc.NewInsert().Model(inst).Exec(context.Background())
	if err != nil {
		t.Fatalf("insertTestInstance: %v", err)
	}
	return inst.ID
}

// insertTestMarker inserts a marker with the given code, ignoring conflicts.
func insertTestMarker(t *testing.T, dbc *bun.DB, code string) {
	t.Helper()
	marker := &models.Marker{Code: code}
	_, err := dbc.NewInsert().Model(marker).
		On("CONFLICT (\"code\") DO NOTHING").
		Exec(context.Background())
	if err != nil {
		t.Fatalf("insertTestMarker: %v", err)
	}
}

// insertTestLocation inserts a location with a new marker, returning the location ID.
func insertTestLocation(t *testing.T, dbc *bun.DB, instanceID string) string {
	t.Helper()
	ctx := context.Background()
	markerCode := gofakeit.LetterN(5)
	marker := &models.Marker{Code: markerCode}
	_, err := dbc.NewInsert().Model(marker).Exec(ctx)
	if err != nil {
		t.Fatalf("insertTestLocation: insert marker: %v", err)
	}
	loc := &models.Location{ID: gofakeit.UUID(), InstanceID: instanceID, MarkerID: markerCode, Name: "test-location"}
	_, err = dbc.NewInsert().Model(loc).Exec(ctx)
	if err != nil {
		t.Fatalf("insertTestLocation: insert location: %v", err)
	}
	return loc.ID
}

// insertTestTeam inserts a team with a specific code and instance, ignoring conflicts.
func insertTestTeam(t *testing.T, dbc *bun.DB, teamCode, instanceID string) {
	t.Helper()
	team := &models.Team{ID: gofakeit.UUID(), Code: teamCode, Name: "test-team", InstanceID: instanceID}
	_, err := dbc.NewInsert().Model(team).
		On("CONFLICT (\"code\") DO NOTHING").
		Exec(context.Background())
	if err != nil {
		t.Fatalf("insertTestTeam: %v", err)
	}
}
