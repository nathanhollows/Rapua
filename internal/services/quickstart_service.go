package services

import (
	"context"

	"github.com/nathanhollows/Rapua/v3/repositories"
)

type QuickstartService struct {
	instanceRepo repositories.InstanceRepository
}

func NewQuickstartService(instanceRepo repositories.InstanceRepository) *QuickstartService {
	return &QuickstartService{
		instanceRepo: instanceRepo,
	}
}

// DismissQuickstart marks the quickstart as dismissed for the given instance.
func (s *QuickstartService) DismissQuickstart(ctx context.Context, instanceID string) error {
	return s.instanceRepo.DismissQuickstart(ctx, instanceID)
}
