---
title: "Job Scheduler"
sidebar: true
order: 7
---

# Job Scheduler

Rapua includes a simple built-in job scheduler for running periodic background tasks. The scheduler handles tasks like monthly credit top-ups, cleanup operations, and other recurring maintenance jobs.

## Overview

The job scheduler is a lightweight, in-process task scheduler that runs jobs at configurable intervals. It uses goroutines for concurrent job execution and supports graceful shutdown.

**Location**: `/internal/scheduler/job.go`

**Key Features**:
- Concurrent job execution using goroutines
- Context-based cancellation for graceful shutdown
- Configurable scheduling functions
- Built-in logging with slog
- No external dependencies (no cron syntax)

## Architecture

### Core Components

#### Scheduler

The `Scheduler` manages all registered jobs and their lifecycle:

```go
type Scheduler struct {
    logger *slog.Logger
    jobs   []*Job
    ctx    context.Context
    cancel context.CancelFunc
}
```

#### Job

Each `Job` represents a scheduled task:

```go
type Job struct {
    Name string                           // Human-readable job name
    Run  func(context.Context) error      // Function to execute
    Next func() time.Time                 // Function that calculates next run time
}
```

### Built-in Scheduling Functions

The scheduler provides two pre-configured scheduling functions:

#### NextDaily

Runs the job at midnight every day:

```go
NextDaily = func() time.Time {
    now := time.Now()
    tomorrow := now.AddDate(0, 0, 1)
    return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, now.Location())
}
```

**Use Cases**: Daily cleanup tasks, daily report generation, stale data removal

#### NextFirstOfMonth

Runs the job at midnight on the first day of each month:

```go
NextFirstOfMonth = func() time.Time {
    now := time.Now()
    nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())

    // Special case: if already on 1st at midnight, use this month
    if now.Day() == 1 && now.Hour() == 0 && now.Minute() == 0 && now.Second() == 0 {
        return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
    }

    return nextMonth
}
```

**Use Cases**: Monthly credit top-ups, monthly billing cycles, monthly reports

## How It Works

### Job Execution Flow

1. **Initialization**: When a job is added, the scheduler calculates the next run time
2. **Timer Setup**: A timer is created for the duration until the next run
3. **Waiting**: The job goroutine blocks on either:
   - The timer firing (time to run)
   - The context being cancelled (shutdown signal)
4. **Execution**: When the timer fires, the job's `Run` function is called
5. **Rescheduling**: After execution, the next run time is calculated and the timer is reset
6. **Loop**: Steps 3-5 repeat until the scheduler is stopped

### Concurrency Model

- Each job runs in its own goroutine
- Jobs execute independently and don't block each other
- All jobs share the same context for coordinated shutdown
- The scheduler uses `context.WithCancel` for graceful termination

### Error Handling

Jobs that return errors are logged but don't prevent rescheduling:

```go
if err := job.Run(s.ctx); err != nil {
    slog.Error("Job execution", "job", job.Name, "error", err)
} else {
    slog.Info("Job completed successfully", "job", job.Name)
}
```

This ensures that temporary failures don't permanently disable scheduled jobs.

## Usage

### Creating the Scheduler

In `cmd/rapua/main.go`, the scheduler is initialised during application startup:

```go
// Create the scheduler
jobs := scheduler.NewScheduler(logger)
```

### Registering Jobs

Jobs are registered using the `AddJob` method:

```go
jobs.AddJob(
    "Monthly Credit Top-Up",
    monthlyCreditTopupJob.TopUpCredits,
    scheduler.NextFirstOfMonth,
)

jobs.AddJob(
    "Stale Credit Purchase Cleanup",
    staleCreditCleanupService.CleanupStalePurchases,
    scheduler.NextDaily,
)
```

**Parameters**:
- `name`: Descriptive name for logging
- `run`: Function that performs the work (must accept `context.Context` and return `error`)
- `next`: Function that calculates the next run time

### Starting the Scheduler

After registering all jobs, start the scheduler:

```go
jobs.Start()
```

This spawns a goroutine for each registered job.

### Stopping the Scheduler

For graceful shutdown, call `Stop()`:

```go
jobs.Stop()
```

This cancels the shared context, causing all job goroutines to exit cleanly.

## Adding a New Job

Follow these steps to add a new scheduled job:

### 1. Create the Service

Create a service with a method that matches the job signature:

```go
type MyService struct {
    // dependencies
}

func (s *MyService) PerformTask(ctx context.Context) error {
    // Your job logic here
    slog.Info("Running my scheduled task")

    // Return error if something goes wrong
    if err := someOperation(); err != nil {
        return fmt.Errorf("failed to perform task: %w", err)
    }

    return nil
}
```

### 2. Initialise the Service

In `cmd/rapua/main.go`, initialise your service:

```go
myService := services.NewMyService(transactor, logger)
```

### 3. Register the Job

Add your job to the scheduler:

```go
jobs.AddJob(
    "My Scheduled Task",
    myService.PerformTask,
    scheduler.NextDaily, // or NextFirstOfMonth, or custom function
)
```

### 4. Custom Scheduling Functions

If you need a custom schedule, create a function that returns `time.Time`:

```go
// Run every 6 hours
nextSixHours := func() time.Time {
    return time.Now().Add(6 * time.Hour)
}

jobs.AddJob(
    "Six Hour Task",
    myService.PerformTask,
    nextSixHours,
)
```

