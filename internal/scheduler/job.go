package scheduler

import (
	"context"
	"log/slog"
	"time"
)

type Job struct {
	Name string
	Run  func(context.Context) error
	Next func() time.Time
}

type Scheduler struct {
	logger *slog.Logger
	jobs   []*Job
	ctx    context.Context
	cancel context.CancelFunc
}

var (
	NextDaily = func() time.Time {
		now := time.Now()
		tomorrow := now.AddDate(0, 0, 1)
		return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, now.Location())
	}
	NextFirstOfMonth = func() time.Time {
		now := time.Now()
		// Always go to next month, day 1, midnight
		nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())

		// If we're already on the 1st before midnight, use this month
		if now.Day() == 1 && now.Hour() == 0 && now.Minute() == 0 && now.Second() == 0 {
			return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		}

		return nextMonth
	}
)

func NewScheduler(logger *slog.Logger) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		logger: logger,
		jobs:   []*Job{},
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *Scheduler) AddJob(name string, run func(context.Context) error, next func() time.Time) *Job {
	job := &Job{
		Name: name,
		Run:  run,
		Next: next,
	}
	s.jobs = append(s.jobs, job)
	return job
}

func (s *Scheduler) Start() {
	for _, job := range s.jobs {
		go s.runJob(job)
	}
}

func (s *Scheduler) Stop() {
	s.cancel()
	s.logger.InfoContext(s.ctx, "Scheduler stopped")
}

func (s *Scheduler) runJob(job *Job) {
	nextRun := job.Next()
	s.logger.InfoContext(s.ctx, "Starting job", "job", job.Name, "nextRun", nextRun)

	timer := time.NewTimer(time.Until(nextRun))
	defer timer.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.InfoContext(s.ctx, "Job stopped", "job", job.Name)
			return
		case <-timer.C:
			s.logger.InfoContext(s.ctx, "Executing job", "job", job.Name)

			if err := job.Run(s.ctx); err != nil {
				s.logger.ErrorContext(s.ctx, "Job execution", "job", job.Name, "error", err)
			} else {
				s.logger.InfoContext(s.ctx, "Job completed successfully", "job", job.Name)
			}

			nextRun = job.Next()
			timer.Reset(time.Until(nextRun))
			s.logger.InfoContext(s.ctx, "Next run scheduled", "job", job.Name, "nextRun", nextRun)
		}
	}
}
