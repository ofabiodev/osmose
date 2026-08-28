package collectors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ofabiodev/osmose/events"
	"github.com/ofabiodev/osmose/types"
)

// InteractionCollectorOptions controls an interaction collector. A zero Max
// means unlimited matches; zero Time and Idle disable those automatic limits.
type InteractionCollectorOptions struct {
	UserID    types.ID
	MessageID types.ID
	Data      string
	Filter    func(*events.InteractionEvent) bool

	Time   time.Duration
	Idle   time.Duration
	Max    int
	Buffer int
}

func (o InteractionCollectorOptions) validate() error {
	if o.Time < 0 || o.Idle < 0 {
		return fmt.Errorf("collector durations cannot be negative")
	}
	if o.Max < 0 {
		return fmt.Errorf("collector max cannot be negative")
	}
	if o.Buffer < 0 {
		return fmt.Errorf("collector buffer cannot be negative")
	}
	return nil
}

type collectorCoreOptions struct {
	Time   time.Duration
	Idle   time.Duration
	Max    int
	Buffer int
}

type collectorCore[T any] struct {
	options collectorCoreOptions
	accept  func(T) bool
	events  chan T
	done    chan struct{}

	mu             sync.Mutex
	result         CollectorResult
	ended          bool
	remove         func()
	stopContext    func() bool
	clientContext  func() bool
	timeTimer      *time.Timer
	idleTimer      *time.Timer
	idleGeneration uint64
}

