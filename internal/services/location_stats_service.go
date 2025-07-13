package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
)

type LocationStatsService struct {
	locationRepo repositories.LocationRepository
}

func NewLocationStatsService(locationRepo repositories.LocationRepository) *LocationStatsService {
	return &LocationStatsService{
		locationRepo: locationRepo,
	}
}

func (s *LocationStatsService) IncrementVisitors(ctx context.Context, location *models.Location) error {
	if location == nil {
		return errors.New("location cannot be nil")
	}

	// Increment the visitor count
	location.TotalVisits++
	location.CurrentCount++
	err := s.locationRepo.Update(ctx, location)
	if err != nil {
		return fmt.Errorf("error saving updated visitor stats: %w", err)
	}

	return nil
}

func (s *LocationStatsService) DecrementVisitors(ctx context.Context, location *models.Location) error {
	if location == nil {
		return errors.New("location cannot be nil")
	}

	// Decrement the visitor count
	if location.CurrentCount > 0 {
		location.CurrentCount--
	} else {
		return errors.New("current count cannot be negative")
	}

	err := s.locationRepo.Update(ctx, location)
	if err != nil {
		return fmt.Errorf("error saving updated visitor stats: %w", err)
	}

	return nil
}