```go
// Run every Monday at 3 AM
nextMonday3AM := func() time.Time {
    now := time.Now()
    // Calculate days until next Monday
    daysUntilMonday := (7 - int(now.Weekday()) + 1) % 7
    if daysUntilMonday == 0 && now.Hour() >= 3 {
        daysUntilMonday = 7 // Already past 3 AM on Monday, wait a week
    }

    nextMonday := now.AddDate(0, 0, daysUntilMonday)
    return time.Date(nextMonday.Year(), nextMonday.Month(), nextMonday.Day(), 3, 0, 0, 0, now.Location())
}

jobs.AddJob(
    "Weekly Monday Task",
    myService.PerformTask,
    nextMonday3AM,
)
```

## Current Scheduled Jobs

### Monthly Credit Top-Up

**Schedule**: First of every month at midnight
**Function**: `monthlyCreditTopupJob.TopUpCredits`
**Purpose**: Replenishes users' free credits to their monthly limit

**Details**:
- Processes users in batches by credit limit tier
- Uses idempotency checks to prevent duplicate top-ups
- Creates credit adjustment logs for audit trail
- Implements retry logic with exponential backoff

**Service**: `/internal/services/monthly_credit_topup.go`

### Stale Credit Purchase Cleanup

**Schedule**: Daily at midnight
**Function**: `staleCreditCleanupService.CleanupStalePurchases`
**Purpose**: Removes abandoned or failed purchase records older than 7 days

**Details**:
- Deletes purchases in `pending` or `failed` status
- Preserves all `completed` purchases regardless of age
- Uses database transactions for safe deletion
- Logs the number of records cleaned up

**Service**: `/internal/services/stale_purchase_cleanup.go`

## Best Practices

### Job Design

1. **Idempotency**: Jobs should be safe to run multiple times with the same result
2. **Context Awareness**: Always respect the context for cancellation
3. **Error Handling**: Return errors but design for failures (job will retry next cycle)
4. **Timeouts**: Consider adding timeouts for long-running operations
5. **Logging**: Use structured logging with relevant context

### Example: Idempotent Job

```go
func (s *Service) ProcessRecords(ctx context.Context) error {
    // Check if already processed this period
    lastRun, err := s.getLastRunTime(ctx)
    if err != nil {
        return fmt.Errorf("failed to check last run: %w", err)
    }

    if lastRun.After(time.Now().Add(-24 * time.Hour)) {
        slog.Info("Already processed in last 24 hours, skipping")
        return nil
    }

    // Perform work...

    // Update last run time
    return s.setLastRunTime(ctx, time.Now())
}
```

### Example: Context-Aware Job

```go
func (s *Service) ProcessBatch(ctx context.Context) error {
    items, err := s.getItems(ctx)
    if err != nil {
        return err
    }

    for _, item := range items {
        // Check for cancellation
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            if err := s.processItem(ctx, item); err != nil {
                slog.Error("Failed to process item", "item", item.ID, "error", err)
                // Continue processing other items
            }
        }
    }

    return nil
}
```

## Testing

### Unit Testing Jobs

Test your job functions independently:

```go
func TestMyService_PerformTask(t *testing.T) {
    service := setupService(t)
    ctx := context.Background()

    err := service.PerformTask(ctx)
    require.NoError(t, err)

    // Verify expected outcomes
}
```

### Testing Idempotency

```go
func TestMyService_PerformTask_Idempotency(t *testing.T) {
    service := setupService(t)
    ctx := context.Background()

    // Run once
    err := service.PerformTask(ctx)
    require.NoError(t, err)

    // Run again - should be safe
    err = service.PerformTask(ctx)
    require.NoError(t, err)

    // Verify only one set of changes occurred
}
```

### Testing Context Cancellation

```go
func TestMyService_PerformTask_ContextCancellation(t *testing.T) {
    service := setupService(t)
    ctx, cancel := context.WithCancel(context.Background())
    cancel() // Cancel immediately

    err := service.PerformTask(ctx)

    // Should handle cancellation gracefully
    if err != nil {
        require.ErrorIs(t, err, context.Canceled)
    }
}
```

## Monitoring

### Log Output

The scheduler produces structured logs for job lifecycle:

```
INFO Starting job job=Monthly Credit Top-Up nextRun=2025-11-01T00:00:00+13:00
INFO Executing job job=Monthly Credit Top-Up
INFO Job completed successfully job=Monthly Credit Top-Up
INFO Next run scheduled job=Monthly Credit Top-Up nextRun=2025-12-01T00:00:00+13:00
```

### Error Logs

Failed jobs produce error logs:

```
ERROR Job execution job=Stale Credit Purchase Cleanup error=failed to connect to database
```

## Troubleshooting

### Job Not Running

**Check**:
1. Verify job is registered before `jobs.Start()` is called
2. Check logs for "Starting job" message
3. Verify the `Next` function returns future timestamps
4. Ensure the scheduler hasn't been stopped

### Job Running Multiple Times

**Causes**:
- Job is registered multiple times
- Multiple instances of the application running
- `Next` function returning past timestamps

**Solution**: Add idempotency checks to your job function

### Jobs Not Stopping on Shutdown

**Check**:
1. Verify `jobs.Stop()` is called during shutdown
2. Ensure job functions respect context cancellation
3. Add timeouts to long-running operations

### Timezone Issues

The scheduler uses the system's local timezone. For consistent behavior across deployments:

```go
// Use UTC for all scheduling
nextRunUTC := func() time.Time {
    now := time.Now().UTC()
    tomorrow := now.AddDate(0, 0, 1)
    return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, time.UTC)
}
```
