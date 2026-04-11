package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/nathanhollows/Rapua/v7/internal/repositories"
	"github.com/nathanhollows/Rapua/v7/internal/services"
	"github.com/uptrace/bun"
	"github.com/urfave/cli/v2"
)

func newGenerateLoginCommand(dbc *bun.DB, logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:      "generate-login",
		Usage:     "generate a temporary admin login link",
		ArgsUsage: "<email>",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "duration",
				Usage: "link validity in seconds",
				Value: 60, //nolint:mnd // default link validity in seconds
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() < 1 {
				return errors.New("usage: rapua generate-login <email>")
			}

			email := c.Args().Get(0)
			duration := time.Duration(c.Int("duration")) * time.Second

			sessionKey := os.Getenv("SESSION_KEY")
			if sessionKey == "" {
				return errors.New("SESSION_KEY environment variable is not set")
			}

			siteURL := os.Getenv("SITE_URL")
			if siteURL == "" {
				siteURL = "http://localhost:8080"
				logger.Warn("SITE_URL not set, using default", "url", siteURL)
			}

			ctx := c.Context
			userRepo := repositories.NewUserRepository(dbc)
			magicTokenService := services.NewMagicTokenService([]byte(sessionKey))

			user, err := userRepo.GetByEmail(ctx, email)
			if err != nil {
				return fmt.Errorf("user not found with email %q: %w", email, err)
			}

			token, err := magicTokenService.GenerateToken(user.ID, duration)
			if err != nil {
				return fmt.Errorf("failed to generate token: %w", err)
			}

			loginURL := fmt.Sprintf("%s/login/magic/%s", siteURL, token)
			fmt.Fprintf(os.Stdout, "\nLogin link (valid for %d seconds):\n%s\n\n", c.Int("duration"), loginURL)

			return nil
		},
	}
}
