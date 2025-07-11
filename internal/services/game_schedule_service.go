package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nathanhollows/Rapua/v3/db"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
	"github.com/uptrace/bun"
)

type GameScheduleService struct {
	transactor   db.Transactor
	instanceRepo repositories.InstanceRepository
}

func NewGameScheduleService(transactor db.Transactor, instanceRepo repositories.InstanceRepository) *GameScheduleService {
	return &GameScheduleService{
		transactor:   transactor,
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
		return errors.New("game is already active")
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
		return errors.New("game is already closed")
	}

	if !instance.StartTime.IsZero() && end.Before(instance.StartTime.Time) {
		return errors.New("end time cannot be before start time")
	}

	instance.EndTime = bun.NullTime{Time: end}
	if err := s.instanceRepo.Update(ctx, instance); err != nil {
		return fmt.Errorf("failed to update game end time: %w", err)
	}

	return nil
}

func (s *GameScheduleService) ScheduleGame(ctx context.Context, instance *models.Instance, start time.Time, endTime time.Time) error {
	if start.After(endTime) {
		return errors.New("start time cannot be after end time")
	}

	instance.StartTime = bun.NullTime{Time: start}
	instance.EndTime = bun.NullTime{Time: endTime}

	if err := s.instanceRepo.Update(ctx, instance); err != nil {
		return fmt.Errorf("failed to schedule game: %w", err)
	}

	return nil
}
