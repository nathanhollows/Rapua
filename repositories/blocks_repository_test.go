package repositories_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v4/blocks"
	"github.com/nathanhollows/Rapua/v4/db"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
)

func setupBlockRepo(t *testing.T) (repositories.BlockRepository, db.Transactor, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)

	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	return blockRepo, transactor, cleanup
}

func TestBlockRepository(t *testing.T) {
	repo, transactor, cleanup := setupBlockRepo(t)
	defer cleanup()

	tests := []struct {
		name        string
		setup       func() (blocks.Block, error)
		action      func(block blocks.Block) (interface{}, error)
		assertion   func(result interface{}, err error)
		cleanupFunc func(block blocks.Block)
	}{
		{
			name: "Create new block",
			setup: func() (blocks.Block, error) {
				return repo.Create(
					context.Background(),
					blocks.NewImageBlock(
						blocks.BaseBlock{
							LocationID: gofakeit.UUID(),
							Type:       "image",
							Points:     10,
						},
					),
					gofakeit.UUID(),
					blocks.ContextLocationContent,
				)
			},
			action: func(block blocks.Block) (interface{}, error) {
				return repo.Create(context.Background(), block, gofakeit.UUID(), blocks.ContextLocationContent)
			},
			assertion: func(result interface{}, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			},
			cleanupFunc: func(block blocks.Block) {
				tx, err := transactor.BeginTx(context.Background(), &sql.TxOptions{})
				assert.NoError(t, err)
				err = repo.Delete(context.Background(), tx, block.GetID())
				if err != nil {
					err2 := tx.Rollback()
					if err2 != nil {
						t.Error(err2)
					}
					t.Error(err)
				} else {
					commitErr := tx.Commit()
					if commitErr != nil {
						t.Error(commitErr)
					}
				}
			},
		},
		{
			name: "Get block by ID",
			setup: func() (blocks.Block, error) {
				block, err := repo.Create(
					context.Background(),
					blocks.NewImageBlock(
						blocks.BaseBlock{
							LocationID: gofakeit.UUID(),
							Type:       "image",
							Points:     10,
						},
					),
					gofakeit.UUID(),
					blocks.ContextLocationContent,
				)
				if err != nil {
					return nil, err
				}
				createdBlock, _ := repo.Create(
					context.Background(),
					block,
					gofakeit.UUID(),
					blocks.ContextLocationContent,
				)
				return createdBlock, nil
			},
			action: func(block blocks.Block) (interface{}, error) {
				return repo.GetByID(context.Background(), block.GetID())
			},
			assertion: func(result interface{}, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			},
			cleanupFunc: func(block blocks.Block) {
				tx, err := transactor.BeginTx(context.Background(), &sql.TxOptions{})
				assert.NoError(t, err)
				err = repo.Delete(context.Background(), tx, block.GetID())
				if err != nil {
					rollbackErr := tx.Rollback()
					if rollbackErr != nil {
						t.Error(rollbackErr)
					}
					t.Error(err)
				} else {
					err := tx.Commit()
					if err != nil {
						t.Error(err)
					}
				}
			},
		},
		{
			name: "Update block",
			setup: func() (blocks.Block, error) {
				block, err := repo.Create(
					context.Background(),
					blocks.NewImageBlock(
						blocks.BaseBlock{
							LocationID: gofakeit.UUID(),
							Type:       "image",
							Points:     10,
						},
					),
					gofakeit.UUID(),
					blocks.ContextLocationContent,
				)
				if err != nil {
					return nil, err
				}
				createdBlock, _ := repo.Create(
					context.Background(),
					block,
					gofakeit.UUID(),
					blocks.ContextLocationContent,
				)
				return createdBlock, nil
			},
			action: func(block blocks.Block) (interface{}, error) {
				// This relies on the Image Block
				// TODO: mock the block data
				data := make(map[string][]string)
				data["url"] = []string{"/updated-url"}
				err := block.UpdateBlockData(data)
				if err != nil {
					return nil, err
				}
				return repo.Update(context.Background(), block)
			},
			assertion: func(result interface{}, err error) {
				assert.NoError(t, err)
				updatedBlock := result.(blocks.Block)
				assert.JSONEq(t, `{"content":"/updated-url","caption":"","link":""}`, string(updatedBlock.GetData()))
			},
			cleanupFunc: func(block blocks.Block) {
				tx, err := transactor.BeginTx(context.Background(), &sql.TxOptions{})
				assert.NoError(t, err)
				err = repo.Delete(context.Background(), tx, block.GetID())
				if err != nil {
					err2 := tx.Rollback()
					if err2 != nil {
						t.Error(err2)
					}
					t.Error(err)
				} else {
					commitErr := tx.Commit()
					if commitErr != nil {
						t.Error(commitErr)
					}
				}
			},
		},
		{
			name: "Delete block",
			setup: func() (blocks.Block, error) {
				block, err := repo.Create(
					context.Background(),
					blocks.NewImageBlock(
						blocks.BaseBlock{
							LocationID: gofakeit.UUID(),
							Type:       "image",
							Points:     10,
						},
					),
					gofakeit.UUID(),
					blocks.ContextLocationContent,
				)
				if err != nil {
					return nil, err
				}
				createdBlock, _ := repo.Create(
					context.Background(),
					block,
					gofakeit.UUID(),
					blocks.ContextLocationContent,
				)
				return createdBlock, nil
			},
			action: func(block blocks.Block) (interface{}, error) {
				tx, err := transactor.BeginTx(context.Background(), &sql.TxOptions{})
				assert.NoError(t, err)
				err = repo.Delete(context.Background(), tx, block.GetID())
				if err != nil {
					rollbackErr := tx.Rollback()
					if rollbackErr != nil {
						return nil, fmt.Errorf("rolling back transaction: %w", rollbackErr)
					}
					return nil, err
				} else {
					commitErr := tx.Commit()
					if commitErr != nil {
						return nil, fmt.Errorf("committing transaction: %w", commitErr)
					}
					return "deletion successful", nil
				}
			},
			assertion: func(result interface{}, err error) {
				assert.NoError(t, err)
			},
			cleanupFunc: func(block blocks.Block) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block, err := tt.setup()
			assert.NoError(t, err)
			result, err := tt.action(block)
			tt.assertion(result, err)
			if tt.cleanupFunc != nil {
				tt.cleanupFunc(block)
			}
		})
	}
}

