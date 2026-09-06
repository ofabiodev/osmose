package gateway

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// Keepalive enqueues the protocol-level payload selected by the client at a
// fixed interval. The gateway deliberately does not know about protobuf
// oneofs; the client prepares the current Osmium keepalive frame.
type Keepalive struct {
	ctx       context.Context
	cancel    context.CancelFunc
	interval  time.Duration
	enqueue   func(context.Context, Frame) error
	frame     Frame
	logger    *slog.Logger
	startOnce sync.Once
	wg        sync.WaitGroup
}

func NewKeepalive(ctx context.Context, interval time.Duration, enqueue func(context.Context, Frame) error, frame Frame, logger *slog.Logger) *Keepalive {
	if ctx == nil {
		ctx = context.Background()
	}
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(ctx)
	return &Keepalive{ctx: ctx, cancel: cancel, interval: interval, enqueue: enqueue, frame: frame, logger: logger}
}

func (k *Keepalive) Start() {
	k.startOnce.Do(func() {
		if k.interval <= 0 || k.enqueue == nil {
			return
		}
		k.wg.Add(1)
		go func() {
			defer k.wg.Done()
			ticker := time.NewTicker(k.interval)
			defer ticker.Stop()
			for {
				select {
				case <-k.ctx.Done():
					return
				case <-ticker.C:
					if err := k.enqueue(k.ctx, k.frame); err != nil {
						if k.ctx.Err() != nil || errors.Is(err, ErrClosed) {
							return
						}
						k.logger.Warn("keepalive enqueue failed", "error", err)
					}
				}
			}
		}()
	})
}

func (k *Keepalive) Stop() {
	if k.cancel != nil {
		k.cancel()
	}
	k.wg.Wait()
}
