package scheduler

import (
	"context"
	"log"
	"time"
)

type Job struct {
	Name string
	Run  func() error
	Next func() time.Time
}

type Scheduler struct {
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

func NewScheduler() *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		jobs:   []*Job{},
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *Scheduler) AddJob(name string, run func() error, next func() time.Time) *Job {
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
	log.Println("Scheduler stopped")
}

func (s *Scheduler) runJob(job *Job) {
	nextRun := job.Next()
	log.Printf("Starting job: %s, next run at: %s", job.Name, nextRun)

	timer := time.NewTimer(time.Until(nextRun))
	defer timer.Stop()

	for {
		select {
		case <-s.ctx.Done():
			log.Printf("Stopping job: %s", job.Name)
			return
		case <-timer.C:
			log.Printf("Running job: %s", job.Name)

			if err := job.Run(); err != nil {
				log.Printf("Job %s failed: %v", job.Name, err)
			} else {
				log.Printf("Job %s completed successfully", job.Name)
			}

			nextRun = job.Next()
			timer.Reset(time.Until(nextRun))
			log.Printf("Next run for job %s at: %s", job.Name, nextRun)
		}
	}
}
