package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// m20260822000000_Objective mirrors models.Objective; slug uniqueness is
// per-quest and only enforced on non-empty slugs, matching locations_instance_slug.
type m20260822000000_Objective struct {
	bun.BaseModel `bun:"table:objectives"`

	ID         string    `bun:"id,pk,notnull,type:varchar(36)"`
	CreatedAt  time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt  time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	QuestID    string    `bun:"quest_id,notnull,type:varchar(36)"`
	Slug       string    `bun:"slug,type:varchar(255)"`
	Title      string    `bun:"title,type:varchar(255)"`
	WhenClause string    `bun:"when_clause,type:text"`
	Order      int       `bun:"order,type:int"`
	ProofSets  string    `bun:"proof_sets,type:text"`
	RevealSets string    `bun:"reveal_sets,type:text"`
}

// m20260822000000_ObjectiveContextCompletion mirrors models.ObjectiveContextCompletion:
// an append-only completion log, never updated or deleted. The primary key is the
// idempotency guard for firing a context's sets exactly once (see models package doc).
type m20260822000000_ObjectiveContextCompletion struct {
	bun.BaseModel `bun:"table:objective_context_completions"`

	RunCode     string    `bun:"run_code,pk,notnull,type:varchar(36)"`
	ObjectiveID string    `bun:"objective_id,pk,notnull,type:varchar(36)"`
	Context     string    `bun:"context,pk,notnull,type:varchar(32)"`
	CompletedAt time.Time `bun:"completed_at,nullzero,notnull,default:current_timestamp"`
}

func init() {
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.NewCreateTable().
				Model(&m20260822000000_Objective{}).
				IfNotExists().
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("create objectives table: %w", err)
			}

			_, err = db.NewCreateIndex().
				Model((*m20260822000000_Objective)(nil)).
				Index("idx_objectives_quest_id").
				Column("quest_id").
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("create index idx_objectives_quest_id: %w", err)
			}

			// Partial unique index: only enforce uniqueness on non-empty slugs, same
			// as locations_instance_slug, so fixtures inserting slugless rows don't collide.
			_, err = db.ExecContext(
				ctx,
				`CREATE UNIQUE INDEX objectives_quest_slug ON objectives (quest_id, slug) WHERE slug != ''`,
			)
			if err != nil {
				return fmt.Errorf("create index objectives_quest_slug: %w", err)
			}

			_, err = db.NewCreateTable().
				Model(&m20260822000000_ObjectiveContextCompletion{}).
				IfNotExists().
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("create objective_context_completions table: %w", err)
			}

			_, err = db.NewCreateIndex().
				Model((*m20260822000000_ObjectiveContextCompletion)(nil)).
				Index("idx_objective_context_completions_run_code").
				Column("run_code").
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("create index idx_objective_context_completions_run_code: %w", err)
			}

			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.NewDropTable().
				Model(&m20260822000000_ObjectiveContextCompletion{}).
				IfExists().
				Exec(ctx)
			if err != nil {
				return fmt.Errorf("drop objective_context_completions table: %w", err)
			}
			_, err = db.NewDropTable().Model(&m20260822000000_Objective{}).IfExists().Exec(ctx)
			if err != nil {
				return fmt.Errorf("drop objectives table: %w", err)
			}
			return nil
		},
	)
}
