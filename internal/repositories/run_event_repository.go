package repositories

import (
	"context"
	"time"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/uptrace/bun"
)

// RunEventRepository appends to the run event stream and reads it back.
type RunEventRepository interface {
	// Record appends one event, filling in a zero CreatedAt.
	Record(ctx context.Context, event *models.RunEvent) error
	// RecordWithTx appends inside a caller's transaction, so the event and the
	// state change it describes commit or roll back together.
	RecordWithTx(ctx context.Context, tx *bun.Tx, event *models.RunEvent) error

	// ListByRun returns one run's events oldest first.
	ListByRun(ctx context.Context, runCode string) ([]models.RunEvent, error)
	// ListByQuestSince returns a quest's events at or after a time, oldest first.
	// A zero since means the whole history.
	ListByQuestSince(ctx context.Context, questID string, since time.Time) ([]models.RunEvent, error)

	// LatestByType returns each run's most recent event of the given types, keyed
	// by run code. Runs with none are absent from the map, not zero: never and not
	// recently would otherwise be indistinguishable.
	LatestByType(
		ctx context.Context, questID string, types []models.EventType,
	) (map[string]time.Time, error)
	// TimesByType returns those events' times per run, oldest first. Since bounds
	// every gap, so a caller wanting how a team is moving now passes a recent time
	// rather than all history.
	TimesByType(
		ctx context.Context, questID string, types []models.EventType, since time.Time,
	) (map[string][]time.Time, error)

	// RunCompleted reports whether the run has recorded its completion, for
	// callers with no transaction and no race to lose.
	RunCompleted(ctx context.Context, runCode string) (bool, error)
	// RunCompletedWithTx reports whether the run has already recorded its
	// completion. Read inside the caller's transaction, where the write lock
	// makes the answer decisive against a second completion racing the first.
	RunCompletedWithTx(ctx context.Context, tx *bun.Tx, runCode string) (bool, error)
}

type runEventRepository struct {
	db *bun.DB
}

// NewRunEventRepository creates a RunEventRepository.
func NewRunEventRepository(db *bun.DB) RunEventRepository {
	return &runEventRepository{db: db}
}

// stamp fills in the timestamp. Written here rather than left to the column
// default because that default is second-resolution and this table's whole purpose
// is the gaps between rows. Explicit UTC, because some existing tables mix the two.
func stamp(event *models.RunEvent) {
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
}

func (r *runEventRepository) Record(ctx context.Context, event *models.RunEvent) error {
	stamp(event)
	_, err := r.db.NewInsert().Model(event).Exec(ctx)
	return err
}

func (r *runEventRepository) RecordWithTx(ctx context.Context, tx *bun.Tx, event *models.RunEvent) error {
	stamp(event)
	_, err := tx.NewInsert().Model(event).Exec(ctx)
	return err
}

func (r *runEventRepository) RunCompleted(ctx context.Context, runCode string) (bool, error) {
	return r.db.NewSelect().
		Model((*models.RunEvent)(nil)).
		Where("run_code = ?", runCode).
		Where("type = ?", models.EventRunCompleted).
		Exists(ctx)
}

func (r *runEventRepository) RunCompletedWithTx(
	ctx context.Context, tx *bun.Tx, runCode string,
) (bool, error) {
	return tx.NewSelect().
		Model((*models.RunEvent)(nil)).
		Where("run_code = ?", runCode).
		Where("type = ?", models.EventRunCompleted).
		Exists(ctx)
}

func (r *runEventRepository) ListByRun(ctx context.Context, runCode string) ([]models.RunEvent, error) {
	var events []models.RunEvent
	err := r.db.NewSelect().
		Model(&events).
		Where("run_code = ?", runCode).
		Order("created_at", "id").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (r *runEventRepository) ListByQuestSince(
	ctx context.Context, questID string, since time.Time,
) ([]models.RunEvent, error) {
	var events []models.RunEvent
	query := r.db.NewSelect().
		Model(&events).
		Where("quest_id = ?", questID)
	if !since.IsZero() {
		query = query.Where("created_at >= ?", since)
	}
	err := query.Order("created_at", "id").Scan(ctx)
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (r *runEventRepository) LatestByType(
	ctx context.Context, questID string, types []models.EventType,
) (map[string]time.Time, error) {
	latest := make(map[string]time.Time)
	if len(types) == 0 {
		return latest, nil
	}

	var rows []struct {
		RunCode string    `bun:"run_code"`
		LastAt  time.Time `bun:"last_at"`
	}
	err := r.db.NewSelect().
		Model((*models.RunEvent)(nil)).
		ColumnExpr("run_code").
		ColumnExpr("MAX(created_at) AS last_at").
		Where("quest_id = ?", questID).
		Where("type IN (?)", bun.In(types)).
		Group("run_code").
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		latest[row.RunCode] = row.LastAt
	}
	return latest, nil
}

func (r *runEventRepository) TimesByType(
	ctx context.Context, questID string, types []models.EventType, since time.Time,
) (map[string][]time.Time, error) {
	times := make(map[string][]time.Time)
	if len(types) == 0 {
		return times, nil
	}

	var events []models.RunEvent
	query := r.db.NewSelect().
		Model(&events).
		Column("run_code", "created_at").
		Where("quest_id = ?", questID).
		Where("type IN (?)", bun.In(types))
	if !since.IsZero() {
		query = query.Where("created_at >= ?", since)
	}
	// Ordered by the id as well, so two events in the same microsecond keep the
	// order they were written in rather than an arbitrary one.
	err := query.Order("run_code", "created_at", "id").Scan(ctx)
	if err != nil {
		return nil, err
	}

	for _, event := range events {
		times[event.RunCode] = append(times[event.RunCode], event.CreatedAt)
	}
	return times, nil
}
