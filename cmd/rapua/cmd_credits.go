package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/uptrace/bun"
	"github.com/urfave/cli/v2"
)

type addCreditsParams struct {
	Email        string
	Credits      int
	Prefix       string
	CustomReason string
}

func addCreditsToUser(
	ctx context.Context,
	params addCreditsParams,
	creditService *services.CreditService,
	userRepo repositories.UserRepository,
) error {
	var reasonPrefix string
	switch params.Prefix {
	case "Admin":
		reasonPrefix = models.CreditAdjustmentReasonPrefixAdmin
	case "Gift":
		reasonPrefix = models.CreditAdjustmentReasonPrefixGift
	default:
		return fmt.Errorf("invalid prefix %q: must be 'Admin' or 'Gift'", params.Prefix)
	}

	if params.Credits <= 0 {
		return errors.New("amount must be greater than 0")
	}

	reason := reasonPrefix
	if params.CustomReason != "" {
		reason = fmt.Sprintf("%s: %s", reasonPrefix, params.CustomReason)
	}

	user, err := userRepo.GetByEmail(ctx, params.Email)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	const (
		maxRetries       = 3
		retryDelayMillis = 100
	)
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = creditService.AddCredits(ctx, user.ID, 0, params.Credits, reason)
		if err == nil {
			return nil
		}

		if attempt < maxRetries && strings.Contains(err.Error(), "database is locked") {
			time.Sleep(time.Millisecond * retryDelayMillis * time.Duration(attempt))
			continue
		}

		return fmt.Errorf("failed to add credits: %w", err)
	}

	return errors.New("failed after maximum retries")
}

func newCreditsCommand(dbc *bun.DB, logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "credits",
		Usage: "manage user credits",
		Subcommands: []*cli.Command{
			{
				Name:      "add",
				Usage:     "add credits to a user (use --prefix=Admin or --prefix=Gift, defaults to Admin)",
				ArgsUsage: "<email> <amount> [reason]",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "prefix",
						Usage: "reason prefix: Admin or Gift",
						Value: "Admin",
					},
				},
				Action: func(c *cli.Context) error {
					//nolint:mnd // Magic number for argument count
					if c.NArg() < 2 {
						return errors.New("usage: rapua credits add <email> <amount> [reason]")
					}

					email := c.Args().Get(0)
					amountStr := c.Args().Get(1)
					//nolint:mnd // Magic number for argument count
					customReason := c.Args().Get(2)
					prefix := c.String("prefix")

					var credits int
					if _, err := fmt.Sscanf(amountStr, "%d", &credits); err != nil {
						return fmt.Errorf("invalid amount %q: must be a number", amountStr)
					}

					ctx := c.Context
					creditRepo := repositories.NewCreditRepository(dbc)
					runStartLogRepo := repositories.NewRunStartLogRepository(dbc)
					userRepo := repositories.NewUserRepository(dbc)
					transactor := db.NewTransactor(dbc)
					creditService := services.NewCreditService(transactor, creditRepo, runStartLogRepo, userRepo)

					params := addCreditsParams{
						Email:        email,
						Credits:      credits,
						Prefix:       prefix,
						CustomReason: customReason,
					}

					if err := addCreditsToUser(ctx, params, creditService, userRepo); err != nil {
						return err
					}

					user, _ := userRepo.GetByEmail(ctx, email)
					logger.Info("credits added successfully",
						"user", email,
						"amount", credits,
						"new_balance", user.PaidCredits)

					return nil
				},
			},
		},
	}
}
