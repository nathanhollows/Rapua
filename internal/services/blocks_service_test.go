package services_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v7/blocks"
	"github.com/nathanhollows/Rapua/v7/internal/repositories"
	"github.com/nathanhollows/Rapua/v7/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupBlocksService(t *testing.T) (services.BlockService, *bun.DB, func()) {
	dbc, cleanup := setupDB(t)

	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blocksRepo := repositories.NewBlockRepository(dbc, blockStateRepo)

	blocksService := services.NewBlockService(blocksRepo, blockStateRepo)

	return *blocksService, dbc, cleanup
}

func TestBlockService_NewBlockWithOwnerAndContext(t *testing.T) {
	testCases := []struct {
		name       string
		locationID string
		blockType  string
		wantErr    bool
	}{
		{
			name:       "Valid block creation",
			locationID: gofakeit.UUID(),
			blockType:  "text",
			wantErr:    false,
		},
		{
			name:       "Missing location ID",
			locationID: "",
			blockType:  "text",
			wantErr:    true,
		},
		{
			name:       "Missing blockType",
			locationID: gofakeit.UUID(),
			blockType:  "",
			wantErr:    true,
		},
	}

	svc, _, cleanup := setupBlocksService(t)
	defer cleanup()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			blk, err := svc.NewBlockWithOwnerAndContext(
				context.Background(),
				tc.locationID,
				blocks.ContextLocationContent,
				tc.blockType,
			)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, blk.GetID(), "Expected a non-empty block ID")
				assert.Equal(t, tc.locationID, blk.GetOwnerID(), "Owner ID should match")
				assert.Equal(t, tc.blockType, blk.GetType(), "Block type should match")
			}
		})
	}
}

func TestBlockService_NewBlockState(t *testing.T) {
	svc, dbc, cleanup := setupBlocksService(t)
	defer cleanup()

	// Create parent chain for valid team codes (user -> instance -> team)
	parents := createTestParents(t, dbc)
	insertTestTeam(t, dbc, "TEAM1", parents.QuestID)
	insertTestTeam(t, dbc, "TEAM2", parents.QuestID)

	// Create real blocks so block IDs satisfy FK constraints
	validBlock, err := svc.NewBlockWithOwnerAndContext(
		context.Background(),
		parents.LocationID,
		blocks.ContextLocationContent,
		"text",
	)
	require.NoError(t, err)

	validBlock2, err := svc.NewBlockWithOwnerAndContext(
		context.Background(),
		parents.LocationID,
		blocks.ContextLocationContent,
		"text",
	)
	require.NoError(t, err)

	testCases := []struct {
		name    string
		blockID string
		runCode string
		wantErr bool
	}{
		{
			name:    "Valid block state",
			blockID: validBlock.GetID(),
			runCode: "TEAM1",
			wantErr: false,
		},
		{
			name:    "Missing blockID",
			blockID: "",
			runCode: "TEAM2",
			wantErr: true,
		},
		{
			name:    "Missing runCode",
			blockID: validBlock2.GetID(),
			runCode: "",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state, err := svc.NewBlockState(context.Background(), tc.blockID, tc.runCode, parents.QuestID)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, state.GetBlockID())
				assert.Equal(t, tc.blockID, state.GetBlockID())
				assert.Equal(t, tc.runCode, state.GetPlayerID())
			}
		})
	}
}

