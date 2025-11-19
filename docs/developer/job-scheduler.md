---
title: "Job Scheduler"
sidebar: true
order: 7
---

# Job Scheduler

Lightweight, in-process task scheduler for periodic background tasks (monthly credit top-ups, cleanup operations, recurring maintenance). Uses goroutines for concurrent execution with graceful shutdown support.

**Location**: `/internal/scheduler/job.go`

**Key Features**: Concurrent execution, context-based cancellation, configurable scheduling, built-in slog logging, no external dependencies

## Architecture

### Core Components

```go
type Scheduler struct {
    logger *slog.Logger
    jobs   []*Job
    ctx    context.Context
    cancel context.CancelFunc
}

type Job struct {
    Name string                       // Human-readable job name
    Run  func(context.Context) error  // Function to execute
    Next func() time.Time             // Next run time calculator
}
```

### Built-in Scheduling Functions

**NextDaily** - Runs at midnight every day (daily cleanup, stale data removal, reports)

**NextFirstOfMonth** - Runs at midnight on the first day of each month (monthly top-ups, billing, reports)

### Execution Model

1. Job added → next run time calculated → timer created
2. Goroutine blocks on timer or context cancellation
3. Timer fires → `Run` function executes
4. Next run time recalculated → timer reset → repeat

**Concurrency**: Each job runs in its own goroutine, sharing a context for coordinated shutdown.

**Error Handling**: Errors are logged but don't prevent rescheduling (temporary failures won't disable jobs).

## Usage

### Setup and Registration

```go
// In cmd/rapua/main.go

// Create scheduler
jobs := scheduler.NewScheduler(logger)

// Register jobs
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

jobs.AddJob(
    "Orphaned Uploads Cleanup",
    func(ctx context.Context) error {
        if err := orphanedUploadsCleanupService.CleanupOrphanedUploads(ctx); err != nil {
            return err
        }
        return orphanedUploadsCleanupService.CleanupEmptyDirectories(ctx)
    },
    scheduler.NextDaily,
)

// Start all jobs
jobs.Start()

// Graceful shutdown
jobs.Stop()
```

## Adding a New Job

### 1. Create Service

```go
type MyService struct {
    // dependencies
}

func (s *MyService) PerformTask(ctx context.Context) error {
    slog.Info("Running my scheduled task")

    if err := someOperation(); err != nil {
        return fmt.Errorf("failed to perform task: %w", err)
    }

    return nil
}
```

### 2. Initialize & Register

```go
// In main.go
myService := services.NewMyService(transactor, logger)

jobs.AddJob(
    "My Scheduled Task",
    myService.PerformTask,
    scheduler.NextDaily, // or NextFirstOfMonth, or custom
)
```

### 3. Custom Schedules

```go
// Every 6 hours
nextSixHours := func() time.Time {
    return time.Now().Add(6 * time.Hour)
}

// Every Monday at 3 AM
nextMonday3AM := func() time.Time {
    now := time.Now()
    daysUntilMonday := (7 - int(now.Weekday()) + 1) % 7
    if daysUntilMonday == 0 && now.Hour() >= 3 {
        daysUntilMonday = 7
    }
    nextMonday := now.AddDate(0, 0, daysUntilMonday)
    return time.Date(nextMonday.Year(), nextMonday.Month(), nextMonday.Day(), 3, 0, 0, 0, now.Location())
}
```

## Current Scheduled Jobs

### Monthly Credit Top-Up
**Schedule**: First of month at midnight
**Function**: `monthlyCreditTopupJob.TopUpCredits`
**Purpose**: Replenishes users' free credits to their monthly limit

Processes users in batches by tier, uses idempotency checks, creates audit logs, implements retry logic with exponential backoff.

**Service**: `/internal/services/monthly_credit_topup.go`

### Stale Credit Purchase Cleanup
**Schedule**: Daily at midnight
**Function**: `staleCreditCleanupService.CleanupStalePurchases`
**Purpose**: Removes abandoned/failed purchase records older than 7 days

Deletes `pending` or `failed` purchases, preserves `completed` purchases, uses transactions, logs cleanup count.

