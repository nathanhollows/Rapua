// Package services provides entity deletion with transaction safety.
package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/nathanhollows/Rapua/v4/db"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/uptrace/bun"
)

// DeleteService coordinates cascading deletions across related entities.
type DeleteService struct {
	transactor           db.Transactor
	blockRepo            repositories.BlockRepository
	blockStateRepo       repositories.BlockStateRepository
	checkInRepo          repositories.CheckInRepository
	instanceRepo         repositories.InstanceRepository
	instanceSettingsRepo repositories.InstanceSettingsRepository
	locationRepo         repositories.LocationRepository
	markerRepo           repositories.MarkerRepository
	teamRepo             repositories.TeamRepository
	userRepo             repositories.UserRepository
}

// NewDeleteService creates a new DeleteService with the provided dependencies.
func NewDeleteService(
	transactor db.Transactor,
	blockRepo repositories.BlockRepository,
	blockStateRepo repositories.BlockStateRepository,
	checkInRepo repositories.CheckInRepository,
	instanceRepo repositories.InstanceRepository,
	instanceSettingsRepo repositories.InstanceSettingsRepository,
	locationRepo repositories.LocationRepository,
	markerRepo repositories.MarkerRepository,
	teamRepo repositories.TeamRepository,
	userRepo repositories.UserRepository,
) *DeleteService {
	return &DeleteService{
		transactor:           transactor,
		blockRepo:            blockRepo,
		blockStateRepo:       blockStateRepo,
		checkInRepo:          checkInRepo,
		instanceRepo:         instanceRepo,
		instanceSettingsRepo: instanceSettingsRepo,
		locationRepo:         locationRepo,
		markerRepo:           markerRepo,
		teamRepo:             teamRepo,
		userRepo:             userRepo,
	}
}

// DeleteUser deletes a user and all associated instances, teams, and progress.
func (s *DeleteService) DeleteUser(ctx context.Context, userID string) error {
	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	// Ensure rollback on failure
	defer func() {
		if p := recover(); p != nil {
			err := tx.Rollback()
			if err != nil {
				fmt.Println("failed to rollback transaction:", err)
			}
			panic(p)
		}
	}()

	err = s.deleteUser(ctx, tx, userID)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("deleting user: %w; rollback failed: %w", err, rollbackErr)
		}
		return fmt.Errorf("deleting user: %w", err)
	}

	return tx.Commit()
}

// DeleteBlock deletes a block and its associated player progress.
func (s *DeleteService) DeleteBlock(ctx context.Context, blockID string) error {
	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	// Ensure rollback on failure
	defer func() {
		if p := recover(); p != nil {
			err := tx.Rollback()
			log.Printf("recovered from panic, rolling back transaction: %v", err)
			panic(p)
		}
	}()

	err = s.deleteBlock(ctx, tx, blockID)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("deleting block: %w; rollback failed: %w", err, rollbackErr)
		}
		return fmt.Errorf("deleting block: %w", err)
	}

	return tx.Commit()
}

