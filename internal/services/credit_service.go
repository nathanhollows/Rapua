package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v4/db"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/uptrace/bun"
)

type CreditRepository interface {
	// CreateCreditAdjustmentWithTx saves a credit adjustment record.
	CreateCreditAdjustmentWithTx(ctx context.Context, tx *bun.Tx, adjustment *models.CreditAdjustments) error

	// GetCreditAdjustmentsByUserID returns all credit adjustments for a user.
	GetCreditAdjustmentsByUserID(ctx context.Context, userID string) ([]models.CreditAdjustments, error)
	// GetCreditAdjustmentsByUserIDWithPagination returns credit adjustments for a user with pagination.
	GetCreditAdjustmentsByUserIDWithPagination(
		ctx context.Context,
		userID string,
		limit, offset int,
	) ([]models.CreditAdjustments, error)

	// AddCreditsWithTx atomically increments credits without read-modify-write to prevent lost updates.
	AddCreditsWithTx(ctx context.Context, tx *bun.Tx, userID string, freeCreditsToAdd int, paidCreditsToAdd int) error

	// TryDeductOneCredit atomically deducts one credit from free first, then paid. Returns ErrInsufficientCredits if not possible.
	DeductOneCreditWithTx(ctx context.Context, tx *bun.Tx, userID string) error
}

const (
	// daysInWeek is the number of days in a week.
	daysInWeek = 7
)

type CreditService struct {
	transactor       db.Transactor
	creditRepo       CreditRepository
	teamStartLogRepo *repositories.TeamStartLogRepository
	userRepo         repositories.UserRepository
}

func NewCreditService(
	transactor db.Transactor,
	creditRepo CreditRepository,
	teamStartLogRepo *repositories.TeamStartLogRepository,
	userRepo repositories.UserRepository,
) *CreditService {
	return &CreditService{
		transactor:       transactor,
		creditRepo:       creditRepo,
		teamStartLogRepo: teamStartLogRepo,
		userRepo:         userRepo,
	}
}

// GetCreditBalance retrieves the credit balance for a user.
func (s *CreditService) GetCreditBalance(
	ctx context.Context,
	userID string,
) (freeCredits int, paidCredits int, err error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	return user.FreeCredits, user.PaidCredits, nil
}

// AddCredits adds credits to a user's account with a reason for the addition.
func (s *CreditService) AddCredits(
	ctx context.Context,
	userID string,
	freeCredits, paidCredits int,
	reason string,
) error {
	if reason == "" {
		return errors.New("reason is required")
	}
	if freeCredits > 0 && paidCredits > 0 {
		return errors.New("cannot add both free and paid credits at the same time")
	}
	if freeCredits < 0 || paidCredits < 0 {
		return errors.New("credits to add must be greater than zero")
	}
	if freeCredits == 0 && paidCredits == 0 {
		return errors.New("must add at least one credit")
	}

	// Save the amount being added
	creditsAdded := freeCredits + paidCredits

	// Start a transaction to ensure atomicity
	tx, err := s.transactor.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Use atomic UPDATE to avoid lost updates from concurrent operations
	// This increments the credits directly in the database without a read-modify-write cycle
	err = s.creditRepo.AddCreditsWithTx(ctx, tx, userID, freeCredits, paidCredits)
	if err != nil {
		return err
	}

	// Create a credit adjustment record
	adjustment := &models.CreditAdjustments{
		ID:        uuid.New().String(),
		CreatedAt: time.Now(),
		UserID:    userID,
		Credits:   creditsAdded,
		Reason:    reason,
	}
	err = s.creditRepo.CreateCreditAdjustmentWithTx(ctx, tx, adjustment)
	if err != nil {
		txErr := tx.Rollback()
		if txErr != nil {
			return txErr
		}
		return err
	}
	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		txErr := tx.Rollback()
		if txErr != nil {
			return txErr
		}
		return err
	}
	return nil
}