**Service**: `/internal/services/stale_purchase_cleanup.go`

### Orphaned Uploads Cleanup
**Schedule**: Daily at midnight
**Function**: `orphanedUploadsCleanupService.CleanupOrphanedUploads` + `CleanupEmptyDirectories`
**Purpose**: Removes orphaned upload files not referenced by any blocks

**Process**:
1. Walks `static/uploads/` directory
2. Checks if each file is referenced in any `blocks.data` field
3. Uses smart URL filtering (local uploads vs external CDNs via `SITE_URL` env)
4. Deletes unreferenced files
5. Removes empty date-based directories

**Safety**: LIKE pattern escaping prevents false matches, environment-aware validation skips external URLs, direct path construction for O(1) lookups, respects context cancellation.

**Service**: `/internal/services/orphaned_uploads_cleanup.go`

**Complementary Strategy**: Works alongside inline cleanup in `DeleteService.DeleteBlock()` as safety net for failed uploads or manual DB edits.

## Best Practices

### Job Design
1. **Idempotency**: Safe to run multiple times with same result
2. **Context Awareness**: Respect context for cancellation
3. **Error Handling**: Return errors but design for failures (retry next cycle)
4. **Timeouts**: Add timeouts for long-running operations
5. **Logging**: Use structured logging with relevant context

### Example: Idempotent Job

```go
func (s *Service) ProcessRecords(ctx context.Context) error {
    lastRun, err := s.getLastRunTime(ctx)
    if err != nil {
        return fmt.Errorf("failed to check last run: %w", err)
    }

    if lastRun.After(time.Now().Add(-24 * time.Hour)) {
        slog.Info("Already processed in last 24 hours, skipping")
        return nil
    }

    // Perform work...

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
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            if err := s.processItem(ctx, item); err != nil {
                slog.Error("Failed to process item", "item", item.ID, "error", err)
            }
        }
    }

    return nil
}
```

## Testing

### Basic Test

```go
func TestMyService_PerformTask(t *testing.T) {
    service := setupService(t)
    err := service.PerformTask(context.Background())
    require.NoError(t, err)
}
```

### Idempotency Test

```go
func TestMyService_PerformTask_Idempotency(t *testing.T) {
    service := setupService(t)
    ctx := context.Background()

    require.NoError(t, service.PerformTask(ctx))
    require.NoError(t, service.PerformTask(ctx)) // Run again - should be safe

    // Verify only one set of changes occurred
}
```

### Context Cancellation Test

```go
func TestMyService_PerformTask_ContextCancellation(t *testing.T) {
    service := setupService(t)
    ctx, cancel := context.WithCancel(context.Background())
    cancel()

    err := service.PerformTask(ctx)
    if err != nil {
        require.ErrorIs(t, err, context.Canceled)
    }
}
```

## Monitoring & Troubleshooting

### Log Output

```
INFO Starting job job=Monthly Credit Top-Up nextRun=2025-11-01T00:00:00+13:00
INFO Executing job job=Monthly Credit Top-Up
INFO Job completed successfully job=Monthly Credit Top-Up
INFO Next run scheduled job=Monthly Credit Top-Up nextRun=2025-12-01T00:00:00+13:00

ERROR Job execution job=Stale Credit Purchase Cleanup error=failed to connect to database
```

### Common Issues

**Job Not Running**
- Verify job registered before `jobs.Start()`
- Check logs for "Starting job" message
- Verify `Next` function returns future timestamps
- Ensure scheduler hasn't been stopped

**Job Running Multiple Times**
- Job registered multiple times
- Multiple app instances running
- `Next` function returning past timestamps
- **Solution**: Add idempotency checks

**Jobs Not Stopping on Shutdown**
- Verify `jobs.Stop()` is called
- Ensure job functions respect context cancellation
- Add timeouts to long-running operations

**Timezone Issues**
Scheduler uses system local timezone. For consistency:
```go
nextRunUTC := func() time.Time {
    now := time.Now().UTC()
    tomorrow := now.AddDate(0, 0, 1)
    return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, time.UTC)
}
```
