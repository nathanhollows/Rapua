package services_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v3/db"
	"github.com/nathanhollows/Rapua/v3/internal/services"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
	"github.com/stretchr/testify/assert"
)

func setupTeamsService(t *testing.T) (services.TeamService, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)

	checkinRepo := repositories.NewCheckInRepository(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	teamRepo := repositories.NewTeamRepository(dbc)
	locationRepo := repositories.NewLocationRepository(dbc)
	teamService := services.NewTeamService(transactor, teamRepo, checkinRepo, blockStateRepo, locationRepo)

	return teamService, cleanup
}

func TestTeamService_Functions(t *testing.T) {
	teamService, cleanup := setupTeamsService(t)
	defer cleanup()

	tests := []struct {
		name      string
		setup     func() (string, int, error)
		action    func(instanceID string, count int) ([]models.Team, error)
		assertion func(result []models.Team, err error)
	}{
		{
			name: "AddTeams successfully",
			setup: func() (string, int, error) {
				return "test-instance", 3, nil
			},
			action: func(instanceID string, count int) ([]models.Team, error) {
				return teamService.AddTeams(context.Background(), instanceID, count)
			},
			assertion: func(result []models.Team, err error) {
				assert.NoError(t, err)
				assert.Len(t, result, 3)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instanceID, count, err := tt.setup()
			assert.NoError(t, err)

			result, err := tt.action(instanceID, count)
			tt.assertion(result, err)
		})
	}
}

func TestTeamService_FindAll(t *testing.T) {
	teamService, cleanup := setupTeamsService(t)
	defer cleanup()

	instanceID := "test-instance"
	_, err := teamService.AddTeams(context.Background(), instanceID, 2)
	assert.NoError(t, err)

	teams, err := teamService.FindAll(context.Background(), instanceID)
	assert.NoError(t, err)
	assert.Len(t, teams, 2)
}

func TestTeamService_FindTeamByCode(t *testing.T) {
	teamService, cleanup := setupTeamsService(t)
	defer cleanup()

	instanceID := "test-instance"
	teams, err := teamService.AddTeams(context.Background(), instanceID, 1)
	assert.NoError(t, err)
	assert.Len(t, teams, 1)

	team, err := teamService.FindTeamByCode(context.Background(), teams[0].Code)
	assert.NoError(t, err)
	assert.Equal(t, teams[0].Code, team.Code)
}

func TestTeamService_Reset(t *testing.T) {
	teamService, cleanup := setupTeamsService(t)
	defer cleanup()

	tests := []struct {
		name      string
		setup     func() (string, []string, error)
		action    func(instanceID string, teamCodes []string) error
		assertion func(err error)
	}{
		{
			name: "Reset team progress",
			setup: func() (string, []string, error) {
				instanceID := gofakeit.UUID()

				teams, err := teamService.AddTeams(context.Background(), instanceID, 3)
				if err != nil {
					assert.NoError(t, err, "expected no error when adding teams")
				}

				var teamCodes []string
				for _, team := range teams {
					teamCodes = append(teamCodes, team.Code)
				}

				return instanceID, teamCodes, nil
			},
			action: func(instanceID string, teamCodes []string) error {
				return teamService.Reset(context.Background(), instanceID, teamCodes)
			},
			assertion: func(err error) {
				assert.NoError(t, err, "expected reset to succeed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instanceID, teamCodes, err := tt.setup()
			assert.NoError(t, err)

			err = tt.action(instanceID, teamCodes)
			tt.assertion(err)
		})
	}
}
