package scheduler

import (
	"context"
	"sync"
	"time"
)

// Scheduler applies one optional minimum interval to outbound RPCs.
// ponytail: one client-wide interval; add protocol-provided buckets when the
// Osmium schema exposes rate-limit metadata.
type Scheduler struct {
	Interval time.Duration

	mu   sync.Mutex
	next time.Time
}

func (s *Scheduler) Wait(ctx context.Context) error {
	if s == nil || s.Interval <= 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	s.mu.Lock()
	now := time.Now()
	at := s.next
	if at.Before(now) {
		at = now
	}
	s.next = at.Add(s.Interval)
	delay := time.Until(at)
	s.mu.Unlock()

	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
