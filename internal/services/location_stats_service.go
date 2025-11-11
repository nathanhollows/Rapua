package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
)

type locationStatsService struct {
	locationRepo repositories.LocationRepository
}

func NewLocationStatsService(locationRepo repositories.LocationRepository) *locationStatsService {
	return &locationStatsService{
		locationRepo: locationRepo,
	}
}

func (s *locationStatsService) IncrementVisitors(ctx context.Context, location *models.Location) error {
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

func (s *locationStatsService) DecrementVisitors(ctx context.Context, location *models.Location) error {
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
