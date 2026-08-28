package gateway

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

type Backoff struct {
	Min time.Duration
	Max time.Duration
	rng *rand.Rand
	mu  sync.Mutex
}

func NewBackoff(minimum, maximum time.Duration) *Backoff {
	if minimum <= 0 {
		minimum = 500 * time.Millisecond
	}
	if maximum < minimum {
		maximum = minimum
	}
	return &Backoff{Min: minimum, Max: maximum, rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

func (b *Backoff) Delay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	delay := b.Min
	for i := 0; i < attempt && delay < b.Max; i++ {
		if delay > b.Max/2 {
			delay = b.Max
		} else {
			delay *= 2
		}
	}
	if delay > b.Max {
		delay = b.Max
	}
	b.mu.Lock()
	jitter := 0.8 + b.rng.Float64()*0.4
	b.mu.Unlock()
	result := time.Duration(float64(delay) * jitter)
	if result <= 0 {
		return time.Nanosecond
	}
	if result > b.Max {
		return b.Max
	}
	return result
}

func WaitBackoff(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
