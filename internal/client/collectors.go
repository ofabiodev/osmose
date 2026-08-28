package client

import (
	"context"
	"fmt"

	"github.com/ofabiodev/osmose/collectors"
	"github.com/ofabiodev/osmose/events"
)

type EndReason = collectors.EndReason

const (
	EndReasonTime     = collectors.EndReasonTime
	EndReasonIdle     = collectors.EndReasonIdle
	EndReasonLimit    = collectors.EndReasonLimit
	EndReasonStopped  = collectors.EndReasonStopped
	EndReasonContext  = collectors.EndReasonContext
	EndReasonOverflow = collectors.EndReasonOverflow
	EndReasonClosed   = collectors.EndReasonClosed
)

type CollectorError = collectors.CollectorError
type CollectorResult = collectors.CollectorResult
type MessageCollectorOptions = collectors.MessageCollectorOptions
type MessageCollector = collectors.MessageCollector
type InteractionCollectorOptions = collectors.InteractionCollectorOptions
type InteractionCollector = collectors.InteractionCollector
type ReactionCollectorOptions = collectors.ReactionCollectorOptions
type ReactionCollector = collectors.ReactionCollector

var (
	ErrCollectorEnded    = collectors.ErrCollectorEnded
	ErrCollectorTimeout  = collectors.ErrCollectorTimeout
	ErrCollectorIdle     = collectors.ErrCollectorIdle
	ErrCollectorOverflow = collectors.ErrCollectorOverflow
	ErrCollectorClosed   = collectors.ErrCollectorClosed
)

func (c *Client) collectorLifecycle() (context.Context, error) {
	if c == nil || c.events == nil {
		return nil, ErrClosed
	}
	c.runMu.Lock()
	defer c.runMu.Unlock()
	if c.closed {
		return nil, ErrClosed
	}
	if c.runFinished {
		return nil, ErrRunCompleted
	}
	return c.lifecycleCtx, nil
}

func (c *Client) collectMessages(ctx context.Context, options MessageCollectorOptions) (*MessageCollector, error) {
	lifecycleCtx, err := c.collectorLifecycle()
	if err != nil {
		return nil, err
	}
	return collectors.NewMessages(ctx, lifecycleCtx, options, func(handler func(context.Context, *events.MessageCreateEvent) error) func() {
		return c.OnMessageCreate(handler)
	})
}

// CollectMessages starts a bounded message collector. Use Stop when no Time,
// Idle, or Max limit is configured.
func (c *Client) CollectMessages(options MessageCollectorOptions) (*MessageCollector, error) {
	return c.collectMessages(context.Background(), options)
}

// CollectMessagesContext starts a bounded message collector tied to ctx and
// the client lifecycle.
func (c *Client) CollectMessagesContext(ctx context.Context, options MessageCollectorOptions) (*MessageCollector, error) {
	return c.collectMessages(ctx, options)
}

// AwaitMessage waits for the first message matching options. Its Max value is
// always one; use CollectMessages when multiple messages are needed.
func (c *Client) AwaitMessage(ctx context.Context, options MessageCollectorOptions) (*MessageCreateEvent, error) {
	if options.Max < 0 {
		return nil, fmt.Errorf("collector max cannot be negative")
	}
	options.Max = 1
	collector, err := c.collectMessages(ctx, options)
	if err != nil {
		return nil, err
	}
	return collector.Next(ctx)
}

func (c *Client) collectInteractions(ctx context.Context, options InteractionCollectorOptions) (*InteractionCollector, error) {
	lifecycleCtx, err := c.collectorLifecycle()
	if err != nil {
		return nil, err
	}
	return collectors.NewInteractions(ctx, lifecycleCtx, options, func(handler func(context.Context, *events.InteractionEvent) error) func() {
		return c.OnInteraction(handler)
	})
}

// CollectInteractions starts a bounded interaction collector.
func (c *Client) CollectInteractions(options InteractionCollectorOptions) (*InteractionCollector, error) {
	return c.collectInteractions(context.Background(), options)
}

// CollectInteractionsContext starts an interaction collector tied to ctx and
// the client lifecycle.
func (c *Client) CollectInteractionsContext(ctx context.Context, options InteractionCollectorOptions) (*InteractionCollector, error) {
	return c.collectInteractions(ctx, options)
}

// AwaitInteraction waits for the first interaction matching options.
func (c *Client) AwaitInteraction(ctx context.Context, options InteractionCollectorOptions) (*InteractionEvent, error) {
	if options.Max < 0 {
		return nil, fmt.Errorf("collector max cannot be negative")
	}
	options.Max = 1
	collector, err := c.collectInteractions(ctx, options)
	if err != nil {
		return nil, err
	}
	return collector.Next(ctx)
}

func (c *Client) collectReactions(ctx context.Context, options ReactionCollectorOptions) (*ReactionCollector, error) {
	lifecycleCtx, err := c.collectorLifecycle()
	if err != nil {
		return nil, err
	}
	return collectors.NewReactions(ctx, lifecycleCtx, options, func(handler func(context.Context, *events.MessageReactionsEvent) error) func() {
		return c.OnMessageReactions(handler)
	})
}

// CollectReactions starts a bounded reaction snapshot collector.
func (c *Client) CollectReactions(options ReactionCollectorOptions) (*ReactionCollector, error) {
	return c.collectReactions(context.Background(), options)
}

// CollectReactionsContext starts a reaction collector tied to ctx and the
// client lifecycle.
func (c *Client) CollectReactionsContext(ctx context.Context, options ReactionCollectorOptions) (*ReactionCollector, error) {
	return c.collectReactions(ctx, options)
}

// AwaitReaction waits for the first matching reaction snapshot.
func (c *Client) AwaitReaction(ctx context.Context, options ReactionCollectorOptions) (*MessageReactionsEvent, error) {
	if options.Max < 0 {
		return nil, fmt.Errorf("collector max cannot be negative")
	}
	options.Max = 1
	collector, err := c.collectReactions(ctx, options)
	if err != nil {
		return nil, err
	}
	return collector.Next(ctx)
}