func newCollectorCore[T any](ctx, lifecycleCtx context.Context, options collectorCoreOptions, accept func(T) bool, register func(func(context.Context, T) error) func()) *collectorCore[T] {
	if options.Buffer == 0 {
		options.Buffer = defaultMessageCollectorBuffer
	}
	core := &collectorCore[T]{
		options: options,
		accept:  accept,
		events:  make(chan T, options.Buffer),
		done:    make(chan struct{}),
	}
	core.mu.Lock()
	if options.Time > 0 {
		core.timeTimer = time.AfterFunc(options.Time, func() { core.finish(EndReasonTime, nil) })
	}
	if options.Idle > 0 {
		core.idleGeneration = 1
		generation := core.idleGeneration
		core.idleTimer = time.AfterFunc(options.Idle, func() { core.finishIdle(generation) })
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Done() != nil {
		core.stopContext = context.AfterFunc(ctx, func() {
			err := ctx.Err()
			if err == nil {
				err = context.Canceled
			}
			core.finish(EndReasonContext, err)
		})
	}
	if lifecycleCtx != nil {
		core.clientContext = context.AfterFunc(lifecycleCtx, func() { core.finish(EndReasonClosed, nil) })
	}
	core.mu.Unlock()
	core.setRemove(register(core.handle))
	return core
}

func (c *collectorCore[T]) handle(_ context.Context, event T) error {
	if c == nil || c.accept == nil || !c.accept(event) {
		return nil
	}
	c.mu.Lock()
	if c.ended {
		c.mu.Unlock()
		return nil
	}
	select {
	case c.events <- event:
		c.result.Collected++
		c.resetIdleLocked()
		var remove func()
		if c.options.Max > 0 && c.result.Collected >= c.options.Max {
			remove = c.finishLocked(EndReasonLimit, nil)
		}
		c.mu.Unlock()
		if remove != nil {
			remove()
		}
		return nil
	default:
		err := &CollectorError{Reason: EndReasonOverflow}
		remove := c.finishLocked(EndReasonOverflow, err)
		c.mu.Unlock()
		if remove != nil {
			remove()
		}
		return nil
	}
}

func (c *collectorCore[T]) resetIdleLocked() {
	if c.idleTimer == nil {
		return
	}
	c.idleGeneration++
	generation := c.idleGeneration
	c.idleTimer.Stop()
	c.idleTimer = time.AfterFunc(c.options.Idle, func() { c.finishIdle(generation) })
}

func (c *collectorCore[T]) finishIdle(generation uint64) {
	c.mu.Lock()
	if c.ended || c.idleGeneration != generation {
		c.mu.Unlock()
		return
	}
	remove := c.finishLocked(EndReasonIdle, nil)
	c.mu.Unlock()
	if remove != nil {
		remove()
	}
}

func (c *collectorCore[T]) setRemove(remove func()) {
	if remove == nil {
		return
	}
	c.mu.Lock()
	if !c.ended {
		c.remove = remove
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()
	remove()
}

func (c *collectorCore[T]) finish(reason EndReason, err error) {
	if c == nil {
		return
	}
	c.mu.Lock()
	remove := c.finishLocked(reason, err)
	c.mu.Unlock()
	if remove != nil {
		remove()
	}
}

func (c *collectorCore[T]) finishLocked(reason EndReason, err error) func() {
	if c.ended {
		return nil
	}
	if reason == "" {
		reason = EndReasonStopped
	}
	if err == nil {
		switch reason {
		case EndReasonTime, EndReasonIdle, EndReasonOverflow, EndReasonContext, EndReasonClosed:
			err = &CollectorError{Reason: reason}
		}
	}
	c.ended = true
	c.result.Reason = reason
	c.result.Err = err
	if c.timeTimer != nil {
		c.timeTimer.Stop()
	}
	if c.idleTimer != nil {
		c.idleTimer.Stop()
	}
	if c.stopContext != nil {
		c.stopContext()
		c.stopContext = nil
	}
	if c.clientContext != nil {
		c.clientContext()
		c.clientContext = nil
	}
	close(c.done)
	close(c.events)
	remove := c.remove
	c.remove = nil
	return remove
}

// InteractionCollector receives matching interaction events until it is
// stopped, reaches a limit, or one of its automatic limits fires.
type InteractionCollector struct {
	options InteractionCollectorOptions
	core    *collectorCore[*events.InteractionEvent]
}

func NewInteractions(ctx, lifecycleCtx context.Context, options InteractionCollectorOptions, register func(func(context.Context, *events.InteractionEvent) error) func()) (*InteractionCollector, error) {
	if err := options.validate(); err != nil {
		return nil, err
	}

	collector := &InteractionCollector{options: options}
	collector.core = newCollectorCore(ctx, lifecycleCtx, collectorCoreOptions{
		Time: options.Time, Idle: options.Idle, Max: options.Max, Buffer: options.Buffer,
	}, collector.matches, register)
	return collector, nil
}

func (c *InteractionCollector) matches(event *events.InteractionEvent) bool {
	if event == nil {
		return false
	}
	if c.options.UserID != 0 && event.UserID != c.options.UserID {
		return false
	}
	if c.options.MessageID != 0 && event.MessageID != c.options.MessageID {
		return false
	}
	if c.options.Data != "" && event.Data != c.options.Data {
		return false
	}
	return c.options.Filter == nil || c.options.Filter(event)
}

// Events returns matching interactions and closes when the collector ends.
func (c *InteractionCollector) Events() <-chan *events.InteractionEvent {
	if c == nil || c.core == nil {
		return nil
	}
	return c.core.events
}

// Done closes when the collector ends.
func (c *InteractionCollector) Done() <-chan struct{} {
	if c == nil || c.core == nil {
		return nil
	}
	return c.core.done
}

// Result returns the current or final collector state.
func (c *InteractionCollector) Result() CollectorResult {
	if c == nil || c.core == nil {
		return CollectorResult{Reason: EndReasonStopped, Err: ErrCollectorEnded}
	}
	c.core.mu.Lock()
	defer c.core.mu.Unlock()
	return c.core.result
}

// Stop ends the collector. Calling Stop more than once is safe.
func (c *InteractionCollector) Stop(reason EndReason) {
	if c != nil && c.core != nil {
		c.core.finish(reason, nil)
	}
}

// Next waits for the next matching interaction or for the collector/context
// to end.
func (c *InteractionCollector) Next(ctx context.Context) (*events.InteractionEvent, error) {
	if c == nil || c.core == nil {
		return nil, ErrCollectorEnded
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case event, ok := <-c.core.events:
		if ok {
			return event, nil
		}
		result := c.Result()
		if result.Err != nil {
			return nil, result.Err
		}
		reason := result.Reason
		if reason == "" {
			reason = EndReasonStopped
		}
		return nil, &CollectorError{Reason: reason}
	case <-ctx.Done():
		c.core.finish(EndReasonContext, ctx.Err())
		return nil, ctx.Err()
	}
}
