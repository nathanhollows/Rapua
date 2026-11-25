package services_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
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
				blocks.ContextObjectiveProof,
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

func TestBlockService_NewBlockWithOwnerAndContext_RejectsWrongContext(t *testing.T) {
	svc, _, cleanup := setupBlocksService(t)
	defer cleanup()
	ctx := context.Background()

	// team_name is registered only for ContextStart (system blocks). Creating
	// one mid-quest, in an objective's proof/reveal zone, must be rejected
	// server-side, not just hidden by the admin dropdown's client-side filter.
	_, err := svc.NewBlockWithOwnerAndContext(ctx, gofakeit.UUID(), blocks.ContextObjectiveProof, "team_name")
	require.ErrorIs(t, err, services.ErrBlockNotValidForContext)

	_, err = svc.NewBlockWithOwnerAndContext(ctx, gofakeit.UUID(), blocks.ContextObjectiveReveal, "team_name")
	require.ErrorIs(t, err, services.ErrBlockNotValidForContext)

	// Sanity check: team_name in its actual home context still works.
	blk, err := svc.NewBlockWithOwnerAndContext(ctx, gofakeit.UUID(), blocks.ContextStart, "team_name")
	require.NoError(t, err)
	assert.Equal(t, "team_name", blk.GetType())
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
		parents.QuestID,
		blocks.ContextObjectiveProof,
		"text",
	)
	require.NoError(t, err)

	validBlock2, err := svc.NewBlockWithOwnerAndContext(
		context.Background(),
		parents.QuestID,
		blocks.ContextObjectiveProof,
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
					blocks.ContextObjectiveProof,
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
					blocks.ContextObjectiveProof,
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
					blocks.ContextObjectiveProof,
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
					blocks.ContextObjectiveProof,
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
						blocks.ContextObjectiveProof,
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
					blocks.ContextObjectiveProof,
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
			context:      blocks.ContextObjectiveProof,
			blockCount:   2,
			otherContext: blocks.ContextObjectiveReveal,
			wantErr:      false,
		},
		{
			name:       "Empty location ID",
			locationID: "",
			context:    blocks.ContextObjectiveProof,
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
			context:      blocks.ContextObjectiveProof,
			blockCount:   2,
			stateCreated: true,
			wantErr:      false,
		},
		{
			name:       "Empty location ID",
			locationID: "",
			runCode:    gofakeit.Password(false, true, false, false, false, 5),
			context:    blocks.ContextObjectiveProof,
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
					blocks.ContextObjectiveReveal,
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
					blocks.ContextObjectiveProof,
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
					blocks.ContextObjectiveProof,
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

func TestBlockService_FindPointsByOwnerIDs(t *testing.T) {
	svc, dbc, cleanup := setupBlocksService(t)
	defer cleanup()
	ctx := context.Background()

	ownerA := gofakeit.UUID()
	ownerB := gofakeit.UUID()
	ownerC := gofakeit.UUID() // owns no blocks at all

	insertBlock := func(ownerID string, points int) {
		t.Helper()
		blk := &models.Block{
			ID:      gofakeit.UUID(),
			OwnerID: ownerID,
			Type:    "checklist",
			Context: blocks.ContextObjectiveProof,
			Data:    json.RawMessage(`{}`),
			Points:  points,
		}
		_, err := dbc.NewInsert().Model(blk).Exec(ctx)
		require.NoError(t, err)
	}
	insertBlock(ownerA, 5)
	insertBlock(ownerA, 3)
	insertBlock(ownerB, 10)

	points, err := svc.FindPointsByOwnerIDs(ctx, []string{ownerA, ownerB, ownerC})
	require.NoError(t, err)
	assert.Equal(t, 8, points[ownerA], "an owner's points are its own blocks summed")
	assert.Equal(t, 10, points[ownerB])
	_, hasC := points[ownerC]
	assert.False(t, hasC, "an owner with no blocks is absent from the map rather than present with 0")

	t.Run("empty ownerIDs returns nothing without erroring", func(t *testing.T) {
		points, err := svc.FindPointsByOwnerIDs(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, points)
	})
}

func TestBlockService_MoveBlock(t *testing.T) {
	svc, _, cleanup := setupBlocksService(t)
	defer cleanup()
	ctx := context.Background()
	owner := gofakeit.UUID()

	newBlock := func(blockContext blocks.BlockContext) string {
		blk, err := svc.NewBlockWithOwnerAndContext(ctx, owner, blockContext, "checklist")
		require.NoError(t, err)
		return blk.GetID()
	}
	idsIn := func(blockContext blocks.BlockContext) []string {
		found, err := svc.FindByOwnerIDAndContext(ctx, owner, blockContext)
		require.NoError(t, err)
		out := make([]string, 0, len(found))
		for _, b := range found {
			out = append(out, b.GetID())
		}
		return out
	}

	proofA, proofB := newBlock(blocks.ContextObjectiveProof), newBlock(blocks.ContextObjectiveProof)
	reveal := newBlock(blocks.ContextObjectiveReveal)

	t.Run("moves a block across and orders the list it joins", func(t *testing.T) {
		// Dropped between the reveal's own block and the end: [reveal, proofA].
		require.NoError(t, svc.MoveBlock(ctx, proofA, blocks.ContextObjectiveReveal, []string{reveal, proofA}))

		assert.Equal(t, []string{proofB}, idsIn(blocks.ContextObjectiveProof))
		assert.Equal(t, []string{reveal, proofA}, idsIn(blocks.ContextObjectiveReveal))
	})

	t.Run("a block missing from the order goes last, not nowhere", func(t *testing.T) {
		require.NoError(t, svc.MoveBlock(ctx, proofB, blocks.ContextObjectiveReveal, []string{reveal}))

		assert.Empty(t, idsIn(blocks.ContextObjectiveProof))
		assert.Equal(t, proofB, idsIn(blocks.ContextObjectiveReveal)[len(idsIn(blocks.ContextObjectiveReveal))-1])
	})

	t.Run("only an objective's two halves trade blocks", func(t *testing.T) {
		err := svc.MoveBlock(ctx, reveal, blocks.ContextStart, nil)
		require.ErrorIs(t, err, services.ErrBlockNotMovable)

		sys, createErr := svc.NewBlockWithOwnerAndContext(ctx, gofakeit.UUID(), blocks.ContextStart, "team_name")
		require.NoError(t, createErr)
		err = svc.MoveBlock(ctx, sys.GetID(), blocks.ContextObjectiveProof, nil)
		require.ErrorIs(t, err, services.ErrBlockNotMovable, "a start-page block is not an objective's to move")
	})

	t.Run("an order naming another objective's block moves nothing", func(t *testing.T) {
		stranger, createErr := svc.NewBlockWithOwnerAndContext(
			ctx, gofakeit.UUID(), blocks.ContextObjectiveReveal, "checklist",
		)
		require.NoError(t, createErr)
		mine := newBlock(blocks.ContextObjectiveProof)

		err := svc.MoveBlock(ctx, mine, blocks.ContextObjectiveReveal, []string{stranger.GetID(), mine})
		require.ErrorIs(t, err, services.ErrBlockNotMovable)

		assert.Contains(t, idsIn(blocks.ContextObjectiveProof), mine, "still where it was")
		assert.NotContains(t, idsIn(blocks.ContextObjectiveReveal), mine)
	})

	t.Run("an unknown block is an error", func(t *testing.T) {
		require.Error(t, svc.MoveBlock(ctx, gofakeit.UUID(), blocks.ContextObjectiveProof, nil))
	})
}

func TestBlockService_CreateKit(t *testing.T) {
	svc, _, cleanup := setupBlocksService(t)
	defer cleanup()
	ctx := context.Background()

	types := func(list []blocks.Block) []string {
		out := make([]string, 0, len(list))
		for _, b := range list {
			out = append(out, b.GetType())
		}
		return out
	}

	t.Run("scan: instructions, then a scan with its QR code already made", func(t *testing.T) {
		owner := gofakeit.UUID()
		made, err := svc.CreateKit(ctx, owner, blocks.ContextObjectiveProof, services.KitScan, 10)
		require.NoError(t, err)
		assert.Equal(t, []string{"text", "scan"}, types(made))

		text, ok := made[0].(*blocks.MarkdownBlock)
		require.True(t, ok)
		assert.NotEmpty(t, text.Content, "the helper text is there to be edited, not blank")

		scan, ok := made[1].(*blocks.ScanBlock)
		require.True(t, ok)
		require.Len(t, scan.Codes, 1)
		assert.Regexp(t, `^[a-hj-km-np-z2-9]{4}-[a-hj-km-np-z2-9]{4}$`, scan.Codes[0].Value,
			"the form of code the editor's own Mint a QR makes")
		assert.True(t, scan.Codes[0].Generate, "marked for a printable image")
		assert.Equal(t, 10, scan.GetPoints())
		assert.Equal(t, 0, text.GetPoints(), "reading earns nothing")

		stored, err := svc.FindByOwnerIDAndContext(ctx, owner, blocks.ContextObjectiveProof)
		require.NoError(t, err)
		assert.Equal(t, []string{"text", "scan"}, types(stored), "saved, in order")
	})

	t.Run("each scan kit gets its own code", func(t *testing.T) {
		a, err := svc.CreateKit(ctx, gofakeit.UUID(), blocks.ContextObjectiveProof, services.KitScan, 0)
		require.NoError(t, err)
		b, err := svc.CreateKit(ctx, gofakeit.UUID(), blocks.ContextObjectiveProof, services.KitScan, 0)
		require.NoError(t, err)
		assert.NotEqual(t, a[1].(*blocks.ScanBlock).Codes[0].Value, b[1].(*blocks.ScanBlock).Codes[0].Value)
	})

	t.Run("location: one check, ready to place", func(t *testing.T) {
		made, err := svc.CreateKit(ctx, gofakeit.UUID(), blocks.ContextObjectiveProof, services.KitLocation, 10)
		require.NoError(t, err)
		assert.Equal(t, []string{"geofence"}, types(made))
		assert.Equal(t, 10, made[0].GetPoints())
	})

	t.Run("quiz: instructions and two quizzes", func(t *testing.T) {
		made, err := svc.CreateKit(ctx, gofakeit.UUID(), blocks.ContextObjectiveProof, services.KitQuiz, 10)
		require.NoError(t, err)
		assert.Equal(t, []string{"text", "quiz", "quiz"}, types(made))
	})

	t.Run("a game without scoring gets no points", func(t *testing.T) {
		made, err := svc.CreateKit(ctx, gofakeit.UUID(), blocks.ContextObjectiveProof, services.KitQuiz, 0)
		require.NoError(t, err)
		for _, b := range made {
			assert.Equal(t, 0, b.GetPoints())
		}
	})

	t.Run("a kit goes after what is already there", func(t *testing.T) {
		owner := gofakeit.UUID()
		_, err := svc.NewBlockWithOwnerAndContext(ctx, owner, blocks.ContextObjectiveProof, "checklist")
		require.NoError(t, err)
		_, err = svc.CreateKit(ctx, owner, blocks.ContextObjectiveProof, services.KitScan, 0)
		require.NoError(t, err)

		stored, err := svc.FindByOwnerIDAndContext(ctx, owner, blocks.ContextObjectiveProof)
		require.NoError(t, err)
		assert.Equal(t, []string{"checklist", "text", "scan"}, types(stored))
	})

	t.Run("an unknown kit, or no owner, makes nothing", func(t *testing.T) {
		owner := gofakeit.UUID()
		_, err := svc.CreateKit(ctx, owner, blocks.ContextObjectiveProof, "nonsense", 0)
		require.ErrorIs(t, err, services.ErrUnknownBlockKit)
		_, err = svc.CreateKit(ctx, "", blocks.ContextObjectiveProof, services.KitScan, 0)
		require.Error(t, err)

		stored, findErr := svc.FindByOwnerIDAndContext(ctx, owner, blocks.ContextObjectiveProof)
		require.NoError(t, findErr)
		assert.Empty(t, stored)
	})

	t.Run("a kit is refused in a context its blocks cannot be used in", func(t *testing.T) {
		_, err := svc.CreateKit(ctx, gofakeit.UUID(), blocks.ContextStart, services.KitScan, 0)
		require.ErrorIs(t, err, services.ErrBlockNotValidForContext)
	})

// Every empty start page needs a way in, so every start kit ends on the button
// that starts a run. These are what an import with `start: []` is offered.
func TestBlockService_CreatePageKit(t *testing.T) {
	svc, _, cleanup := setupBlocksService(t)
	defer cleanup()
	ctx := context.Background()

	types := func(list []blocks.Block) []string {
		out := make([]string, 0, len(list))
		for _, b := range list {
			out = append(out, b.GetType())
		}
		return out
	}

	t.Run("every start kit lets a player begin", func(t *testing.T) {
		for _, kit := range []services.BlockKit{
			services.KitStartScavenger, services.KitStartWelcome, services.KitStartRules,
		} {
			made, err := svc.CreatePageKit(ctx, gofakeit.UUID(), blocks.ContextStart, kit, "Garden Trail")
			require.NoError(t, err, kit)
			assert.Equal(t, "start_button", types(made)[len(made)-1], kit)
			assert.Contains(t, types(made), "game_status", kit)
		}
	})

	t.Run("the header carries the game's name", func(t *testing.T) {
		quest := gofakeit.UUID()
		made, err := svc.CreatePageKit(ctx, quest, blocks.ContextStart, services.KitStartWelcome, "Garden Trail")
		require.NoError(t, err)
		header, ok := made[0].(*blocks.HeaderBlock)
		require.True(t, ok)
		assert.Equal(t, "Garden Trail", header.TitleText)

		stored, err := svc.FindByOwnerIDAndContext(ctx, quest, blocks.ContextStart)
		require.NoError(t, err)
		assert.Len(t, stored, len(made))
	})

	t.Run("a nameless quest still gets a heading", func(t *testing.T) {
		made, err := svc.CreatePageKit(ctx, gofakeit.UUID(), blocks.ContextStart, services.KitStartWelcome, "")
		require.NoError(t, err)
		header, ok := made[0].(*blocks.HeaderBlock)
		require.True(t, ok)
		assert.Equal(t, "Welcome", header.TitleText)
	})

	t.Run("the team name can be changed and the countdown shows", func(t *testing.T) {
		made, err := svc.CreatePageKit(ctx, gofakeit.UUID(), blocks.ContextStart, services.KitStartScavenger, "X")
		require.NoError(t, err)
		for _, block := range made {
			switch typed := block.(type) {
			case *blocks.TeamNameChangerBlock:
				assert.True(t, typed.AllowChanging)
			case *blocks.GameStatusAlertBlock:
				assert.True(t, typed.ShowCountdown)
			}
		}
	})

	t.Run("finish: congratulations with a party popper", func(t *testing.T) {
		made, err := svc.CreatePageKit(ctx, gofakeit.UUID(), blocks.ContextFinish, services.KitFinishCongrats, "X")
		require.NoError(t, err)
		assert.Equal(t, []string{"header", "text"}, types(made))
		header, ok := made[0].(*blocks.HeaderBlock)
		require.True(t, ok)
		assert.Equal(t, "party-popper", header.Icon)
		assert.Equal(t, "Congratulations!", header.TitleText)
	})

	t.Run("the survey asks, and pays nothing", func(t *testing.T) {
		made, err := svc.CreatePageKit(ctx, gofakeit.UUID(), blocks.ContextFinish, services.KitFinishSurvey, "X")
		require.NoError(t, err)
		assert.Equal(t, []string{"header", "text", "rating", "free_text"}, types(made))
		for _, block := range made {
			assert.Zero(t, block.GetPoints(), block.GetType())
		}
	})

	t.Run("a kit stays on its own page", func(t *testing.T) {
		_, err := svc.CreatePageKit(ctx, gofakeit.UUID(), blocks.ContextFinish, services.KitStartWelcome, "X")
		require.ErrorIs(t, err, services.ErrBlockNotValidForContext)
		_, err = svc.CreatePageKit(ctx, gofakeit.UUID(), blocks.ContextStart, services.KitFinishCongrats, "X")
		require.ErrorIs(t, err, services.ErrBlockNotValidForContext)
	})

	t.Run("an objective template is not a page template", func(t *testing.T) {
		_, err := svc.CreatePageKit(ctx, gofakeit.UUID(), blocks.ContextStart, services.KitScan, "X")
		require.ErrorIs(t, err, services.ErrUnknownBlockKit)
	})
}
}
