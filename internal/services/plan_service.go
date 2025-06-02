package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/nathanhollows/Rapua/v3/internal/repositories"
	"github.com/nathanhollows/Rapua/v3/services"
)

const (
	TierTrial    = "Trial"
	TierBasic    = "Basic"
	TierPro      = "Pro"
	TierEducator = "Educator"
)

type planService struct {
	billingRepository BillingRepository
	userRepository    repositories.UserRepository
}

// Define BillingRepository interface here to avoid circular import
type BillingRepository interface {
	// Retrieve billing information for the user
	GetPlanStatus(ctx context.Context, userID string) (*services.PlanStatus, error)
}

// NewPlanService creates a new billing service
func NewPlanService() services.PlanService {
	return &planService{
		nil, // We'll replace this with a proper implementation
		repositories.NewUserRepository(),
	}
}

// CheckPlanLimits checks the plan limits for a team
func (s *planService) CheckPlanLimits(ctx context.Context, userID string) error {
	user, err := s.userRepository.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}

	// Event Boost gives users a temporary increase in their plan limits
	if time.Now().After(user.EventBoostExpiry.Time) {
		// Reset the event boost
		user.EventBoostExpiry = sql.NullTime{}
		s.userRepository.ResetEventBoost(ctx, userID)
	}

	// If the user has an event boost, they can play an unlimited number of games
	if user.EventBoostExpiry.Valid {
		return nil
	}

	// Even though the tiers are also limited by the number of players per game,
	// the billing service only checks the number of games played this month.
	// The number of players per game is checked by the game manager service.
	switch user.Tier {
	case TierTrial:
		if user.GamesThisMonth >= 3 {
			return fmt.Errorf("plan limit reached")
		}
	case TierBasic:
		if user.GamesThisMonth >= 10 {
			return fmt.Errorf("plan limit reached")
		}
	case TierPro, TierEducator:
		if user.GamesThisMonth >= 20 {
			return fmt.Errorf("plan limit reached")
		}
	}

	return nil
}

// SubscribeUser subscribes a user to a plan
func (s *planService) SubscribeUser(ctx context.Context, userID string, tier string) error {
	_, err := s.userRepository.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to find user: %w", err)
	}

	return nil

}

// UpgradePlan upgrades the plan for a team
func (s *planService) UpgradePlan(ctx context.Context, userID string, newTier string) error {
	return nil
}

// ActivateEventBoost activates an event boost for a team
func (s *planService) ActivateEventBoost(ctx context.Context, userID string, duration time.Duration) error {
	return nil
}

// TrackUsage tracks the usage of a team's instance
func (s *planService) TrackUsage(ctx context.Context, userID string, instanceID string, numPlayers int) error {
	return nil
}

// RequiresPlanSelection checks if a user needs to select a plan
func (s *planService) RequiresPlanSelection(ctx context.Context, userID string) (bool, error) {
	status, err := s.GetPlanStatus(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("getting plan status: %w", err)
	}

	if status.Tier == "" {
		return true, nil
	}

	if (status.PlanExpiryDate.IsZero() || time.Now().After(status.PlanExpiryDate)) &&
		(status.EventBoostExpiry.IsZero() || time.Now().After(status.EventBoostExpiry)) {
		return true, nil
	}

	return false, nil
}

// GetPlanStatus retrieves the plan status for a user
func (s *planService) GetPlanStatus(ctx context.Context, userID string) (*services.PlanStatus, error) {
	return s.billingRepository.GetPlanStatus(ctx, userID)
}
