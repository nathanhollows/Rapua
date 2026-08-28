package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Test fixture base time for deterministic timestamps.
const baseTime = "2024-01-15T10:00:00Z"

func setupRunsService(t *testing.T) (services.RunService, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	transactor := db.NewTransactor(dbc)

	checkinRepo := repositories.NewCheckInRepository(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	teamRepo := repositories.NewRunRepository(dbc)
	locationRepo := repositories.NewLocationRepository(dbc)
	creditRepo := repositories.NewCreditRepository(dbc)
	runStartLogRepo := repositories.NewRunStartLogRepository(dbc)
	varStateRepo := repositories.NewRunVarStateRepository(dbc)
	creditService := services.NewCreditService(transactor, creditRepo, runStartLogRepo, nil)
	objectiveRepo := repositories.NewObjectiveRepository(dbc)
	objectiveContextCompletionRepo := repositories.NewObjectiveContextCompletionRepository(dbc)
	runService := services.NewRunService(
		transactor,
		teamRepo,
		checkinRepo,
		creditService,
		blockStateRepo,
		locationRepo,
		varStateRepo,
		objectiveRepo,
		objectiveContextCompletionRepo,
	)

	return *runService, dbc, cleanup
}

// getBaseTime returns a fixed time for deterministic testing.
func getBaseTime() time.Time {
	t, _ := time.Parse(time.RFC3339, baseTime)
	return t
}

func TestRunService_AddTeams(t *testing.T) {
	runService, dbc, cleanup := setupRunsService(t)
	defer cleanup()

	// Create FK-valid instances for each test case
	mkInstance := func() string {
		p := createTestParents(t, dbc)
		return p.QuestID
	}

	tests := []struct {
		name      string
		questID   string
		count     int
		wantCount int
		wantErr   bool
	}{
		{
			name:      "add teams successfully",
			questID:   mkInstance(),
			count:     3,
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:      "add single team",
			questID:   mkInstance(),
			count:     1,
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "add many teams",
			questID:   mkInstance(),
			count:     10,
			wantCount: 10,
			wantErr:   false,
		},
		{
			name:      "zero count should create no teams",
			questID:   mkInstance(),
			count:     0,
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "negative count should create no teams",
			questID:   mkInstance(),
			count:     -1,
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "empty instance ID fails with FK constraint",
			questID:   "",
			count:     3,
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := runService.AddTeams(context.Background(), tt.questID, tt.count)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, result, tt.wantCount)

			// Verify each team has proper values
			for _, team := range result {
				assert.NotEmpty(t, team.Code, "team code should not be empty")
				assert.Equal(t, tt.questID, team.QuestID, "instance ID should match")
			}
		})
	}
}

func TestRunService_FindAll(t *testing.T) {
	runService, dbc, cleanup := setupRunsService(t)
	defer cleanup()

	// Create FK-valid instances for each test case
	mkInstance := func() string {
		p := createTestParents(t, dbc)
		return p.QuestID
	}

	tests := []struct {
		name      string
		setupRuns int
		questID   string
		wantCount int
		wantErr   bool
	}{
		{
			name:      "find all teams for instance",
			setupRuns: 5,
			questID:   mkInstance(),
			wantCount: 5,
			wantErr:   false,
		},
		{
			name:      "find no teams for empty instance",
			setupRuns: 0,
			questID:   mkInstance(),
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "find teams with special characters in instance ID",
			setupRuns: 3,
			questID:   mkInstance(),
			wantCount: 3,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup: Create teams for this instance
			if tt.setupRuns > 0 {
				teams, err := runService.AddTeams(context.Background(), tt.questID, tt.setupRuns)
				require.NoError(t, err)
				require.Len(t, teams, tt.setupRuns, "setup should create expected number of teams")
			}

			// Test: Find all teams
			result, err := runService.FindAll(context.Background(), tt.questID)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, result, tt.wantCount)

			// Verify all teams belong to correct instance
			for _, team := range result {
				assert.Equal(t, tt.questID, team.QuestID)
			}
		})
	}
}

func TestRunService_FindTeamByCode(t *testing.T) {
	runService, dbc, cleanup := setupRunsService(t)
	defer cleanup()

	t.Run("find existing team by code", func(t *testing.T) {
		p := createTestParents(t, dbc)
		questID := p.QuestID
		teams, err := runService.AddTeams(context.Background(), questID, 1)
		require.NoError(t, err)
		require.Len(t, teams, 1)

		team, err := runService.GetRunByCode(context.Background(), teams[0].Code)
		require.NoError(t, err)
		assert.Equal(t, teams[0].Code, team.Code)
		assert.Equal(t, questID, team.QuestID)
	})

	t.Run("return error for non-existent code", func(t *testing.T) {
		nonExistentCode := gofakeit.LetterN(6)

		team, err := runService.GetRunByCode(context.Background(), nonExistentCode)
		require.Error(t, err, "should return error for non-existent code")
		assert.Nil(t, team, "team should be nil when not found")
	})

	t.Run("return error for empty code", func(t *testing.T) {
		team, err := runService.GetRunByCode(context.Background(), "")
		require.Error(t, err, "should return error for empty code")
		assert.Nil(t, team, "team should be nil for empty code")
	})

	t.Run("find correct team among multiple teams", func(t *testing.T) {
		p := createTestParents(t, dbc)
		questID := p.QuestID
		teams, err := runService.AddTeams(context.Background(), questID, 5)
		require.NoError(t, err)
		require.Len(t, teams, 5)

		// Test finding each team
		for _, expectedTeam := range teams {
			foundRun, err := runService.GetRunByCode(context.Background(), expectedTeam.Code)
			require.NoError(t, err)
			assert.Equal(t, expectedTeam.Code, foundRun.Code)
			assert.Equal(t, expectedTeam.QuestID, foundRun.QuestID)
		}
	})
}

