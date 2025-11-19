// Package services provides entity deletion with transaction safety.
package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/nathanhollows/Rapua/v6/db"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
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
	creditRepo           *repositories.CreditRepository
	creditPurchaseRepo   *repositories.CreditPurchaseRepository
	teamStartLogRepo     *repositories.TeamStartLogRepository
	db                   *bun.DB
	uploadsDir           string
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
	creditRepo *repositories.CreditRepository,
	creditPurchaseRepo *repositories.CreditPurchaseRepository,
	teamStartLogRepo *repositories.TeamStartLogRepository,
	db *bun.DB,
	uploadsDir string,
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
		creditRepo:           creditRepo,
		creditPurchaseRepo:   creditPurchaseRepo,
		teamStartLogRepo:     teamStartLogRepo,
		db:                   db,
		uploadsDir:           uploadsDir,
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
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				slog.Error("transaction", "error", rollbackErr)
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
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				slog.Error("transaction", "error", rollbackErr)
			}
			panic(p)
		}
	}()

	// Extract image URL before deletion (if it's an image block)
	imageURL, err := s.extractImageURL(ctx, tx, blockID)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("extracting image URL: %w; rollback failed: %w", err, rollbackErr)
		}
		return fmt.Errorf("extracting image URL: %w", err)
	}

	err = s.deleteBlock(ctx, tx, blockID)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("deleting block: %w; rollback failed: %w", err, rollbackErr)
		}
		return fmt.Errorf("deleting block: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	// Only cleanup after transaction commits successfully
	if imageURL != "" {
		go s.cleanupOrphanedUpload(imageURL)
	}

	return nil
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
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				panic(fmt.Errorf("rolling back transaction: %w", rollbackErr))
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
				slog.Error("transaction", "error", rollbackErr)
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
				slog.Error("transaction", "error", rollbackErr)
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
	err := s.blockRepo.DeleteByOwnerID(ctx, tx, locationID)
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

	// Delete credit-related data
	err = s.teamStartLogRepo.DeleteByUserID(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("deleting team start logs: %w", err)
	}

	err = s.creditPurchaseRepo.DeleteByUserID(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("deleting credit purchases: %w", err)
	}

	err = s.creditRepo.DeleteCreditAdjustmentsByUserID(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("deleting credit adjustments: %w", err)
	}

	// Delete the user
	err = s.userRepo.Delete(ctx, tx, userID)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}

	return nil
}

// deleteBlock deletes a block and its states.
// Note: Image URL extraction and cleanup should be handled by the caller (DeleteBlock).
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

// extractImageURL extracts the image URL from a block if it's an image block.
// Returns empty string if block is not an image block or doesn't have a URL.
func (s *DeleteService) extractImageURL(ctx context.Context, tx *bun.Tx, blockID string) (string, error) {
	// Fetch the block to check its type and data
	var modelBlock models.Block
	err := tx.NewSelect().
		Model(&modelBlock).
		Where("id = ?", blockID).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil // Block doesn't exist, nothing to extract
		}
		return "", fmt.Errorf("fetching block: %w", err)
	}

	// Only process image blocks
	if modelBlock.Type != "image" {
		return "", nil
	}

	// Parse the JSON data to extract the URL
	var imageData struct {
		URL string `json:"content"`
	}
	err = json.Unmarshal(modelBlock.Data, &imageData)
	if err != nil {
		slog.Warn("failed to parse image block data", "blockID", blockID, "error", err)
		return "", nil // Don't fail deletion if we can't parse
	}

	return imageData.URL, nil
}

// escapeLikePattern escapes special characters in LIKE patterns to prevent unintended matches.
func escapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