// DeductCreditForTeamStartWithTx handles credit deduction and team start logging within a transaction.
func (s *CreditService) DeductCreditForTeamStartWithTx(
	ctx context.Context,
	tx *bun.Tx,
	userID, teamID, instanceID string,
) error {
	// Step 1: Atomically deduct one credit (free first, then paid)
	err := s.creditRepo.DeductOneCreditWithTx(ctx, tx, userID)
	if err != nil {
		// Convert repository error to service error for consistency
		if err.Error() == "insufficient credits to start team" {
			return ErrInsufficientCredits
		}
		return err
	}

	// Step 2: Log the team start
	log := &models.TeamStartLog{
		ID:         uuid.New().String(),
		CreatedAt:  time.Now(),
		UserID:     userID,
		TeamID:     teamID,
		InstanceID: instanceID,
	}
	return s.teamStartLogRepo.CreateWithTx(ctx, tx, log)
}

// TeamStartLogFilter defines filtering options for team start logs.
type TeamStartLogFilter struct {
	UserID     string
	InstanceID string
	StartTime  time.Time
	EndTime    time.Time
	GroupBy    string
}

// CreditAdjustmentFilter defines filtering options for credit adjustments.
type CreditAdjustmentFilter struct {
	UserID string
	Limit  int
	Offset int
}

// GetCreditAdjustments returns credit adjustments based on filter criteria with pagination.
func (s *CreditService) GetCreditAdjustments(
	ctx context.Context,
	filter CreditAdjustmentFilter,
) ([]models.CreditAdjustments, error) {
	// Set default limit if not specified
	limit := filter.Limit
	if limit <= 0 {
		limit = 25 // Default page size
	}

	// Use pagination if offset is specified, otherwise get all
	if filter.Offset > 0 || filter.Limit > 0 {
		return s.creditRepo.GetCreditAdjustmentsByUserIDWithPagination(ctx, filter.UserID, limit, filter.Offset)
	}

	// Get all adjustments (no pagination)
	return s.creditRepo.GetCreditAdjustmentsByUserID(ctx, filter.UserID)
}

// TeamStartSummary represents aggregated team start data for a time period.
type TeamStartSummary struct {
	Date  time.Time `json:"date"`
	Count int       `json:"count"`
	Ratio float64   `json:"ratio,omitempty"` // Optional ratio for percentage calculations
}

// GetTeamStartLogsSummary returns team start logs aggregated by time periods with zero-filling.
func (s *CreditService) GetTeamStartLogsSummary(
	ctx context.Context,
	filter TeamStartLogFilter,
) ([]TeamStartSummary, error) {
	// Validate GroupBy parameter
	if filter.GroupBy == "" {
		filter.GroupBy = day // Default to daily grouping
	}
	if filter.GroupBy != day && filter.GroupBy != week && filter.GroupBy != month && filter.GroupBy != year {
		return nil, errors.New("groupBy must be one of: day, week, month, year")
	}

	// Get raw team start logs
	logs, err := s.getTeamStartLogsFiltered(ctx, filter)
	if err != nil {
		return nil, err
	}

	// If no time range specified, use a default range based on existing data
	startTime, endTime := s.determineTimeRange(filter, logs)

	// Group logs by time period
	groupedData := s.groupTeamStartLogs(logs, filter.GroupBy)

	// Fill in zero values for missing periods
	values := s.fillZeroValues(groupedData, startTime, endTime, filter.GroupBy)

	// Calculate ratios if needed
	values = s.calculateScaledRatios(values)

	return values, nil
}

// calculateScaledRatios scales the ratios based on the maximum count in the summary data.
func (s *CreditService) calculateScaledRatios(summaryData []TeamStartSummary) []TeamStartSummary {
	if len(summaryData) == 0 {
		return summaryData // No data to scale
	}

	maxCount := 0
	for _, item := range summaryData {
		if item.Count > maxCount {
			maxCount = item.Count
		}
	}

	if maxCount == 0 {
		// Avoid division by zero, return empty ratios
		for i := range summaryData {
			summaryData[i].Ratio = 0
		}
		return summaryData
	}

	for i := range summaryData {
		summaryData[i].Ratio = float64(summaryData[i].Count) / float64(maxCount)
	}

	return summaryData
}