func TestBlockService_NewMockBlockState(t *testing.T) {
	testCases := []struct {
		name    string
		blockID string
		runCode string
		wantErr bool
	}{
		{
			name:    "Valid mock block state",
			blockID: gofakeit.UUID(),
			runCode: "MOCKTEAM",
			wantErr: false,
		},
		{
			name:    "No block ID",
			blockID: "",
			runCode: "TEAMX",
			wantErr: true,
		},
		{
			name:    "No team code",
			blockID: gofakeit.UUID(),
			runCode: "",
			wantErr: false, // This is for admin use, so we expect no team code
		},
	}

	svc, _, cleanup := setupBlocksService(t)
	defer cleanup()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state, err := svc.NewMockBlockState(context.Background(), tc.blockID, tc.runCode, "test-quest")
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, state.GetBlockID())
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
				blk, err := svc.NewBlockWithOwnerAndContext(
					context.Background(),
					gofakeit.UUID(),
					blocks.ContextLocationContent,
					"text",
				)
				if err != nil {
					return "", err
				}
				return blk.GetID(), nil
			},
			wantErr: false,
		},
		{
			name: "Non-existent block",
			setupFn: func(_ services.BlockService) (string, error) {
				return gofakeit.UUID(), nil
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, cleanup := setupBlocksService(t)
			defer cleanup()

			blockID, err := tc.setupFn(svc)
			require.NoError(t, err, "setupFn should not fail")

			blk, err := svc.GetByBlockID(context.Background(), blockID)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, blockID, blk.GetID())
			}
		})
	}
}

func TestBlockService_GetBlockWithStateByBlockIDAndRunCode(t *testing.T) {
	testCases := []struct {
		name    string
		setupFn func(svc services.BlockService) (string, string, error) // (blockID, runCode)
		wantErr bool
	}{
		{
			name: "Valid block + state",
			setupFn: func(svc services.BlockService) (string, string, error) {
				// 1) Create block
				blk, err := svc.NewBlockWithOwnerAndContext(
					context.Background(),
					gofakeit.UUID(),
					blocks.ContextLocationContent,
					"checklist",
				)
				if err != nil {
					return "", "", err
				}
				// 2) Create block state
				st, err := svc.NewBlockState(context.Background(), blk.GetID(), "TEAM123", "test-quest")
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
				blk, err := svc.NewBlockWithOwnerAndContext(
					context.Background(),
					gofakeit.UUID(),
					blocks.ContextLocationContent,
					"checklist",
				)
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
			svc, dbc, cleanup := setupBlocksService(t)
			defer cleanup()

			// Create parent chain for team codes used in block states
			parents := createTestParents(t, dbc)
			insertTestTeam(t, dbc, "TEAM123", parents.QuestID)
			insertTestTeam(t, dbc, "NOSUCHTEAM", parents.QuestID)

			blockID, runCode, err := tc.setupFn(svc)
			require.NoError(t, err, "setup should succeed")

			blk, st, err := svc.GetBlockWithStateByBlockIDAndRunCode(
				context.Background(), blockID, runCode, "test-quest",
			)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, blockID, blk.GetID())
				assert.Equal(t, runCode, st.GetPlayerID())
			}
		})
	}
}

func TestBlockService_FindByOwnerID(t *testing.T) {
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
			svc, _, cleanup := setupBlocksService(t)
			defer cleanup()

			// Setup: create tc.blockCount blocks (if locationID is not empty)
			for range tc.blockCount {
				_, err := svc.NewBlockWithOwnerAndContext(
					context.Background(),
					tc.locationID,
					blocks.ContextLocationContent,
					"checklist",
				)
				require.NoError(t, err, "block creation should succeed in setup")
			}

			blocksFound, err := svc.FindByOwnerID(context.Background(), tc.locationID)
			if tc.wantErr {
				require.Error(t, err)
				assert.Empty(t, blocksFound)
			} else {
				require.NoError(t, err)
				assert.Len(t, blocksFound, tc.blockCount, "expected to find exactly %d blocks", tc.blockCount)
			}
		})
	}
}

