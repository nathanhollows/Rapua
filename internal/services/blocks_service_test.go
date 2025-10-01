package services_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v4/blocks"
	"github.com/nathanhollows/Rapua/v4/internal/services"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
)

func setupBlocksService(t *testing.T) (services.BlockService, func()) {
	dbc, cleanup := setupDB(t)

	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blocksRepo := repositories.NewBlockRepository(dbc, blockStateRepo)

	blocksService := services.NewBlockService(blocksRepo, blockStateRepo)

	return *blocksService, cleanup
}

func TestBlockService_NewBlock(t *testing.T) {
	testCases := []struct {
		name       string
		locationID string
		blockType  string
		wantErr    bool
	}{
		{
			name:       "Valid block creation",
			locationID: gofakeit.UUID(),
			blockType:  "markdown",
			wantErr:    false,
		},
		{
			name:       "Missing location ID",
			locationID: "",
			blockType:  "markdown",
			wantErr:    true,
		},
		{
			name:       "Missing blockType",
			locationID: gofakeit.UUID(),
			blockType:  "",
			wantErr:    true,
		},
	}

	svc, cleanup := setupBlocksService(t)
	defer cleanup()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			blk, err := svc.NewBlock(context.Background(), tc.locationID, tc.blockType)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, blk.GetID(), "Expected a non-empty block ID")
				assert.Equal(t, tc.locationID, blk.GetLocationID(), "Location ID should match")
				assert.Equal(t, tc.blockType, blk.GetType(), "Block type should match")
			}
		})
	}
}

func TestBlockService_NewBlockState(t *testing.T) {
	testCases := []struct {
		name     string
		blockID  string
		teamCode string
		wantErr  bool
	}{
		{
			name:     "Valid block state",
			blockID:  gofakeit.UUID(),
			teamCode: "TEAM1",
			wantErr:  false,
		},
		{
			name:     "Missing blockID",
			blockID:  "",
			teamCode: "TEAM2",
			wantErr:  true,
		},
		{
			name:     "Missing teamCode",
			blockID:  gofakeit.UUID(),
			teamCode: "",
			wantErr:  true,
		},
	}

	svc, cleanup := setupBlocksService(t)
	defer cleanup()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state, err := svc.NewBlockState(context.Background(), tc.blockID, tc.teamCode)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, state.GetBlockID())
				assert.Equal(t, tc.blockID, state.GetBlockID())
				assert.Equal(t, tc.teamCode, state.GetPlayerID())
			}
		})
	}
}

func TestBlockService_NewMockBlockState(t *testing.T) {
	testCases := []struct {
		name     string
		blockID  string
		teamCode string
		wantErr  bool
	}{
		{
			name:     "Valid mock block state",
			blockID:  gofakeit.UUID(),
			teamCode: "MOCKTEAM",
			wantErr:  false,
		},
		{
			name:     "No block ID",
			blockID:  "",
			teamCode: "TEAMX",
			wantErr:  true,
		},
		{
			name:     "No team code",
			blockID:  gofakeit.UUID(),
			teamCode: "",
			wantErr:  false, // This is for admin use, so we expect no team code
		},
	}

	svc, cleanup := setupBlocksService(t)
	defer cleanup()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state, err := svc.NewMockBlockState(context.Background(), tc.blockID, tc.teamCode)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, state.GetBlockID())
				assert.Equal(t, tc.blockID, state.GetBlockID())
			}
		})
	}
}

func TestBlockService_GetByBlockID(t *testing.T) {
	// Typically, you'd first create a block, then fetch it by ID.
	// Here we just illustrate the pattern.

	testCases := []struct {
		name    string
		setupFn func(svc services.BlockService) (string, error) // returns blockID
		wantErr bool
	}{
		{
			name: "Valid existing block",
			setupFn: func(svc services.BlockService) (string, error) {
				blk, err := svc.NewBlock(context.Background(), gofakeit.UUID(), "markdown")
				if err != nil {
					return "", err
				}
				return blk.GetID(), nil
			},
			wantErr: false,
		},
		{
			name: "Non-existent block",
			setupFn: func(svc services.BlockService) (string, error) {
				// Return random ID that doesn't exist
				return gofakeit.UUID(), nil
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, cleanup := setupBlocksService(t)
			defer cleanup()

			blockID, err := tc.setupFn(svc)
			assert.NoError(t, err, "setupFn should not fail")

			blk, err := svc.GetByBlockID(context.Background(), blockID)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, blockID, blk.GetID())
			}
		})
	}
}

