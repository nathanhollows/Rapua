package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
	"github.com/urfave/cli/v2"
)

const (
	seedEmailDomain = "@seed.rapua.test"
	seedEmail       = "designer" + seedEmailDomain
	seedLinkHours   = 12
)

// Nothing in the app routes to this: it is a command and only a command.
func newSeedCommand(dbc *bun.DB, migrator *migrate.Migrator, logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "seed",
		Usage: "fill a development database with a designer and a game holding every block",
		Description: "Point DB_CONNECTION at a scratch SQLite file first. " +
			"It refuses production, and any database that holds other people's accounts.",
		Action: func(c *cli.Context) error {
			if err := seedAllowed(c.Context, dbc); err != nil {
				return err
			}
			if err := seedMigrate(c.Context, dbc, migrator); err != nil {
				return err
			}
			return seedRun(c.Context, dbc, logger)
		},
	}
}

// A seed run on a live database would add an account beside real ones.
func seedAllowed(ctx context.Context, dbc *bun.DB) error {
	if os.Getenv("IS_PROD") == "1" {
		return errors.New("refusing to seed: IS_PROD is 1")
	}
	if os.Getenv("DB_TYPE") != "sqlite3" {
		return errors.New("refusing to seed: DB_TYPE must be sqlite3")
	}
	if os.Getenv("SESSION_KEY") == "" {
		return errors.New("SESSION_KEY is not set, so there is no way to sign a login link")
	}

	exists, err := dbc.NewSelect().
		TableExpr("sqlite_master").
		Where("name = 'users'").
		Exists(ctx)
	if err != nil {
		return fmt.Errorf("checking for a users table: %w", err)
	}
	if !exists {
		return nil // a database nothing has been written to
	}
	others, err := dbc.NewSelect().
		Model((*models.User)(nil)).
		Where("email NOT LIKE ?", "%"+seedEmailDomain).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("checking for existing accounts: %w", err)
	}
	if others > 0 {
		return fmt.Errorf("refusing to seed: %d account(s) here are not seed accounts", others)
	}
	return nil
}

func seedMigrate(ctx context.Context, dbc *bun.DB, migrator *migrate.Migrator) error {
	defer pinToOneConn(dbc)()
	if err := migrator.Init(ctx); err != nil {
		return err
	}
	if err := migrator.Lock(ctx); err != nil {
		return err
	}
	defer func() { _ = migrator.Unlock(ctx) }()
	_, err := migrator.Migrate(ctx)
	return err
}

func seedRun(ctx context.Context, dbc *bun.DB, logger *slog.Logger) error {
	userRepo := repositories.NewUserRepository(dbc)

	user, err := userRepo.GetByEmail(ctx, seedEmail)
	if err != nil || user == nil {
		user = &models.User{
			ID:            uuid.NewString(),
			Name:          "Seed Designer",
			Email:         seedEmail,
			EmailVerified: true,
			Provider:      models.ProviderEmail,
			FreeCredits:   100,
			PaidCredits:   100,
		}
		if err := userRepo.Create(ctx, user); err != nil {
			return fmt.Errorf("creating the designer: %w", err)
		}
	}

	doc, err := seedGame()
	if err != nil {
		return err
	}

	transactor := db.NewTransactor(dbc)
	importer := services.NewImportService(
		logger,
		transactor,
		repositories.NewQuestRepository(dbc),
		repositories.NewQuestSettingsRepository(dbc),
		repositories.NewObjectiveRepository(dbc),
		repositories.NewBlockRepository(dbc, repositories.NewBlockStateRepository(dbc)),
	)
	result, err := importer.ImportCreate(ctx, user.ID, doc)
	if err != nil {
		return fmt.Errorf("importing the seed game: %w", err)
	}

	token, err := services.NewMagicTokenService([]byte(os.Getenv("SESSION_KEY"))).
		GenerateToken(user.ID, seedLinkHours*time.Hour)
	if err != nil {
		return err
	}
	siteURL := os.Getenv("SITE_URL")
	if siteURL == "" {
		siteURL = "http://localhost:8080"
	}
	fmt.Fprintf(os.Stdout, "\nSeeded game %s for %s.\nLogin link (valid %d hours):\n%s/login/magic/%s\n\n",
		result.QuestID, seedEmail, seedLinkHours, siteURL, token)
	return nil
}

