package repositories

import (
	"context"
	"strings"

	"github.com/nathanhollows/Rapua/v3/helpers"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/uptrace/bun"
)

type MarkerRepository interface {
	// Create a new marker in the database
	Create(ctx context.Context, marker *models.Marker) error

	// GetByCode finds a marker by its code
	GetByCode(ctx context.Context, code string) (*models.Marker, error)
	// FindNotInInstance finds markers that are not in an instance
	FindNotInInstance(ctx context.Context, instanceID string, otherInstances []string) ([]models.Marker, error)

	// Update updates a marker in the database
	Update(ctx context.Context, marker *models.Marker) error
	// UpdateCoords updates the latitude and longitude of a marker
	UpdateCoords(ctx context.Context, marker *models.Marker, lat, lng float64) error

	// Delete deletes a marker from the database
	Delete(ctx context.Context, code string) error

	// IsShared checks if a marker is used by more than one location
	IsShared(ctx context.Context, code string) (bool, error)
	// UserOwnsMarker checks if a user owns a marker
	UserOwnsMarker(ctx context.Context, userID, markerCode string) (bool, error)
}

type markerRepository struct {
	db *bun.DB
}

func NewMarkerRepository(db *bun.DB) MarkerRepository {
	return &markerRepository{
		db: db,
	}
}

// Create saves or updates a marker in the database.
func (r *markerRepository) Create(ctx context.Context, marker *models.Marker) error {
	if marker.Code == "" {
		// TODO: Remove magic number
		marker.Code = helpers.NewCode(5)
		_, err := r.db.NewInsert().Model(marker).Exec(ctx)
		return err
	}
	_, err := r.db.NewUpdate().Model(marker).WherePK("code").Exec(ctx)
	return err
}

// Update updates a marker in the database.
func (r *markerRepository) Update(ctx context.Context, marker *models.Marker) error {
	_, err := r.db.
		NewUpdate().
		Model(marker).
		Column("name", "lat", "lng", "total_visits", "current_count", "avg_duration").
		WherePK("code").
		Exec(ctx)

	return err
}

// Delete deletes a marker from the database.
func (r *markerRepository) Delete(ctx context.Context, markerCode string) error {
	_, err := r.db.NewDelete().Model(&models.Marker{Code: markerCode}).WherePK().Exec(ctx)
	return err
}

// GetByCode finds a marker by its code.
func (r *markerRepository) GetByCode(ctx context.Context, code string) (*models.Marker, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	var marker models.Marker
	err := r.db.NewSelect().Model(&marker).Where("code = ?", code).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &marker, nil
}

func (r *markerRepository) FindNotInInstance(ctx context.Context, instanceID string, otherInstances []string) ([]models.Marker, error) {
	var markers []models.Marker
	err := r.db.NewSelect().
		Model(&markers).
		Where("code IN (SELECT marker_id FROM locations WHERE instance_id IN (?))", bun.In(otherInstances)). // Markers used by otherInstances
		Where("code NOT IN (SELECT marker_id FROM locations WHERE instance_id = ?)", instanceID).            // Exclude markers used by instanceID
		Order("name ASC").
		Scan(ctx)
	return markers, err
}

// UpdateCoords updates the latitude and longitude of a marker in the database.
func (r *markerRepository) UpdateCoords(ctx context.Context, marker *models.Marker, lat, lng float64) error {
	marker.Lat = lat
	marker.Lng = lng
	_, err := r.db.NewUpdate().Model(marker).WherePK().Column("lat", "lng").Exec(ctx)
	return err
}

// IsShared checks if a marker is shared.
func (r *markerRepository) IsShared(ctx context.Context, code string) (bool, error) {
	var count int
	count, err := r.db.NewSelect().Model(&models.Location{}).Where("marker_id = ?", code).Count(ctx)
	if err != nil {
		return false, err
	}
	return count > 1, nil
}

// UserOwnsMarker checks if a user owns a marker.
func (r *markerRepository) UserOwnsMarker(ctx context.Context, userID, markerCode string) (bool, error) {
	var count int
	count, err := r.db.
		NewSelect().
		Model(&models.Location{}).
		Where("marker_id = ? AND user_id = ?", markerCode, userID).
		Count(ctx)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