func TestBlockService_GetBlockWithStateByBlockIDAndTeamCode(t *testing.T) {
	testCases := []struct {
		name    string
		setupFn func(svc services.BlockService) (string, string, error) // (blockID, teamCode)
		wantErr bool
	}{
		{
			name: "Valid block + state",
			setupFn: func(svc services.BlockService) (string, string, error) {
				// 1) Create block
				blk, err := svc.NewBlock(context.Background(), gofakeit.UUID(), "checklist")
				if err != nil {
					return "", "", err
				}
				// 2) Create block state
				st, err := svc.NewBlockState(context.Background(), blk.GetID(), "TEAM123")
				if err != nil {
					return "", "", err
				}
				return blk.GetID(), st.GetPlayerID(), nil
			},
			wantErr: false,
		},
		{
			// The service should create the state for the given team
			name: "No state for team",
			setupFn: func(svc services.BlockService) (string, string, error) {
				// 1) Create block
				blk, err := svc.NewBlock(context.Background(), gofakeit.UUID(), "checklist")
				if err != nil {
					return "", "", err
				}
				return blk.GetID(), "NOSUCHTEAM", nil
			},
			wantErr: false, // State should always be created
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, cleanup := setupBlocksService(t)
			defer cleanup()

			blockID, teamCode, err := tc.setupFn(svc)
			assert.NoError(t, err, "setup should succeed")

			blk, st, err := svc.GetBlockWithStateByBlockIDAndTeamCode(context.Background(), blockID, teamCode)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, blockID, blk.GetID())
				assert.Equal(t, teamCode, st.GetPlayerID())
			}
		})
	}
}

func TestBlockService_FindByLocationID(t *testing.T) {
	testCases := []struct {
		name       string
		locationID string
		blockCount int
		wantErr    bool
	}{
		{
			name:       "Multiple blocks found",
			locationID: gofakeit.UUID(),
			blockCount: 3,
			wantErr:    false,
		},
		{
			name:       "No blocks found (valid location)",
			locationID: gofakeit.UUID(),
			blockCount: 0,
			wantErr:    false,
		},
		{
			name:       "Empty locationID",
			locationID: "",
			blockCount: 0,
			wantErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, cleanup := setupBlocksService(t)
			defer cleanup()

			// Setup: create tc.blockCount blocks (if locationID is not empty)
			for range tc.blockCount {
				_, err := svc.NewBlock(context.Background(), tc.locationID, "checklist")
				assert.NoError(t, err, "block creation should succeed in setup")
			}

			blocksFound, err := svc.FindByLocationID(context.Background(), tc.locationID)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Empty(t, blocksFound)
			} else {
				assert.NoError(t, err)
				assert.Len(t, blocksFound, tc.blockCount, "expected to find exactly %d blocks", tc.blockCount)
			}
		})
	}
}

func TestBlockService_FindByLocationIDAndTeamCodeWithState(t *testing.T) {
	testCases := []struct {
		name         string
		locationID   string
		teamCode     string
		blockCount   int
		stateCreated bool
		wantErr      bool
	}{
		{
			name:         "Blocks with matching state",
			locationID:   gofakeit.UUID(),
			teamCode:     gofakeit.Password(false, true, false, false, false, 5),
			blockCount:   2,
			stateCreated: true,
			wantErr:      false,
		},
		{
			name:       "Empty location ID",
			locationID: "",
			teamCode:   gofakeit.Password(false, true, false, false, false, 5),
			wantErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, cleanup := setupBlocksService(t)
			defer cleanup()

			if tc.locationID != "" {
				// Create blocks
				for range tc.blockCount {
					blk, err := svc.NewBlock(context.Background(), tc.locationID, "checklist")
					assert.NoError(t, err)
					if tc.stateCreated {
						_, err := svc.NewBlockState(context.Background(), blk.GetID(), tc.teamCode)
						assert.NoError(t, err)
					}
				}
			}

			blocksFound, states, err := svc.FindByLocationIDAndTeamCodeWithState(
				context.Background(),
				tc.locationID,
				tc.teamCode,
			)
			if tc.wantErr {
				assert.Error(t, err)
				assert.Empty(t, blocksFound)
				assert.Empty(t, states)
			} else {
				assert.NoError(t, err)
				assert.Len(t, blocksFound, tc.blockCount)
				if tc.stateCreated {
					// We expect each block to have a PlayerState
					assert.Len(t, states, len(blocksFound), "states map should match blocks count")
				} else {
					// Might have zero states if none were created
					assert.Empty(t, states, "expected no states for this scenario")
				}
			}
		})
	}
}

func TestBlockService_UpdateState(t *testing.T) {
	testCases := []struct {
		name    string
		setupFn func(svc services.BlockService) (blocks.PlayerState, error)
		wantErr bool
	}{
		{
			name: "Valid state update",
			setupFn: func(svc services.BlockService) (blocks.PlayerState, error) {
				// Create block
				blk, err := svc.NewBlock(context.Background(), gofakeit.UUID(), "checklist")
				if err != nil {
					return nil, err
				}
				// Create state
				st, err := svc.NewBlockState(context.Background(), blk.GetID(), "TEAMUP")
				if err != nil {
					return nil, err
				}
				// Modify st as desired
				st.SetComplete(true)
				return st, nil
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, cleanup := setupBlocksService(t)
			defer cleanup()

			initialState, err := tc.setupFn(svc)
			assert.NoError(t, err, "setup should not fail")

			updated, err := svc.UpdateState(context.Background(), initialState)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, initialState.GetBlockID(), updated.GetBlockID())
				assert.True(t, updated.IsComplete(), "Expected state to be updated to 'Complete'")
			}
		})
	}
}