func TestBlockService_FindByOwnerIDAndRunCodeWithState(t *testing.T) {
	testCases := []struct {
		name         string
		locationID   string
		runCode      string
		blockCount   int
		stateCreated bool
		wantErr      bool
	}{
		{
			name:         "Blocks with matching state",
			locationID:   gofakeit.UUID(),
			runCode:      gofakeit.Password(false, true, false, false, false, 5),
			blockCount:   2,
			stateCreated: true,
			wantErr:      false,
		},
		{
			name:       "Empty location ID",
			locationID: "",
			runCode:    gofakeit.Password(false, true, false, false, false, 5),
			wantErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, dbc, cleanup := setupBlocksService(t)
			defer cleanup()

			// Create parent chain for team codes used in block states
			if tc.runCode != "" && tc.stateCreated {
				parents := createTestParents(t, dbc)
				insertTestTeam(t, dbc, tc.runCode, parents.QuestID)
			}

			if tc.locationID != "" {
				// Create blocks
				for range tc.blockCount {
					blk, err := svc.NewBlockWithOwnerAndContext(
						context.Background(),
						tc.locationID,
						blocks.ContextLocationContent,
						"checklist",
					)
					require.NoError(t, err)
					if tc.stateCreated {
						_, createErr := svc.NewBlockState(context.Background(), blk.GetID(), tc.runCode, "test-quest")
						require.NoError(t, createErr)
					}
				}
			}

			blocksFound, states, err := svc.FindByOwnerIDAndRunCodeWithState(
				context.Background(),
				tc.locationID,
				tc.runCode,
				"test-quest",
			)
			if tc.wantErr {
				require.Error(t, err)
				assert.Empty(t, blocksFound)
				assert.Empty(t, states)
			} else {
				require.NoError(t, err)
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
				blk, err := svc.NewBlockWithOwnerAndContext(
					context.Background(),
					gofakeit.UUID(),
					blocks.ContextLocationContent,
					"checklist",
				)
				if err != nil {
					return nil, err
				}
				// Create state
				st, err := svc.NewBlockState(context.Background(), blk.GetID(), "TEAMUP", "test-quest")
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
			svc, dbc, cleanup := setupBlocksService(t)
			defer cleanup()

			// Create parent chain for team code "TEAMUP"
			parents := createTestParents(t, dbc)
			insertTestTeam(t, dbc, "TEAMUP", parents.QuestID)

			initialState, err := tc.setupFn(svc)
			require.NoError(t, err, "setup should not fail")

			updated, err := svc.UpdateState(context.Background(), initialState)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
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
			svc, _, cleanup := setupBlocksService(t)
			defer cleanup()

			// Create blocks
			var ids []string
			for range tc.blockCount {
				blk, err := svc.NewBlockWithOwnerAndContext(
					context.Background(),
					tc.locationID,
					blocks.ContextLocationContent,
					"checklist",
				)
				require.NoError(t, err)
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
				require.Error(t, err)
			} else {
				require.NoError(t, err)
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
				_, err := svc.NewBlockWithOwnerAndContext(
					context.Background(),
					locID,
					blocks.ContextLocationContent,
					"text",
				)
				return err
			},
			wantVal: false,
		},
		{
			name:       "Validation required",
			locationID: "LOC-VALID-1",
			setupFn: func(svc services.BlockService, locID string) error {
				// Create block(s) that do require validation
				_, err := svc.NewBlockWithOwnerAndContext(
					context.Background(),
					locID,
					blocks.ContextLocationContent,
					"checklist",
				)
				return err
			},
			wantVal: true,
		},
		{
			name:       "Empty location ID",
			locationID: "",
			setupFn:    func(_ services.BlockService, _ string) error { return nil },
			wantErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, cleanup := setupBlocksService(t)
			defer cleanup()

			err := tc.setupFn(svc, tc.locationID)
			require.NoError(t, err, "setup should not fail")

			valRequired, err := svc.CheckValidationRequiredForLocation(context.Background(), tc.locationID)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantVal, valRequired, "Expected validation required to match")
			}
		})
	}
}