// getTeamStartLogsFiltered gets team start logs based on filter criteria.
func (s *CreditService) getTeamStartLogsFiltered(
	ctx context.Context,
	filter TeamStartLogFilter,
) ([]models.TeamStartLog, error) {
	hasInstanceFilter := filter.InstanceID != ""
	hasTimeFilter := !filter.StartTime.IsZero() && !filter.EndTime.IsZero()

	switch {
	case hasInstanceFilter && hasTimeFilter:
		return s.teamStartLogRepo.GetByUserIDAndInstanceIDWithTimeframe(
			ctx,
			filter.UserID,
			filter.InstanceID,
			filter.StartTime,
			filter.EndTime,
		)
	case hasInstanceFilter:
		return s.teamStartLogRepo.GetByUserIDAndInstanceID(ctx, filter.UserID, filter.InstanceID)
	case hasTimeFilter:
		return s.teamStartLogRepo.GetByUserIDWithTimeframe(ctx, filter.UserID, filter.StartTime, filter.EndTime)
	default:
		return s.teamStartLogRepo.GetByUserID(ctx, filter.UserID)
	}
}

// determineTimeRange calculates appropriate start/end times if not provided.
func (s *CreditService) determineTimeRange(
	filter TeamStartLogFilter,
	logs []models.TeamStartLog,
) (time.Time, time.Time) {
	// If time range is specified, use it
	if !filter.StartTime.IsZero() && !filter.EndTime.IsZero() {
		return filter.StartTime, filter.EndTime
	}

	// If no logs, return a default range (last 30 days)
	if len(logs) == 0 {
		now := time.Now()
		return now.AddDate(0, 0, -30), now
	}

	// Use the range from first to last log with some padding
	earliest := logs[len(logs)-1].CreatedAt // logs are ordered DESC
	latest := logs[0].CreatedAt

	// Add padding based on groupBy
	switch filter.GroupBy {
	case year:
		return earliest.AddDate(-1, 0, 0), latest.AddDate(1, 0, 0)
	case month:
		return earliest.AddDate(0, -1, 0), latest.AddDate(0, 1, 0)
	case week:
		return earliest.AddDate(0, 0, -daysInWeek), latest.AddDate(0, 0, daysInWeek)
	default: // day
		return earliest.AddDate(0, 0, -1), latest.AddDate(0, 0, 1)
	}
}

// groupTeamStartLogs groups logs by time period.
func (s *CreditService) groupTeamStartLogs(logs []models.TeamStartLog, groupBy string) map[string]int {
	grouped := make(map[string]int)

	for _, log := range logs {
		key := s.formatDateKey(log.CreatedAt, groupBy)
		grouped[key]++
	}

	return grouped
}

// formatDateKey formats a date according to the groupBy parameter.
func (s *CreditService) formatDateKey(t time.Time, groupBy string) string {
	switch groupBy {
	case year:
		return t.Format("2006")
	case month:
		return t.Format("2006-01")
	case week:
		// Use Monday of the week as the key
		year, week := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	default: // day
		return t.Format("2006-01-02")
	}
}

// fillZeroValues creates a complete time series with zero values for missing periods.
func (s *CreditService) fillZeroValues(
	groupedData map[string]int,
	startTime, endTime time.Time,
	groupBy string,
) []TeamStartSummary {
	var result []TeamStartSummary

	current := s.truncateToGroupBy(startTime, groupBy)
	end := s.truncateToGroupBy(endTime, groupBy)

	for current.Before(end) || current.Equal(end) {
		key := s.formatDateKey(current, groupBy)
		count := groupedData[key] // Will be 0 if key doesn't exist

		result = append(result, TeamStartSummary{
			Date:  current,
			Count: count,
		})

		current = s.addPeriod(current, groupBy)
	}

	return result
}

// truncateToGroupBy truncates a time to the start of the specified period.
func (s *CreditService) truncateToGroupBy(t time.Time, groupBy string) time.Time {
	switch groupBy {
	case year:
		return time.Date(t.Year(), 1, 1, 0, 0, 0, 0, t.Location())
	case month:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	case week:
		// Go to Monday of the week
		weekday := int(t.Weekday())
		if weekday == 0 { // Sunday
			weekday = 7
		}
		monday := t.AddDate(0, 0, -weekday+1)
		return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, t.Location())
	default: // day
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	}
}

// addPeriod adds one period to a time based on groupBy.
func (s *CreditService) addPeriod(t time.Time, groupBy string) time.Time {
	switch groupBy {
	case year:
		return t.AddDate(1, 0, 0)
	case month:
		return t.AddDate(0, 1, 0)
	case week:
		return t.AddDate(0, 0, daysInWeek)
	default: // day
		return t.AddDate(0, 0, 1)
	}
}
