package dispatcher

import (
	"context"
	"errors"
	"sync"
)

var ErrQueueFull = errors.New("event dispatcher queue is full")

// Dispatcher owns bounded event delivery. It does not know about Osmose
// events, which keeps protocol and public event mapping outside this package.
type Dispatcher[T any] struct {
	queue   chan item[T]
	workers int
	handler func(context.Context, T)

	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	started bool
	closed  bool
	wg      sync.WaitGroup
}

type item[T any] struct {
	ctx   context.Context
	value T
}

func New[T any](queueSize, workers int, handler func(context.Context, T)) *Dispatcher[T] {
	if queueSize <= 0 {
		queueSize = 1024
	}
	if workers <= 0 {
		workers = 1
	}
	return &Dispatcher[T]{
		queue:   make(chan item[T], queueSize),
		workers: workers,
		handler: handler,
	}
}

func (d *Dispatcher[T]) Start(parent context.Context) {
	if parent == nil {
		parent = context.Background()
	}
	d.mu.Lock()
	if d.started {
		d.mu.Unlock()
		return
	}
	d.started = true
	d.ctx, d.cancel = context.WithCancel(parent)
	if d.closed {
		d.mu.Unlock()
		return
	}
	d.wg.Add(d.workers)
	d.mu.Unlock()
	for i := 0; i < d.workers; i++ {
		go d.worker()
	}
}

func (d *Dispatcher[T]) Enqueue(ctx context.Context, value T) error {
	if ctx == nil {
		ctx = context.Background()
	}
	d.mu.Lock()
	started, dispatcherCtx, closed := d.started, d.ctx, d.closed
	d.mu.Unlock()
	if !started || closed {
		return context.Canceled
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-dispatcherCtx.Done():
		return context.Canceled
	case d.queue <- item[T]{ctx: ctx, value: value}:
		return nil
	default:
		return ErrQueueFull
	}
}

func (d *Dispatcher[T]) Close() {
	d.mu.Lock()
	if !d.closed {
		d.closed = true
		if d.cancel != nil {
			d.cancel()
		}
	}
	d.mu.Unlock()
	d.wg.Wait()
}

func (d *Dispatcher[T]) worker() {
	defer d.wg.Done()
	for {
		select {
		case <-d.ctx.Done():
			return
		case event := <-d.queue:
			if d.handler != nil {
				d.handler(event.ctx, event.value)
			}
		}
	}
}