// DeleteInstance deletes an instance and all its content.
// Returns ErrUserNotAuthenticated if userID doesn't own the instance.
func (s *DeleteService) DeleteInstance(ctx context.Context, userID, instanceID string) error {
	if userID == "" {
		return ErrUserNotAuthenticated
	}

	if instanceID == "" {
		return errors.New("instanceID cannot be empty")
	}

	// Check if the user has permission to delete the instance
	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("finding instance: %w", err)
	}

	if userID != instance.UserID {
		return ErrUserNotAuthenticated
	}

	// Start transaction
	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			err := tx.Rollback()
			if err != nil {
				panic(fmt.Errorf("rolling back transaction: %w", err))
			}
			panic(p)
		}
	}()

	err = s.deleteInstance(ctx, tx, instanceID)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("deleting instance: %w; rollback failed: %w", err, rollbackErr)
		}
		return fmt.Errorf("deleting instance: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// DeleteLocation deletes a location and all associated blocks and progress.
func (s *DeleteService) DeleteLocation(ctx context.Context, locationID string) error {
	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				panic(fmt.Errorf("rolling back transaction: %v; %w", p, rollbackErr))
			}
			panic(p)
		}
	}()

	err = s.deleteLocation(ctx, tx, locationID)
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("rolling back transaction: %w; %w", err, rollbackErr)
		}
		return fmt.Errorf("deleting location: %w", err)
	}

	err = s.markerRepo.DeleteUnused(ctx, tx)
	if err != nil {
		return fmt.Errorf("deleting unused markers: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("rolling back transaction: %w; %w", err, rollbackErr)
		}
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// deleteLocation deletes a location and its related data.
func (s *DeleteService) deleteLocation(ctx context.Context, tx *bun.Tx, locationID string) error {
	// Delete all blocks and their states for this location
	err := s.deleteBlocksByLocationID(ctx, tx, locationID)
	if err != nil {
		return fmt.Errorf("deleting blocks: %w", err)
	}

	// Delete the location
	err = s.locationRepo.Delete(ctx, tx, locationID)
	if err != nil {
		return fmt.Errorf("deleting location: %w", err)
	}

	return nil
}

// ResetTeams clears team progress while preserving the teams themselves.
func (s *DeleteService) ResetTeams(ctx context.Context, instanceID string, teamCodes []string) error {
	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("failed to rollback transaction: %w", rollbackErr)
		}
		return fmt.Errorf("starting transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				fmt.Printf("rolling back transaction: %v\n", rollbackErr)
			}
			panic(p)
		}
	}()

	err = s.teamRepo.Reset(ctx, tx, instanceID, teamCodes)
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("resetting team: rollback failed: %w", rollbackErr)
		}
		return fmt.Errorf("resetting team: %w", err)
	}

	err = s.checkInRepo.DeleteByTeamCodes(ctx, tx, instanceID, teamCodes)
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("rolling back transaction: %w", rollbackErr)
		}
		return fmt.Errorf("deleting check ins: %w", err)
	}

	err = s.blockStateRepo.DeleteByTeamCodes(ctx, tx, teamCodes)
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("rolling back transaction: %w", rollbackErr)
		}
		return fmt.Errorf("deleting block states: %w", err)
	}

	err = s.locationRepo.UpdateStatistics(ctx, tx, instanceID)
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("rolling back transaction: %w", rollbackErr)
		}
		return fmt.Errorf("updating location statistics: %w", err)
	}

	return tx.Commit()
}

// DeleteTeams deletes teams and their associated progress data.
func (s *DeleteService) DeleteTeams(ctx context.Context, instanceID string, teamCodes []string) error {
	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			rollbackErr := tx.Rollback()
			if rollbackErr != nil {
				fmt.Printf("rolling back transaction: %v\n", rollbackErr)
			}
			panic(p)
		}
	}()

	err = s.deleteTeams(ctx, tx, instanceID, teamCodes)
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return fmt.Errorf("deleting teams: %w; rollback failed: %w", err, rollbackErr)
		}
		return fmt.Errorf("deleting teams: %w", err)
	}

	return tx.Commit()
}

// deleteTeamsByInstanceID removes all teams and related data for a specific instance.
func (s *DeleteService) deleteTeamsByInstanceID(ctx context.Context, tx *bun.Tx, instanceID string) error {
	// Get all teams for this instance to delete their related data
	teams, err := s.teamRepo.FindAll(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("finding teams for instance: %w", err)
	}

	// Extract team codes
	teamCodes := make([]string, len(teams))
	for i, team := range teams {
		teamCodes[i] = team.Code
	}

	// Delete check-ins for all teams in this instance
	if len(teamCodes) > 0 {
		err = s.checkInRepo.DeleteByTeamCodes(ctx, tx, instanceID, teamCodes)
		if err != nil {
			return fmt.Errorf("deleting check ins: %w", err)
		}

		// Delete block states for all teams in this instance
		err = s.blockStateRepo.DeleteByTeamCodes(ctx, tx, teamCodes)
		if err != nil {
			return fmt.Errorf("deleting block states: %w", err)
		}
	}

	// Delete all teams for this instance
	err = s.teamRepo.DeleteByInstanceID(ctx, tx, instanceID)
	if err != nil {
		return fmt.Errorf("deleting teams by instance ID: %w", err)
	}

	return nil
}

