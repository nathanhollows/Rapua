package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(m20260906000000_up, m20260906000000_down)
}

// m20260906000000_up gives every objective a routing strategy.
//
// An empty value was accepted by the editor and read as free roam by the
// runtime, so it played as a strategy while reading as an unmade decision.
// Ordered is what the empty rows become: the three strategies produce quests
// that play nothing like one another, and ordered is the only one that cannot
// offer a player more than the author sequenced.
//
// Rows written this way were free roam in practice, so a quest that relied on
// the default now presents its objectives in the order they were authored.
// That is the point: the choice is visible and the author can change it.
//
// Every quest is converted, running ones included. A game left open with no end
// time reads as running forever, so gating on that would refuse the migration
// on any database holding an old quest nobody closed, which is most of them.
//
// Rows carrying the retired "secret" strategy are converted too. It meant
// keeping a group out of the listings, nothing does that now, and the engine
// was already playing those rows as free roam while the linter called them
// invalid.
func m20260906000000_up(ctx context.Context, db *bun.DB) error {
	if !columnExists(ctx, db, "objectives", "routing") {
		return nil
	}

	// Anything that is not one of the three strategies, rather than a list of
	// the bad values seen so far: an empty string, a NULL, the retired
	// "secret", and whatever else a hand-edited database holds. After this,
	// every row carries a routing the engine and the linter both recognise.
	if _, err := db.ExecContext(
		ctx,
		`UPDATE "objectives" SET "routing" = 'ordered'`+
			` WHERE "routing" IS NULL OR "routing" NOT IN ('ordered', 'free_roam', 'randomised')`,
	); err != nil {
		return fmt.Errorf("setting routing on objectives without a valid one: %w", err)
	}

	return nil
}

// m20260906000000_down does nothing: the rows it would restore were empty
// because nobody chose, and there is no record of which ones those were.
func m20260906000000_down(_ context.Context, _ *bun.DB) error {
	return nil
}
