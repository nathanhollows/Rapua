package services

import (
	"context"

	"github.com/nathanhollows/Rapua/v8/internal/repositories"
)

type QuickstartService struct {
	instanceRepo repositories.QuestRepository
}

func NewQuickstartService(instanceRepo repositories.QuestRepository) *QuickstartService {
	return &QuickstartService{
		instanceRepo: instanceRepo,
	}
}

// DismissQuickstart marks the quickstart as dismissed for the given instance.
func (s *QuickstartService) DismissQuickstart(ctx context.Context, questID string) error {
	return s.instanceRepo.DismissQuickstart(ctx, questID)
}
