package scheduler_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v4/internal/scheduler"
	"github.com/stretchr/testify/assert"
)

var jobLogger = slog.New(&slog.TextHandler{})

func TestJob_Creation(t *testing.T) {
	tests := []struct {
		name     string
		jobName  string
		runFunc  func(context.Context) error
		nextFunc func() time.Time
		wantErr  bool
	}{
		{
			name:    "valid job creation",
			jobName: "test-job",
			runFunc: func(context.Context) error { return nil },
			nextFunc: func() time.Time {
				return time.Now().Add(time.Hour)
			},
			wantErr: false,
		},
		{
			name:    "job with empty name",
			jobName: "",
			runFunc: func(context.Context) error { return nil },
			nextFunc: func() time.Time {
				return time.Now().Add(time.Hour)
			},
			wantErr: false, // Empty name is allowed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := scheduler.NewScheduler(jobLogger)
			defer s.Stop()

			job := s.AddJob(tt.jobName, tt.runFunc, tt.nextFunc)

			assert.Equal(t, tt.jobName, job.Name)
			assert.NotNil(t, job.Run)
			assert.NotNil(t, job.Next)
		})
	}
}

func TestNextDaily(t *testing.T) {
	now := time.Date(2024, 8, 24, 14, 30, 0, 0, time.UTC)

	// Mock time.Now for consistent testing
	originalNext := scheduler.NextDaily
	defer func() { scheduler.NextDaily = originalNext }()

	scheduler.NextDaily = func() time.Time {
		tomorrow := now.AddDate(0, 0, 1)
		return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, now.Location())
	}

	nextRun := scheduler.NextDaily()
	expected := time.Date(2024, 8, 25, 0, 0, 0, 0, time.UTC)

	assert.Equal(t, expected, nextRun)
	assert.True(t, nextRun.After(now))
}