func TestBlockService_CheckValidationRequiredForCheckIn(t *testing.T) {
	testCases := []struct {
		name       string
		locationID string
		runCode    string
		setupFn    func(svc services.BlockService, locID, team string) error
		wantErr    bool
		wantVal    bool
	}{
		{
			name:       "Validation required",
			locationID: "LOC-CHIN-1",
			runCode:    "TEAMCHK",
			setupFn: func(svc services.BlockService, locID, team string) error {
				// Create block that needs validation
				blk, err := svc.NewBlockWithOwnerAndContext(
					context.Background(),
					locID,
					blocks.ContextLocationContent,
					"checklist",
				)
				if err != nil {
					return err
				}
				// Create block state that isn't validated
				_, err = svc.NewBlockState(context.Background(), blk.GetID(), team, "test-quest")
				return err
			},
			wantVal: true,
		},
		{
			name:       "No validation needed",
			locationID: "LOC-CHIN-2",
			runCode:    "TEAMCHK2",
			setupFn: func(svc services.BlockService, locID, _ string) error {
				// Maybe a block that doesn't require validation at all
				_, err := svc.NewBlockWithOwnerAndContext(
					context.Background(),
					locID,
					blocks.ContextLocationContent,
					"text",
				)
				return err
			},
			wantVal: false,
		},
		{
			name:       "Empty location or team code",
			locationID: "",
			runCode:    "",
			setupFn:    func(_ services.BlockService, _, _ string) error { return nil },
			wantErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, dbc, cleanup := setupBlocksService(t)
			defer cleanup()

			// Create parent chain and team for tests that use NewBlockState
			if tc.runCode != "" {
				parents := createTestParents(t, dbc)
				insertTestTeam(t, dbc, tc.runCode, parents.QuestID)
			}

			err := tc.setupFn(svc, tc.locationID, tc.runCode)
			require.NoError(t, err, "setup should not fail")

			valRequired, err := svc.CheckValidationRequiredForCheckIn(
				context.Background(), tc.locationID, tc.runCode, "test-quest",
			)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantVal, valRequired, "Expected validation result to match")
			}
		})
	}
}

func TestBlockService_FindByOwnerIDAndContext(t *testing.T) {
	testCases := []struct {
		name         string
		locationID   string
		context      blocks.BlockContext
		blockCount   int
		otherContext blocks.BlockContext
		wantErr      bool
	}{
		{
			name:         "Find blocks with specific context",
			locationID:   gofakeit.UUID(),
			context:      blocks.ContextLocationContent,
			blockCount:   2,
			otherContext: blocks.ContextNavigation,
			wantErr:      false,
		},
		{
			name:       "Empty location ID",
			locationID: "",
			context:    blocks.ContextLocationContent,
			wantErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, cleanup := setupBlocksService(t)
			defer cleanup()

			if tc.locationID != "" {
				// Create blocks with target context
				for range tc.blockCount {
					_, err := svc.NewBlockWithOwnerAndContext(
						context.Background(),
						tc.locationID,
						tc.context,
						"text",
					)
					require.NoError(t, err)
				}

				// Create a block with different context
				if tc.otherContext != "" {
					_, err := svc.NewBlockWithOwnerAndContext(
						context.Background(),
						tc.locationID,
						tc.otherContext,
						"text",
					)
					require.NoError(t, err)
				}
			}

			blocksFound, err := svc.FindByOwnerIDAndContext(context.Background(), tc.locationID, tc.context)
			if tc.wantErr {
				require.Error(t, err)
				assert.Empty(t, blocksFound)
			} else {
				require.NoError(t, err)
				assert.Len(t, blocksFound, tc.blockCount)
			}
		})
	}
}

