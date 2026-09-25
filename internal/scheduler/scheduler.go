package scheduler

import (
	"context"
	"sync"
	"time"
)

type JobFunc func(ctx context.Context) error

type Scheduler struct {
	interval time.Duration
	job      JobFunc
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

func NewScheduler(interval time.Duration, job JobFunc) *Scheduler {
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return &Scheduler{
		interval: interval,
		job:      job,
		stopCh:   make(chan struct{}),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stopCh:
				return
			case <-ticker.C:
				if s.job != nil {
					_ = s.job(ctx)
				}
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}