func TestRunService_BuildObjectiveGroupMap(t *testing.T) {
	runService, _, cleanup := setupRunsService(t)
	defer cleanup()

	tests := []struct {
		name      string
		structure *models.GameStructure
		want      map[string]services.ObjectiveGroupInfo
	}{
		{
			name: "empty root structure",
			structure: &models.GameStructure{
				IsRoot:       true,
				ObjectiveIDs: []string{},
				SubGroups:    []models.GameStructure{},
			},
			want: map[string]services.ObjectiveGroupInfo{},
		},
		{
			name: "root with objectives only",
			structure: &models.GameStructure{
				IsRoot:       true,
				ObjectiveIDs: []string{"obj1", "obj2"},
				SubGroups:    []models.GameStructure{},
			},
			want: map[string]services.ObjectiveGroupInfo{},
		},
		{
			name: "single level with one group",
			structure: &models.GameStructure{
				IsRoot:       true,
				ObjectiveIDs: []string{},
				SubGroups: []models.GameStructure{
					{
						Name:         "Museum Tour",
						Color:        "primary",
						ObjectiveIDs: []string{"obj1", "obj2"},
					},
				},
			},
			want: map[string]services.ObjectiveGroupInfo{
				"obj1": {GroupName: "Museum Tour", GroupColor: "primary"},
				"obj2": {GroupName: "Museum Tour", GroupColor: "primary"},
			},
		},
		{
			name: "nested groups",
			structure: &models.GameStructure{
				IsRoot:       true,
				ObjectiveIDs: []string{},
				SubGroups: []models.GameStructure{
					{
						Name:         "Zone A",
						Color:        "primary",
						ObjectiveIDs: []string{"obj1"},
						SubGroups: []models.GameStructure{
							{
								Name:         "Subzone A1",
								Color:        "secondary",
								ObjectiveIDs: []string{"obj2", "obj3"},
							},
						},
					},
					{
						Name:         "Zone B",
						Color:        "accent",
						ObjectiveIDs: []string{"obj4"},
					},
				},
			},
			want: map[string]services.ObjectiveGroupInfo{
				"obj1": {GroupName: "Zone A", GroupColor: "primary"},
				"obj2": {GroupName: "Subzone A1", GroupColor: "secondary"},
				"obj3": {GroupName: "Subzone A1", GroupColor: "secondary"},
				"obj4": {GroupName: "Zone B", GroupColor: "accent"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runService.BuildObjectiveGroupMap(tt.structure)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRunService_GetIncompleteObjectives(t *testing.T) {
	runService, dbc, cleanup := setupRunsService(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)

	transactor := db.NewTransactor(dbc)
	objectiveRepo := repositories.NewObjectiveRepository(dbc)
	objectiveService := services.NewObjectiveService(transactor, objectiveRepo)
	completionRepo := repositories.NewObjectiveContextCompletionRepository(dbc)

	obj1, err := objectiveService.CreateObjective(ctx, parents.QuestID, "Find the key")
	require.NoError(t, err)
	obj2, err := objectiveService.CreateObjective(ctx, parents.QuestID, "Open the door")
	require.NoError(t, err)

	teams, err := runService.AddTeams(ctx, parents.QuestID, 1)
	require.NoError(t, err)
	require.Len(t, teams, 1)
	runCode := teams[0].Code

	t.Run("no completions yet: all objectives incomplete", func(t *testing.T) {
		incomplete, err := runService.GetIncompleteObjectives(ctx, parents.QuestID, runCode)
		require.NoError(t, err)
		assert.Len(t, incomplete, 2)
	})

	t.Run("one objective completed: only the other remains", func(t *testing.T) {
		_, err := completionRepo.Insert(ctx, runCode, obj1.ID, game.ContextObjectiveReveal)
		require.NoError(t, err)

		incomplete, err := runService.GetIncompleteObjectives(ctx, parents.QuestID, runCode)
		require.NoError(t, err)
		require.Len(t, incomplete, 1)
		assert.Equal(t, obj2.ID, incomplete[0].ID)
	})

	t.Run("proof completion alone does not mark an objective complete", func(t *testing.T) {
		// Its own objective, not obj1/obj2: this subtest must hold on its own,
		// independent of whether the earlier subtests ran or what they left behind.
		obj3, err := objectiveService.CreateObjective(ctx, parents.QuestID, "Escape the room")
		require.NoError(t, err)

		_, err = completionRepo.Insert(ctx, runCode, obj3.ID, game.ContextObjectiveProof)
		require.NoError(t, err)

		incomplete, err := runService.GetIncompleteObjectives(ctx, parents.QuestID, runCode)
		require.NoError(t, err)

		ids := make([]string, len(incomplete))
		for i, o := range incomplete {
			ids[i] = o.ID
		}
		assert.Contains(t, ids, obj3.ID)
	})
}