// A proof block with no example here fails the command, so a new block cannot be missed.
func seedGame() (*game.GameDoc, error) {
	proof := seedProofBlocks()

	var missing []string
	for _, reg := range blocks.GetRegisteredBlocks() {
		if !slices.Contains(reg.SupportedContexts, game.ContextObjectiveProof) {
			continue
		}
		if !seedHasType(proof, reg.BlockType) {
			missing = append(missing, reg.BlockType)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return nil, fmt.Errorf("seed has no example for block type(s): %s", strings.Join(missing, ", "))
	}

	return &game.GameDoc{
		Rapua:    "v8",
		Name:     "Every block",
		Settings: game.SettingsDoc{EnablePoints: true, ShowLeaderboard: true},
		Structure: game.ObjectiveDoc{
			Slug:    "root",
			Routing: game.RouteStrategyOrdered,
			Title:   "Every block",
			Children: []game.ObjectiveDoc{
				{
					Slug:        "gallery",
					Title:       "Block gallery",
					Description: ptr("One of each block, to try the editor on."),
					Proof:       game.ObjectiveContextDoc{Blocks: proof},
					Reveal: game.ObjectiveContextDoc{Blocks: []game.BlockDoc{
						{"type": "text", "content": "Well done. That was every block."},
					}},
				},
			},
		},
	}, nil
}

func ptr[T any](v T) *T { return &v }

func seedHasType(docs []game.BlockDoc, blockType string) bool {
	return slices.ContainsFunc(docs, func(d game.BlockDoc) bool { return d["type"] == blockType })
}

//nolint:funlen // a list of examples, one per block
func seedProofBlocks() []game.BlockDoc {
	return []game.BlockDoc{
		{"type": "header", "icon": "map-pin", "title": "Block gallery", "size": "large"},
		{"type": "text", "content": "Some **Markdown** with a [link](https://example.com).\n\n- one\n- two"},
		{"type": "alert", "content": "Watch for traffic on the way.", "style": "warning"},
		{"type": "button", "link": "https://example.com", "text": "Open the map", "style": "primary"},
		{"type": "divider", "title": "Questions"},
		{"type": "image", "url": "https://placehold.co/600x300", "caption": "A placeholder image", "link": ""},
		{"type": "youtube", "url": "https://www.youtube.com/watch?v=dQw4w9WgXcQ"},
		{"type": "toggle_text", "title": "Read more", "content": "Hidden until the player opens it."},
		{"type": "random_text", "texts": []any{"The first text.", "The second text.", "The third text."}},
		{"type": "clue", "clue": "Look **up**.", "description": "A hint", "button_label": "Reveal clue", "points": -2},
		{"type": "map", "latitude": -41.2865, "longitude": 174.7762, "zoom": 14, "caption": "Wellington"},
		{"type": "geofence", "prompt": "Stand in the square.", "area": blocks.NewCircle(-41.2865, 174.7762, 60)},
		{"type": "scan", "prompt": "Scan the code on the gate.", "match": "exact", "allow_typing": true, "points": 10, "codes": []any{
			map[string]any{"value": "gate-1234"},
			map[string]any{"value": "9780141036144"},
		}},
		{"type": "password", "prompt": "What is the password?", "answer": "kiwi", "unlocked_content": "Correct.", "points": 5},
		{"type": "pincode", "prompt": "Enter the four digits.", "pincode": "1234", "unlocked_content": "Open.", "points": 5},
		{"type": "free_text", "prompt": "What did you notice?", "placeholder": "Write here...", "points": 5},
		{"type": "photo", "prompt": "Take a photo of your team.", "max_images": 3, "points": 10},
		{"type": "rating", "prompt": "How was it?"},
		{"type": "choice", "prompt": "Which path will you take?", "button_text": "Confirm choice", "multi_select": false,
			"options": []any{
				map[string]any{"label": "The river", "sets": "path_river"},
				map[string]any{"label": "The hill", "sets": "path_hill"},
				map[string]any{"label": "The bush", "sets": "path_bush"},
			}},
		{"type": "quiz", "question": "Which of these are birds?", "multiple_choice": true, "randomise_order": true,
			"allow_retry": true, "unlocked_content": "Kiwi and tui are both birds.", "points": 10,
			"options": []any{
				map[string]any{"id": "option_0", "text": "Kiwi", "correct": true},
				map[string]any{"id": "option_1", "text": "Tui", "correct": true},
				map[string]any{"id": "option_2", "text": "Possum", "correct": false},
				map[string]any{"id": "option_3", "text": "Tuatara", "correct": false},
			}},
		{"type": "sorting", "content": "Put these in the order they happen.", "scoring": "correct_item_correct_place",
			"points": 10, "items": []any{
				map[string]any{"description": "Seed", "position": 1},
				map[string]any{"description": "Shoot", "position": 2},
				map[string]any{"description": "Flower", "position": 3},
				map[string]any{"description": "Fruit", "position": 4},
			}},
		{"type": "checklist", "content": "Tick each one as you go.", "points": 5, "items": []any{
			map[string]any{"description": "Find the gate"},
			map[string]any{"description": "Count the steps"},
			map[string]any{"description": "Take a photo"},
		}},
	}
}