func TestBlockService_ReorderBlocks(t *testing.T) {
	testCases := []struct {
		name       string
		locationID string
		blockCount int
		reorderIDs []string
		wantErr    bool
	}{
		{
			name:       "Valid reorder",
			locationID: "LOC-REORDER",
			blockCount: 3,
			wantErr:    false,
		},
		// {
		// 	name:       "Mismatched reorder IDs",
		// 	locationID: "LOC-REORDER",
		// 	blockCount: 2,
		// 	reorderIDs: []string{"randomID"},
		// 	wantErr:    true,
		// },
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, cleanup := setupBlocksService(t)
			defer cleanup()

			// Create blocks
			var ids []string
			for range tc.blockCount {
				blk, err := svc.NewBlock(context.Background(), tc.locationID, "checklist")
				assert.NoError(t, err)
				ids = append(ids, blk.GetID())
			}

			if len(tc.reorderIDs) == 0 {
				// By default, reorder with all block IDs but in reversed order
				for left, right := 0, len(ids)-1; left < right; left, right = left+1, right-1 {
					ids[left], ids[right] = ids[right], ids[left]
				}
				tc.reorderIDs = ids
			}

			err := svc.ReorderBlocks(context.Background(), tc.reorderIDs)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBlockService_CheckValidationRequiredForLocation(t *testing.T) {
	testCases := []struct {
		name       string
		locationID string
		setupFn    func(svc services.BlockService, locID string) error
		wantErr    bool
		wantVal    bool
	}{
		{
			name:       "No validation required",
			locationID: "LOC-VALID-0",
			setupFn: func(svc services.BlockService, locID string) error {
				// Create block(s) that do not require validation
				_, err := svc.NewBlock(context.Background(), locID, "markdown")
				return err
			},
			wantVal: false,
		},
		{
			name:       "Validation required",
			locationID: "LOC-VALID-1",
			setupFn: func(svc services.BlockService, locID string) error {
				// Create block(s) that do require validation
				_, err := svc.NewBlock(context.Background(), locID, "checklist")
				return err
			},
			wantVal: true,
		},
		{
			name:       "Empty location ID",
			locationID: "",
			setupFn:    func(svc services.BlockService, locID string) error { return nil },
			wantErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, cleanup := setupBlocksService(t)
			defer cleanup()

			err := tc.setupFn(svc, tc.locationID)
			assert.NoError(t, err, "setup should not fail")

			valRequired, err := svc.CheckValidationRequiredForLocation(context.Background(), tc.locationID)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantVal, valRequired, "Expected validation required to match")
			}
		})
	}
}

func TestBlockService_CheckValidationRequiredForCheckIn(t *testing.T) {
	testCases := []struct {
		name       string
		locationID string
		teamCode   string
		setupFn    func(svc services.BlockService, locID, team string) error
		wantErr    bool
		wantVal    bool
	}{
		{
			name:       "Validation required",
			locationID: "LOC-CHIN-1",
			teamCode:   "TEAMCHK",
			setupFn: func(svc services.BlockService, locID, team string) error {
				// Create block that needs validation
				blk, err := svc.NewBlock(context.Background(), locID, "checklist")
				if err != nil {
					return err
				}
				// Create block state that isn't validated
				_, err = svc.NewBlockState(context.Background(), blk.GetID(), team)
				return err
			},
			wantVal: true,
		},
		{
			name:       "No validation needed",
			locationID: "LOC-CHIN-2",
			teamCode:   "TEAMCHK2",
			setupFn: func(svc services.BlockService, locID, team string) error {
				// Maybe a block that doesn't require validation at all
				_, err := svc.NewBlock(context.Background(), locID, "markdown")
				return err
			},
			wantVal: false,
		},
		{
			name:       "Empty location or team code",
			locationID: "",
			teamCode:   "",
			setupFn:    func(svc services.BlockService, locID, team string) error { return nil },
			wantErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, cleanup := setupBlocksService(t)
			defer cleanup()

			err := tc.setupFn(svc, tc.locationID, tc.teamCode)
			assert.NoError(t, err, "setup should not fail")

			valRequired, err := svc.CheckValidationRequiredForCheckIn(context.Background(), tc.locationID, tc.teamCode)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantVal, valRequired, "Expected validation result to match")
			}
		})
	}
}