// deleteTeams deletes specific teams by their codes.
func (s *DeleteService) deleteTeams(ctx context.Context, tx *bun.Tx, instanceID string, teamCodes []string) error {
	// Delete teams one by one (no bulk delete method available)
	for _, teamCode := range teamCodes {
		err := s.teamRepo.Delete(ctx, tx, instanceID, teamCode)
		if err != nil {
			return fmt.Errorf("deleting team %s: %w", teamCode, err)
		}
	}

	// Delete check-ins for these teams
	err := s.checkInRepo.DeleteByTeamCodes(ctx, tx, instanceID, teamCodes)
	if err != nil {
		return fmt.Errorf("deleting check ins: %w", err)
	}

	// Delete block states for these teams
	err = s.blockStateRepo.DeleteByTeamCodes(ctx, tx, teamCodes)
	if err != nil {
		return fmt.Errorf("deleting block states: %w", err)
	}

	// Update location statistics
	err = s.locationRepo.UpdateStatistics(ctx, tx, instanceID)
	if err != nil {
		return fmt.Errorf("updating location statistics: %w", err)
	}

	return nil
}

// deleteBlocksByLocationID deletes all blocks for a location.
func (s *DeleteService) deleteBlocksByLocationID(ctx context.Context, tx *bun.Tx, locationID string) error {
	// Delete all blocks (block states should cascade delete via database constraints)
	err := s.blockRepo.DeleteByLocationID(ctx, tx, locationID)
	if err != nil {
		return fmt.Errorf("deleting blocks: %w", err)
	}

	return nil
}

// deleteUser deletes a user and all their instances.
func (s *DeleteService) deleteUser(ctx context.Context, tx *bun.Tx, userID string) error {
	// Get all instances for this user to properly cascade delete
	instances, err := s.instanceRepo.FindByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("finding user instances: %w", err)
	}

	// Delete each instance properly (this will cascade to locations, teams, etc.)
	for _, instance := range instances {
		err = s.deleteInstance(ctx, tx, instance.ID)
		if err != nil {
			return fmt.Errorf("deleting instance %s: %w", instance.ID, err)
		}
	}

	// Delete the user
	err = s.userRepo.Delete(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}

	return nil
}

// deleteBlock deletes a block and its states.
func (s *DeleteService) deleteBlock(ctx context.Context, tx *bun.Tx, blockID string) error {
	// Delete block states first
	err := s.blockStateRepo.DeleteByBlockID(ctx, tx, blockID)
	if err != nil {
		return fmt.Errorf("deleting block states: %w", err)
	}

	// Delete the block
	err = s.blockRepo.Delete(ctx, tx, blockID)
	if err != nil {
		return fmt.Errorf("deleting block: %w", err)
	}

	return nil
}

// deleteInstance deletes an instance and all related data.
func (s *DeleteService) deleteInstance(ctx context.Context, tx *bun.Tx, instanceID string) error {
	// Get instance to access its locations
	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("finding instance: %w", err)
	}

	// Delete all teams for this instance
	err = s.deleteTeamsByInstanceID(ctx, tx, instanceID)
	if err != nil {
		return fmt.Errorf("deleting teams: %w", err)
	}

	// Delete all locations for this instance
	for _, location := range instance.Locations {
		err = s.deleteLocation(ctx, tx, location.ID)
		if err != nil {
			return fmt.Errorf("deleting location %s: %w", location.ID, err)
		}
	}

	// Delete instance settings
	err = s.instanceSettingsRepo.Delete(ctx, tx, instanceID)
	if err != nil {
		return fmt.Errorf("deleting instance settings: %w", err)
	}

	// Delete the instance
	err = s.instanceRepo.Delete(ctx, tx, instanceID)
	if err != nil {
		return fmt.Errorf("deleting instance: %w", err)
	}

	return nil
}
