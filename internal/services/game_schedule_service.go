package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
	"github.com/uptrace/bun"
)

// Game scheduling errors.
var (
	ErrGameAlreadyActive = errors.New("game is already active")
	ErrGameAlreadyClosed = errors.New("game is already closed")
	ErrInvalidTimeRange  = errors.New("end time cannot be before start time")
	ErrStartAfterEnd     = errors.New("start time cannot be after end time")
)

type GameScheduleService struct {
	instanceRepo repositories.InstanceRepository
}

func NewGameScheduleService(instanceRepo repositories.InstanceRepository) *GameScheduleService {
	return &GameScheduleService{
		instanceRepo: instanceRepo,
	}
}

func (s *GameScheduleService) Start(ctx context.Context, instance *models.Instance) error {
	return s.SetStartTime(ctx, instance, time.Now())
}

func (s *GameScheduleService) Stop(ctx context.Context, instance *models.Instance) error {
	return s.SetEndTime(ctx, instance, time.Now())
}

func (s *GameScheduleService) SetStartTime(ctx context.Context, instance *models.Instance, start time.Time) error {
	if instance.GetStatus() == models.Active {
		return ErrGameAlreadyActive
	}

	instance.StartTime = bun.NullTime{Time: start}
	if instance.EndTime.Before(start) {
		instance.EndTime = bun.NullTime{}
	}

	if err := s.instanceRepo.Update(ctx, instance); err != nil {
		return fmt.Errorf("failed to update game start time: %w", err)
	}

	return nil
}

func (s *GameScheduleService) SetEndTime(ctx context.Context, instance *models.Instance, end time.Time) error {
	if instance.GetStatus() == models.Closed {
		return ErrGameAlreadyClosed
	}

	if !instance.StartTime.IsZero() && end.Before(instance.StartTime.Time) {
		return ErrInvalidTimeRange
	}

	instance.EndTime = bun.NullTime{Time: end}
	if err := s.instanceRepo.Update(ctx, instance); err != nil {
		return fmt.Errorf("failed to update game end time: %w", err)
	}

	return nil
}

func (s *GameScheduleService) ScheduleGame(
	ctx context.Context,
	instance *models.Instance,
	start time.Time,
	endTime time.Time,
) error {
	if !endTime.IsZero() && start.After(endTime) {
		return ErrStartAfterEnd
	}

	instance.StartTime = bun.NullTime{Time: start}
	instance.EndTime = bun.NullTime{Time: endTime}

	if err := s.instanceRepo.Update(ctx, instance); err != nil {
		return fmt.Errorf("failed to schedule game: %w", err)
	}

	return nil
}