// isUploadedFile checks if a URL is a local uploaded file, not an external URL.
// Returns true for:
// - /static/uploads/... (relative path)
// - http://localhost:8090/static/uploads/... (matches SITE_URL)
// - https://yourdomain.com/static/uploads/... (matches SITE_URL)
// Returns false for external URLs like https://example.com/image.png
func isUploadedFile(url string) bool {
	// Check for relative upload path
	if strings.HasPrefix(url, "/static/uploads/") {
		return true
	}

	// Check for absolute URL matching site domain
	if strings.Contains(url, "/static/uploads/") {
		// Extract domain from URL if present
		if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
			// Get site URL from environment (with fallback)
			siteURL := os.Getenv("SITE_URL")
			if siteURL == "" {
				siteURL = "http://localhost:8090" // Default fallback
			}

			// Check if URL starts with site domain
			if strings.HasPrefix(url, siteURL+"/static/uploads/") {
				return true
			}
		}
	}

	return false
}

// isFileReferencedInBlocks checks if a URL is referenced in any block's data.
// Returns true if the filename appears in any block (uses same logic as orphaned uploads cleanup).
func (s *DeleteService) isFileReferencedInBlocks(ctx context.Context, urlPath string) (bool, error) {
	// Extract just the filename for flexible search
	filename := filepath.Base(urlPath)

	// Escape LIKE special characters to prevent unintended matches
	escapedFilename := escapeLikePattern(filename)

	// Search for the filename in the blocks data (JSON field)
	count, err := s.db.NewSelect().
		Model((*models.Block)(nil)).
		Where("data LIKE ? ESCAPE '\\'", "%"+escapedFilename+"%").
		Count(ctx)

	if err != nil {
		return false, fmt.Errorf("querying blocks: %w", err)
	}

	return count > 0, nil
}

// cleanupOrphanedUpload deletes a file from the filesystem if it's not referenced by any blocks.
// This runs in a goroutine with background context, so errors are only logged.
func (s *DeleteService) cleanupOrphanedUpload(url string) {
	ctx := context.Background()

	// Skip external URLs - only clean up local uploads
	if !isUploadedFile(url) {
		slog.Debug("skipping external URL", "url", url)
		return
	}

	// Check if the upload is still referenced by other blocks
	isReferenced, err := s.isFileReferencedInBlocks(ctx, url)
	if err != nil {
		slog.Warn("failed to check upload references", "url", url, "error", err)
		return
	}

	if isReferenced {
		// Upload is still used by other blocks, don't delete
		slog.Debug("upload still referenced, keeping", "url", url)
		return
	}

	// Try direct path construction first for performance
	// Expected format: /static/uploads/YYYY/MM/DD/filename.ext
	filename := filepath.Base(url)
	parts := strings.Split(strings.Trim(url, "/"), "/")

	var filePath string
	if len(parts) >= 6 && parts[0] == "static" && parts[1] == "uploads" {
		// Extract date path: YYYY/MM/DD
		datePath := filepath.Join(parts[2], parts[3], parts[4])
		filePath = filepath.Join(s.uploadsDir, datePath, filename)

		// Verify file exists at expected location
		if _, statErr := os.Stat(filePath); statErr == nil {
			// File exists, delete it
			if removeErr := os.Remove(filePath); removeErr != nil {
				slog.Warn("failed to delete orphaned upload", "path", filePath, "error", removeErr)
			} else {
				slog.Info("deleted orphaned upload", "path", filePath, "url", url)
			}
			return
		}
	}

	// Fallback: Walk the uploads directory to find the file
	// (Only needed if URL format is unexpected or file moved)
	err = filepath.Walk(s.uploadsDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			//nolint:nilerr // Continue walking on errors, intentionally ignore per-file errors
			return nil
		}
		if !info.IsDir() && filepath.Base(path) == filename {
			filePath = path
			return filepath.SkipAll // Found it, stop walking
		}
		return nil
	})

	if err != nil {
		slog.Warn("error walking uploads directory", "error", err)
		return
	}

	if filePath == "" {
		// File doesn't exist on filesystem, nothing to clean up
		slog.Debug("upload file not found on filesystem", "url", url)
		return
	}

	// Delete the file
	err = os.Remove(filePath)
	if err != nil {
		slog.Warn("failed to delete orphaned upload", "path", filePath, "error", err)
		return
	}

	slog.Info("deleted orphaned upload", "path", filePath, "url", url)
}
