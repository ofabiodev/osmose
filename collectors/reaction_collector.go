package collectors

import (
	"context"
	"fmt"
	"time"

	"github.com/ofabiodev/osmose/events"
	"github.com/ofabiodev/osmose/types"
)

// ReactionCollectorOptions controls a collector for message reaction
// snapshots. The current protocol sends the complete reaction state for a
// message, not an individual add/remove event.
type ReactionCollectorOptions struct {
	Chat      types.ChatRef
	MessageID types.ID
	Filter    func(*events.MessageReactionsEvent) bool

	Time   time.Duration
	Idle   time.Duration
	Max    int
	Buffer int
}

func (o ReactionCollectorOptions) validate() error {
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

// ReactionCollector receives matching reaction snapshots until it is stopped,
// reaches a limit, or one of its automatic limits fires.
type ReactionCollector struct {
	options ReactionCollectorOptions
	core    *collectorCore[*events.MessageReactionsEvent]
}

func NewReactions(ctx, lifecycleCtx context.Context, options ReactionCollectorOptions, register func(func(context.Context, *events.MessageReactionsEvent) error) func()) (*ReactionCollector, error) {
	if err := options.validate(); err != nil {
		return nil, err
	}
	collector := &ReactionCollector{options: options}
	collector.core = newCollectorCore(ctx, lifecycleCtx, collectorCoreOptions{
		Time: options.Time, Idle: options.Idle, Max: options.Max, Buffer: options.Buffer,
	}, collector.matches, register)
	return collector, nil
}

func (c *ReactionCollector) matches(event *events.MessageReactionsEvent) bool {
	if event == nil {
		return false
	}
	if c.options.Chat != (types.ChatRef{}) && event.Chat != c.options.Chat {
		return false
	}
	if c.options.MessageID != 0 && event.MessageID != c.options.MessageID {
		return false
	}
	return c.options.Filter == nil || c.options.Filter(event)
}

func (c *ReactionCollector) Events() <-chan *events.MessageReactionsEvent {
	if c == nil || c.core == nil {
		return nil
	}
	return c.core.events
}

func (c *ReactionCollector) Done() <-chan struct{} {
	if c == nil || c.core == nil {
		return nil
	}
	return c.core.done
}

func (c *ReactionCollector) Result() CollectorResult {
	if c == nil || c.core == nil {
		return CollectorResult{Reason: EndReasonStopped, Err: ErrCollectorEnded}
	}
	c.core.mu.Lock()
	defer c.core.mu.Unlock()
	return c.core.result
}

func (c *ReactionCollector) Stop(reason EndReason) {
	if c != nil && c.core != nil {
		c.core.finish(reason, nil)
	}
}

func (c *ReactionCollector) Next(ctx context.Context) (*events.MessageReactionsEvent, error) {
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
