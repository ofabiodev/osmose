// Package collectors provides bounded helpers for waiting on typed events.
package collectors

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ofabiodev/osmose/events"
	"github.com/ofabiodev/osmose/types"
)

const defaultMessageCollectorBuffer = 16

var (
	ErrCollectorEnded    = errors.New("osmose collector ended")
	ErrCollectorTimeout  = errors.New("osmose collector timed out")
	ErrCollectorIdle     = errors.New("osmose collector became idle")
	ErrCollectorOverflow = errors.New("osmose collector buffer is full")
	ErrCollectorClosed   = errors.New("osmose client closed while collecting")
)

// EndReason explains why a message collector stopped.
type EndReason string

const (
	EndReasonTime     EndReason = "time"
	EndReasonIdle     EndReason = "idle"
	EndReasonLimit    EndReason = "limit"
	EndReasonStopped  EndReason = "stopped"
	EndReasonContext  EndReason = "context"
	EndReasonOverflow EndReason = "overflow"
	EndReasonClosed   EndReason = "client_closed"
)

// CollectorError reports an automatic collector termination.
type CollectorError struct{ Reason EndReason }

func (e *CollectorError) Error() string {
	if e == nil || e.Reason == "" {
		return ErrCollectorEnded.Error()
	}
	return fmt.Sprintf("collector ended: %s", e.Reason)
}

func (e *CollectorError) Unwrap() error {
	if e == nil {
		return nil
	}
	switch e.Reason {
	case EndReasonTime:
		return ErrCollectorTimeout
	case EndReasonIdle:
		return ErrCollectorIdle
	case EndReasonOverflow:
		return ErrCollectorOverflow
	case EndReasonClosed:
		return ErrCollectorClosed
	default:
		return ErrCollectorEnded
	}
}

// MessageCollectorOptions controls a message collector. A zero Max means
// unlimited matches; zero Time and Idle disable those automatic limits.
type MessageCollectorOptions struct {
	Chat     types.ChatRef
	AuthorID types.ID
	Filter   func(*events.MessageCreateEvent) bool

	Time   time.Duration
	Idle   time.Duration
	Max    int
	Buffer int
}

func (o MessageCollectorOptions) validate() error {
	if o.Chat != (types.ChatRef{}) && !o.Chat.Valid() {
		return fmt.Errorf("collector chat reference is invalid")
	}
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

// CollectorResult is the final state of a MessageCollector. Reason is empty
// while the collector is still active.
type CollectorResult struct {
	Collected int
	Reason    EndReason
	Err       error
}

// MessageCollector receives matching MessageCreate events until it is
// stopped, reaches a limit, or one of its automatic limits fires.
type MessageCollector struct {
	options MessageCollectorOptions
	events  chan *events.MessageCreateEvent
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

func NewMessages(ctx, lifecycleCtx context.Context, options MessageCollectorOptions, register func(func(context.Context, *events.MessageCreateEvent) error) func()) (*MessageCollector, error) {
	if err := options.validate(); err != nil {
		return nil, err
	}
	if options.Buffer == 0 {
		options.Buffer = defaultMessageCollectorBuffer
	}
	if ctx == nil {
		ctx = context.Background()
	}

	collector := &MessageCollector{
		options: options,
		events:  make(chan *events.MessageCreateEvent, options.Buffer),
		done:    make(chan struct{}),
	}
	collector.mu.Lock()
	if options.Time > 0 {
		collector.timeTimer = time.AfterFunc(options.Time, func() {
			collector.finish(EndReasonTime, nil)
		})
	}
	if options.Idle > 0 {
		collector.idleGeneration = 1
		generation := collector.idleGeneration
		collector.idleTimer = time.AfterFunc(options.Idle, func() {
			collector.finishIdle(generation)
		})
	}
	if ctx.Done() != nil {
		collector.stopContext = context.AfterFunc(ctx, func() {
			err := ctx.Err()
			if err == nil {
				err = context.Canceled
			}
			collector.finish(EndReasonContext, err)
		})
	}
	if lifecycleCtx != nil {
		collector.clientContext = context.AfterFunc(lifecycleCtx, func() {
			collector.finish(EndReasonClosed, nil)
		})
	}
	collector.mu.Unlock()
	collector.setRemove(register(collector.handle))
	return collector, nil
}

// Events returns matching messages and closes when the collector ends.
func (c *MessageCollector) Events() <-chan *events.MessageCreateEvent {
	if c == nil {
		return nil
	}
	return c.events
}

// Done closes when the collector ends.
func (c *MessageCollector) Done() <-chan struct{} {
	if c == nil {
		return nil
	}
	return c.done
}

// Result returns the current or final collector state.
func (c *MessageCollector) Result() CollectorResult {
	if c == nil {
		return CollectorResult{Reason: EndReasonStopped, Err: ErrCollectorEnded}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.result
}

// Stop ends the collector. Calling Stop more than once is safe.
func (c *MessageCollector) Stop(reason EndReason) {
	c.finish(reason, nil)
}

// Next waits for the next matching event or for the collector/context to end.
func (c *MessageCollector) Next(ctx context.Context) (*events.MessageCreateEvent, error) {
	if c == nil {
		return nil, ErrCollectorEnded
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case event, ok := <-c.events:
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
		c.finish(EndReasonContext, ctx.Err())
		return nil, ctx.Err()
	}
}

func (c *MessageCollector) matches(event *events.MessageCreateEvent) bool {
	if event == nil || event.Message == nil {
		return false
	}
	if c.options.Chat != (types.ChatRef{}) && event.Message.Chat != c.options.Chat {
		return false
	}
	if c.options.AuthorID != 0 && event.Message.AuthorID != c.options.AuthorID {
		return false
	}
	return c.options.Filter == nil || c.options.Filter(event)
}

func (c *MessageCollector) handle(_ context.Context, event *events.MessageCreateEvent) error {
	if !c.matches(event) {
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

func (c *MessageCollector) resetIdleLocked() {
	if c.idleTimer == nil {
		return
	}
	c.idleGeneration++
	generation := c.idleGeneration
	c.idleTimer.Stop()
	c.idleTimer = time.AfterFunc(c.options.Idle, func() {
		c.finishIdle(generation)
	})
}

func (c *MessageCollector) finishIdle(generation uint64) {
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

func (c *MessageCollector) setRemove(remove func()) {
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

func (c *MessageCollector) finish(reason EndReason, err error) {
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

func (c *MessageCollector) finishLocked(reason EndReason, err error) func() {
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
