// Package scheduler runs backup section jobs with bounded parallelism.
package scheduler

import (
	"context"
	"sync"
	"time"
)

// Job is one unit of work (one backup section).
type Job struct {
	Name string
	Run  func(ctx context.Context) error
}

// Result holds the outcome of a single Job.
type Result struct {
	Name     string
	Err      error
	Duration time.Duration
}

// Scheduler runs jobs with at most maxWorkers concurrent goroutines.
type Scheduler struct {
	maxWorkers int
	jobs       []Job
}

// New returns a Scheduler that runs at most maxWorkers jobs concurrently.
func New(maxWorkers int) *Scheduler {
	if maxWorkers < 1 {
		maxWorkers = 1
	}
	return &Scheduler{maxWorkers: maxWorkers}
}

// Submit enqueues a job. Must be called before Run.
func (s *Scheduler) Submit(job Job) {
	s.jobs = append(s.jobs, job)
}

// Run executes all submitted jobs and blocks until all complete or ctx is cancelled.
// Returns results in completion order (not submission order).
func (s *Scheduler) Run(ctx context.Context) []Result {
	sem := make(chan struct{}, s.maxWorkers)
	resultCh := make(chan Result, len(s.jobs))
	var wg sync.WaitGroup

	for _, job := range s.jobs {
		job := job
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Acquire semaphore slot.
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				resultCh <- Result{Name: job.Name, Err: ctx.Err()}
				return
			}
			defer func() { <-sem }()

			start := time.Now()
			err := job.Run(ctx)
			resultCh <- Result{
				Name:     job.Name,
				Err:      err,
				Duration: time.Since(start),
			}
		}()
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var results []Result
	for r := range resultCh {
		results = append(results, r)
	}
	return results
}
