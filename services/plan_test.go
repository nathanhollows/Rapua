package services_test

import (
	"context"
	"testing"

	"github.com/nathanhollows/Rapua/v3/services"
	"github.com/stretchr/testify/assert"
)

// TestNoOpBillingService tests that NoOpBillingService does nothing whatsoever.
func TestNoOpBillingService(t *testing.T) {
	t.Parallel()

	service := services.NewNoOpBillingService()
	ctx := context.Background()

	// Test: NoOpBillingService should always return nil.
	if service == nil {
		t.Fatal("expected non-nil billing service")
	}

	// Test: CheckPlanLimits should always return nil.
	err := service.CheckPlanLimits(ctx, "team-1")
	assert.NoError(t, err, "expected no error when checking plan limits")

	// Test: UpgradePlan should always return nil.
	err = service.UpgradePlan(ctx, "team-1", "tier-1")
	assert.NoError(t, err, "expected no error when upgrading plan")

	// Test: SubscribeUser should always return nil.
	err = service.SubscribeUser(ctx, "team-1", "tier-1")
	assert.NoError(t, err, "expected no error when subscribing user")

	// Test: ActivateEventBoost should always return nil.
	err = service.ActivateEventBoost(ctx, "team-1", 0)
	assert.NoError(t, err, "expected no error when activating event boost")

	// Test: TrackUsage should always return nil.
	err = service.TrackUsage(ctx, "team-1", "instance-1", 10)
	assert.NoError(t, err, "expected no error when tracking usage")

	// Test: RequiresPlanSelection should always return false.
	requiresSelection, err := service.RequiresPlanSelection(ctx, "team-1")
	assert.NoError(t, err, "expected no error when checking plan selection")
	assert.False(t, requiresSelection, "expected no plan selection required")

	// Test: GetPlanStatus should always return a non-nil BillingStatus.
	status, err := service.GetPlanStatus(ctx, "team-1")
	assert.NoError(t, err, "expected no error when getting plan status")
	if status == nil {
		t.Fatal("expected non-nil plan status")
	}
	assert.Equal(t, "", string(status.Tier), "expected empty tier")
	assert.True(t, status.PlanExpiryDate.IsZero(), "expected zero plan expiry date")
	assert.True(t, status.EventBoostExpiry.IsZero(), "expected zero event boost expiry")

}