func TestBlockRepository_Bulk(t *testing.T) {
	repo, transactor, cleanup := setupBlockRepo(t)
	defer cleanup()

	tests := []struct {
		name        string
		setup       func() ([]blocks.Block, error)
		action      func(block []blocks.Block) (interface{}, error)
		assertion   func(result interface{}, err error)
		cleanupFunc func(block []blocks.Block)
	}{
		{
			name: "Delete blocks by Location ID",
			setup: func() ([]blocks.Block, error) {
				// Create 3 blocks
				blockSet := make([]blocks.Block, 3)
				locationID := gofakeit.UUID()
				for i := range 3 {
					block, err := repo.Create(
						context.Background(),
						blocks.NewImageBlock(
							blocks.BaseBlock{
								LocationID: locationID,
								Type:       "image",
								Points:     10,
							},
						),
						locationID,
						blocks.ContextLocationContent,
					)
					if err != nil {
						return nil, err
					}
					blockSet[i] = block
				}
				return blockSet, nil
			},
			action: func(block []blocks.Block) (interface{}, error) {
				tx, err := transactor.BeginTx(context.Background(), &sql.TxOptions{})
				assert.NoError(t, err)

				err = repo.DeleteByOwnerID(context.Background(), tx, block[0].GetLocationID())
				if err != nil {
					rollbackErr := tx.Rollback()
					if rollbackErr != nil {
						return nil, rollbackErr
					}
					return nil, err
				}
				err = tx.Commit()
				if err != nil {
					return nil, err
				}

				for i, b := range block {
					t.Logf("Checking block %d, ID: %s", i, b.GetID())
					_, getErr := repo.GetByID(context.Background(), b.GetID())
					if getErr != nil && getErr.Error() != "sql: no rows in result set" {
						return nil, getErr
					}
				}

				return "bulk deletion verified", nil
			},
			assertion: func(result interface{}, err error) {
				assert.NoError(t, err)
			},
			cleanupFunc: func(block []blocks.Block) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			block, err := tt.setup()
			assert.NoError(t, err)
			result, err := tt.action(block)
			tt.assertion(result, err)
			if tt.cleanupFunc != nil {
				tt.cleanupFunc(block)
			}
		})
	}
}

// Test that creating a new block with a new location ID replaces the old location ID.
func TestBlockRepository_Create_NewLocationID(t *testing.T) {
	repo, _, cleanup := setupBlockRepo(t)
	defer cleanup()

	block, err := repo.Create(
		context.Background(),
		blocks.NewImageBlock(
			blocks.BaseBlock{
				LocationID: gofakeit.UUID(),
				Type:       "image",
				Points:     10,
			},
		),
		gofakeit.UUID(),
		blocks.ContextLocationContent,
	)

	assert.NoError(t, err)
	assert.NotNil(t, block)

	// Create a new block with a new location ID
	newBlock, err := repo.Create(
		context.Background(),
		block,
		gofakeit.UUID(),
		blocks.ContextLocationContent,
	)

	assert.NoError(t, err)
	assert.NotNil(t, newBlock)
	assert.NotEqual(t, block.GetLocationID(), newBlock.GetLocationID())
}
