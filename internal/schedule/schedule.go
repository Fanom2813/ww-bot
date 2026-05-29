// Package schedule runs daily tasks at a wall-clock time — used for proactive
// greetings (the morning salaam + dua to Dad, family check-ins). It is a small
// dependency-free scheduler; each task runs on its own goroutine and reschedules
// for the next day after firing.
package schedule

import (
	"context"
	"sync"
	"time"

	"wwbot/internal/dbg"
)

type task struct {
	name string
	hour int
	min  int
	run  func(context.Context)
}

// Scheduler runs registered daily tasks.
type Scheduler struct {
	mu      sync.Mutex
	tasks   []task
	quit    chan struct{}
	wg      sync.WaitGroup
	started bool
}

// New creates a Scheduler.
func New() *Scheduler { return &Scheduler{quit: make(chan struct{})} }

// Daily registers a task to run every day at hour:min (local time). Must be
// called before Start.
func (s *Scheduler) Daily(name string, hour, min int, run func(context.Context)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks = append(s.tasks, task{name: name, hour: hour, min: min, run: run})
}

// Start launches a goroutine per task.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return
	}
	s.started = true
	for _, t := range s.tasks {
		s.wg.Add(1)
		go s.runTask(ctx, t)
	}
}

// Stop stops all tasks and waits for them to exit.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	close(s.quit)
	s.wg.Wait()
}

func (s *Scheduler) runTask(ctx context.Context, t task) {
	defer s.wg.Done()
	defer dbg.Recover("schedule.runTask")
	for {
		timer := time.NewTimer(time.Until(nextRun(time.Now(), t.hour, t.min)))
		select {
		case <-s.quit:
			timer.Stop()
			return
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			t.run(ctx)
		}
	}
}

// nextRun returns the next occurrence of hour:min strictly after now.
func nextRun(now time.Time, hour, min int) time.Time {
	n := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
	if !n.After(now) {
		n = n.Add(24 * time.Hour)
	}
	return n
}
