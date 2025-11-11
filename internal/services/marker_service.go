package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
)

var (
	ErrInvalidLatitude  = errors.New("latitude must be between -90 and 90")
	ErrInvalidLongitude = errors.New("longitude must be between -180 and 180")
)

type MarkerService struct {
	markerRepo repositories.MarkerRepository
}

func NewMarkerService(markerRepo repositories.MarkerRepository) *MarkerService {
	return &MarkerService{
		markerRepo: markerRepo,
	}
}

// CreateMarker creates a new marker for the given instance.
func (s *MarkerService) CreateMarker(ctx context.Context, name string, lat, lng float64) (models.Marker, error) {
	if name == "" {
		return models.Marker{}, errors.New("name cannot be empty")
	}
	if lat < -90 || lat > 90 {
		return models.Marker{}, ErrInvalidLatitude
	}
	if lng < -180 || lng > 180 {
		return models.Marker{}, ErrInvalidLongitude
	}
	marker := models.Marker{
		Name: name,
		Lat:  lat,
		Lng:  lng,
	}
	err := s.markerRepo.Create(ctx, &marker)
	if err != nil {
		return models.Marker{}, fmt.Errorf("failed to create marker: %w", err)
	}
	return marker, nil
}

func (s *MarkerService) GetMarkerByCode(ctx context.Context, locationCode string) (models.Marker, error) {
	locationCode = strings.TrimSpace(strings.ToUpper(locationCode))
	marker, err := s.markerRepo.GetByCode(ctx, locationCode)
	if err != nil {
		return models.Marker{}, fmt.Errorf("GetMarkerByCode: %w", err)
	}
	return *marker, nil
}

// FindMarkersNotInInstance finds all markers that are not in the given instance.
func (s *MarkerService) FindMarkersNotInInstance(
	ctx context.Context,
	instanceID string,
	otherInstances []string,
) ([]models.Marker, error) {
	if instanceID == "" {
		return nil, errors.New("instanceID cannot be empty")
	}

	if len(otherInstances) == 0 {
		return nil, errors.New("otherInstances cannot be empty")
	}

	markers, err := s.markerRepo.FindNotInInstance(ctx, instanceID, otherInstances)
	if err != nil {
		return nil, fmt.Errorf("finding markers not in instance: %w", err)
	}
	return markers, nil
}