func TestBlockService_FindByOwnerIDAndRunCodeWithStateAndContext(t *testing.T) {
	testCases := []struct {
		name         string
		locationID   string
		runCode      string
		context      blocks.BlockContext
		blockCount   int
		stateCreated bool
		wantErr      bool
	}{
		{
			name:         "Find blocks with context and states",
			locationID:   gofakeit.UUID(),
			runCode:      gofakeit.Password(false, true, false, false, false, 5),
			context:      blocks.ContextLocationContent,
			blockCount:   2,
			stateCreated: true,
			wantErr:      false,
		},
		{
			name:       "Empty location ID",
			locationID: "",
			runCode:    gofakeit.Password(false, true, false, false, false, 5),
			context:    blocks.ContextLocationContent,
			wantErr:    true,
		},
		{
			// Simulates preview mode: team code is empty so no DB row exists.
			// Blocks should still be returned with in-memory mock states.
			name:         "Empty team code returns blocks with mock states",
			locationID:   gofakeit.UUID(),
			runCode:      "",
			context:      blocks.ContextStart,
			blockCount:   2,
			stateCreated: false,
			wantErr:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, dbc, cleanup := setupBlocksService(t)
			defer cleanup()

			// Create parent chain and team for tests that use NewBlockState
			if tc.runCode != "" && tc.stateCreated {
				parents := createTestParents(t, dbc)
				insertTestTeam(t, dbc, tc.runCode, parents.QuestID)
			}

			if tc.locationID != "" {
				// Create blocks with specific context
				for range tc.blockCount {
					blk, err := svc.NewBlockWithOwnerAndContext(
						context.Background(),
						tc.locationID,
						tc.context,
						"checklist",
					)
					require.NoError(t, err)
					if tc.stateCreated {
						_, stateErr := svc.NewBlockState(context.Background(), blk.GetID(), tc.runCode, "test-quest")
						require.NoError(t, stateErr)
					}
				}

				// Create a block with different context
				_, err := svc.NewBlockWithOwnerAndContext(
					context.Background(),
					tc.locationID,
					blocks.ContextNavigation,
					"text",
				)
				require.NoError(t, err)
			}

			blocksFound, states, err := svc.FindByOwnerIDAndRunCodeWithStateAndContext(
				context.Background(),
				tc.locationID,
				tc.runCode,
				"test-quest",
				tc.context,
			)
			if tc.wantErr {
				require.Error(t, err)
				assert.Empty(t, blocksFound)
				assert.Empty(t, states)
			} else {
				require.NoError(t, err)
				assert.Len(t, blocksFound, tc.blockCount)
				if tc.stateCreated || tc.runCode == "" {
					// stateCreated: states were persisted before the call.
					// runCode == "": preview path — mock states must still be returned for every block.
					assert.Len(t, states, tc.blockCount)
				}
			}
		})
	}
}

func TestBlockService_UpdateBlock(t *testing.T) {
	testCases := []struct {
		name    string
		setupFn func(svc services.BlockService) (blocks.Block, map[string][]string, error)
		wantErr bool
	}{
		{
			name: "Valid block update",
			setupFn: func(svc services.BlockService) (blocks.Block, map[string][]string, error) {
				blk, err := svc.NewBlockWithOwnerAndContext(
					context.Background(),
					gofakeit.UUID(),
					blocks.ContextLocationContent,
					"image",
				)
				if err != nil {
					return nil, nil, err
				}
				data := map[string][]string{
					"url":     {"/new-image.jpg"},
					"caption": {"Updated caption"},
				}
				return blk, data, nil
			},
			wantErr: false,
		},
		{
			name: "Update with invalid data",
			setupFn: func(svc services.BlockService) (blocks.Block, map[string][]string, error) {
				blk, err := svc.NewBlockWithOwnerAndContext(
					context.Background(),
					gofakeit.UUID(),
					blocks.ContextLocationContent,
					"text",
				)
				if err != nil {
					return nil, nil, err
				}
				// Provide invalid/empty data
				data := map[string][]string{}
				return blk, data, nil
			},
			wantErr: false, // Empty data should not error, just not update anything
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, cleanup := setupBlocksService(t)
			defer cleanup()

			blk, data, err := tc.setupFn(svc)
			require.NoError(t, err, "setup should not fail")

			updatedBlock, err := svc.UpdateBlock(context.Background(), blk, data)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, updatedBlock)
				assert.Equal(t, blk.GetID(), updatedBlock.GetID())
			}
		})
	}
}