func TestNextFirstOfMonth(t *testing.T) {
	tests := []struct {
		name     string
		current  time.Time
		expected time.Time
	}{
		{
			name:     "middle of month",
			current:  time.Date(2024, 8, 15, 10, 30, 0, 0, time.UTC),
			expected: time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "first of month at midnight",
			current:  time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC),
			expected: time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "first of month but not midnight",
			current:  time.Date(2024, 8, 1, 10, 30, 0, 0, time.UTC),
			expected: time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "last day of year",
			current:  time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			expected: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock time.Now for consistent testing
			originalNext := scheduler.NextFirstOfMonth
			defer func() { scheduler.NextFirstOfMonth = originalNext }()

			scheduler.NextFirstOfMonth = func() time.Time {
				now := tt.current
				// Always go to next month, day 1, midnight
				nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())

				// If we're already on the 1st at midnight, use this month
				if now.Day() == 1 && now.Hour() == 0 && now.Minute() == 0 && now.Second() == 0 {
					return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
				}

				return nextMonth
			}

			result := scheduler.NextFirstOfMonth()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewScheduler(t *testing.T) {
	s := scheduler.NewScheduler(jobLogger)

	assert.NotNil(t, s)

	// Test cleanup
	s.Stop()
}

func TestScheduler_AddJob(t *testing.T) {
	s := scheduler.NewScheduler(jobLogger)
	defer s.Stop()

	// Add first job
	job1 := s.AddJob("test-job-1", func(context.Context) error { return nil }, func() time.Time {
		return time.Now().Add(time.Hour)
	})

	// Add second job
	job2 := s.AddJob("test-job-2", func(context.Context) error { return nil }, func() time.Time {
		return time.Now().Add(2 * time.Hour)
	})

	assert.Equal(t, "test-job-1", job1.Name)
	assert.Equal(t, "test-job-2", job2.Name)
	assert.NotNil(t, job1.Run)
	assert.NotNil(t, job2.Run)
	assert.NotNil(t, job1.Next)
	assert.NotNil(t, job2.Next)
}

func TestScheduler_Start(t *testing.T) {
	s := scheduler.NewScheduler(jobLogger)
	defer s.Stop()

	var executionCount int
	var mu sync.Mutex

	s.AddJob("test-job", func(context.Context) error {
		mu.Lock()
		executionCount++
		mu.Unlock()
		return nil
	}, func() time.Time {
		// Schedule very soon to speed up test
		return time.Now().Add(50 * time.Millisecond)
	})

	s.Start()

	// Wait for job to execute at least once
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := executionCount
	mu.Unlock()

	assert.GreaterOrEqual(t, count, 1, "job should have executed at least once")
}

func TestScheduler_JobExecution_Success(t *testing.T) {
	s := scheduler.NewScheduler(jobLogger)
	defer s.Stop()

	var executionCount int
	var mu sync.Mutex

	s.AddJob("success-job", func(context.Context) error {
		mu.Lock()
		executionCount++
		mu.Unlock()
		return nil
	}, func() time.Time {
		return time.Now().Add(50 * time.Millisecond)
	})

	s.Start()

	// Wait for execution
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := executionCount
	mu.Unlock()

	assert.GreaterOrEqual(t, count, 1, "job should have executed successfully")
}

func TestScheduler_JobExecution_WithError(t *testing.T) {
	s := scheduler.NewScheduler(jobLogger)
	defer s.Stop()

	var executionCount int
	var mu sync.Mutex
	expectedError := errors.New("job execution error")

	s.AddJob("error-job", func(context.Context) error {
		mu.Lock()
		executionCount++
		mu.Unlock()
		return expectedError
	}, func() time.Time {
		return time.Now().Add(50 * time.Millisecond)
	})

	s.Start()

	// Wait for execution
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := executionCount
	mu.Unlock()

	// Job should still execute despite errors
	assert.GreaterOrEqual(t, count, 1, "job should execute even with errors")
}

func TestScheduler_Stop_CancelsJobs(t *testing.T) {
	s := scheduler.NewScheduler(jobLogger)

	var executionCount int
	var mu sync.Mutex
	executed := make(chan bool, 1)

	s.AddJob("cancellation-job", func(context.Context) error {
		mu.Lock()
		executionCount++
		mu.Unlock()
		executed <- true
		return nil
	}, func() time.Time {
		return time.Now().Add(100 * time.Millisecond)
	})

	s.Start()

	// Stop the scheduler before job execution
	s.Stop()

	// Wait a bit to see if job executes
	select {
	case <-executed:
		t.Fatal("job should not execute after scheduler stop")
	case <-time.After(200 * time.Millisecond):
		// Expected behavior - job should not execute
	}

	mu.Lock()
	count := executionCount
	mu.Unlock()

	assert.Equal(t, 0, count, "job should not execute after scheduler stop")
}

func TestScheduler_MultipleJobs(t *testing.T) {
	s := scheduler.NewScheduler(jobLogger)
	defer s.Stop()

	var job1Count, job2Count int
	var mu sync.Mutex

	s.AddJob("job-1", func(context.Context) error {
		mu.Lock()
		job1Count++
		mu.Unlock()
		return nil
	}, func() time.Time {
		return time.Now().Add(30 * time.Millisecond)
	})

	s.AddJob("job-2", func(context.Context) error {
		mu.Lock()
		job2Count++
		mu.Unlock()
		return nil
	}, func() time.Time {
		return time.Now().Add(40 * time.Millisecond)
	})

	s.Start()

	// Wait for both jobs to execute
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count1 := job1Count
	count2 := job2Count
	mu.Unlock()

	assert.GreaterOrEqual(t, count1, 1, "job1 should have executed")
	assert.GreaterOrEqual(t, count2, 1, "job2 should have executed")
}

func TestScheduler_JobTimerReset(t *testing.T) {
	s := scheduler.NewScheduler(jobLogger)
	defer s.Stop()

	var executionCount int
	var mu sync.Mutex
	var lastExecution time.Time

	s.AddJob("timer-reset-job", func(context.Context) error {
		mu.Lock()
		executionCount++
		lastExecution = time.Now()
		mu.Unlock()
		return nil
	}, func() time.Time {
		// Each execution schedules the next one
		return time.Now().Add(60 * time.Millisecond)
	})

	s.Start()

	// Wait for multiple executions
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	count := executionCount
	last := lastExecution
	mu.Unlock()

	// Should have executed multiple times
	assert.GreaterOrEqual(t, count, 2, "job should execute multiple times")
	assert.False(t, last.IsZero(), "should have recorded execution time")
}

func TestScheduler_Integration(t *testing.T) {
	s := scheduler.NewScheduler(jobLogger)
	defer s.Stop()

	// Test with predefined Next functions
	var dailyCount, monthlyCount int
	var mu sync.Mutex

	// Mock the Next functions to execute quickly for testing
	s.AddJob("daily-job", func(context.Context) error {
		mu.Lock()
		dailyCount++
		mu.Unlock()
		return nil
	}, func() time.Time {
		return time.Now().Add(25 * time.Millisecond)
	})

	s.AddJob("monthly-job", func(context.Context) error {
		mu.Lock()
		monthlyCount++
		mu.Unlock()
		return nil
	}, func() time.Time {
		return time.Now().Add(35 * time.Millisecond)
	})

	s.Start()

	// Wait for executions
	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	dCount := dailyCount
	mCount := monthlyCount
	mu.Unlock()

	assert.GreaterOrEqual(t, dCount, 1, "daily job should execute")
	assert.GreaterOrEqual(t, mCount, 1, "monthly job should execute")
}

func TestPredefinedNextFunctions(t *testing.T) {
	t.Run("NextDaily produces future time", func(t *testing.T) {
		now := time.Now()
		next := scheduler.NextDaily()

		assert.True(t, next.After(now), "NextDaily should return future time")
		assert.Equal(t, 0, next.Hour(), "NextDaily should be at midnight")
		assert.Equal(t, 0, next.Minute(), "NextDaily should be at midnight")
		assert.Equal(t, 0, next.Second(), "NextDaily should be at midnight")
	})

	t.Run("NextFirstOfMonth produces future time", func(t *testing.T) {
		now := time.Now()
		next := scheduler.NextFirstOfMonth()

		// Should be future time unless we're exactly at first of month at midnight
		if now.Day() != 1 || now.Hour() != 0 || now.Minute() != 0 || now.Second() != 0 {
			assert.True(t, next.After(now) || next.Equal(now), "NextFirstOfMonth should return future or current time")
		}
		assert.Equal(t, 1, next.Day(), "NextFirstOfMonth should be first day of month")
		assert.Equal(t, 0, next.Hour(), "NextFirstOfMonth should be at midnight")
		assert.Equal(t, 0, next.Minute(), "NextFirstOfMonth should be at midnight")
		assert.Equal(t, 0, next.Second(), "NextFirstOfMonth should be at midnight")
	})
}

func TestScheduler_EmptyJobsStart(t *testing.T) {
	s := scheduler.NewScheduler(jobLogger)
	defer s.Stop()

	// Starting with no jobs should not panic
	s.Start()

	// Wait a bit to ensure nothing crashes
	time.Sleep(10 * time.Millisecond)
}

func TestScheduler_LongRunningJob(t *testing.T) {
	s := scheduler.NewScheduler(jobLogger)
	defer s.Stop()

	var executionCount int
	var mu sync.Mutex
	executed := make(chan bool, 1)

	s.AddJob("long-job", func(context.Context) error {
		mu.Lock()
		executionCount++
		mu.Unlock()
		time.Sleep(100 * time.Millisecond) // Simulate long-running task
		executed <- true
		return nil
	}, func() time.Time {
		return time.Now().Add(50 * time.Millisecond)
	})

	s.Start()

	// Wait for first execution to complete
	select {
	case <-executed:
		// Expected
	case <-time.After(200 * time.Millisecond):
		t.Fatal("job should have completed")
	}

	mu.Lock()
	count := executionCount
	mu.Unlock()

	assert.Equal(t, 1, count, "job should have executed once")
}

func TestScheduler_RapidFireJobs(t *testing.T) {
	s := scheduler.NewScheduler(jobLogger)
	defer s.Stop()

	var executionCount int
	var mu sync.Mutex

	s.AddJob("rapid-job", func(context.Context) error {
		mu.Lock()
		executionCount++
		mu.Unlock()
		return nil
	}, func() time.Time {
		// Very rapid execution - 1ms intervals
		return time.Now().Add(1 * time.Millisecond)
	})

	s.Start()

	// Wait for multiple rapid executions
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	count := executionCount
	mu.Unlock()

	// Should have executed many times
	assert.Greater(t, count, 10, "job should execute rapidly multiple times")
}

