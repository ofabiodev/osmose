---
title: Message collectors
description: Wait for matching messages with filters and bounded lifetimes.
group: Guides
order: 2
layout: doc
---

Collectors are a small convenience layer over `OnMessageCreate`. They observe
matching events without preventing other message handlers from running.

## Wait for one message

Use `AwaitMessage` for forms, confirmations, and other one-response flows:

```go
event, err := client.AwaitMessage(ctx, osmose.MessageCollectorOptions{
	Chat:     chat,
	AuthorID: userID,
	Time:     60 * time.Second,
	Filter: func(event *osmose.MessageCreateEvent) bool {
		return event.Message.Content != ""
	},
})
if err != nil {
	return err
}

return event.Reply(ctx, "Recebi: "+event.Message.Content)
```

`AwaitMessage` automatically stops after the first matching message.

## Collect several messages

```go
collector, err := client.CollectMessages(osmose.MessageCollectorOptions{
	Chat:   chat,
	Time:   2 * time.Minute,
	Idle:   30 * time.Second,
	Max:    5,
	Buffer: 16,
})
if err != nil {
	return err
}
defer collector.Stop(osmose.EndReasonStopped)

for event := range collector.Events() {
	log.Println(event.Message.Content)
}

result := collector.Result()
log.Println(result.Collected, result.Reason)
```

`Events()` is closed when the collector ends. `Done()` can be used when the
consumer does not need to range over events. `Next(ctx)` waits for one event
and respects a separate context. Use `CollectMessagesContext(ctx, options)`
when a multi-message collector belongs to a request or interaction lifetime.

## Interaction collectors

Use `AwaitInteraction` for buttons or other interaction data emitted by the
current Osmium protocol:

```go
interaction, err := client.AwaitInteraction(ctx, osmose.InteractionCollectorOptions{
	UserID:    userID,
	MessageID: messageID,
	Data:      "confirm",
	Time:      time.Minute,
})
if err != nil {
	return err
}
return interaction.Reply(ctx, "Confirmed")
```

For multiple interactions, use `CollectInteractions` or
`CollectInteractionsContext`. The collector exposes the same `Events`,
`Next`, `Done`, `Result`, and `Stop` methods as a message collector.

`CollectReactions` and `AwaitReaction` are available for the protocol's
message-reaction snapshots. Filter them with `Chat`, `MessageID`, and a typed
`Filter`; the returned `MessageReactionsEvent` includes the message ID and the
current reaction state.

## Filters and limits

| Option | Behavior |
| --- | --- |
| `Chat` | Accept messages from one chat; the zero value accepts any chat. |
| `AuthorID` | Accept messages from one author; zero accepts any author. |
| `Filter` | Apply an additional typed predicate. |
| `Time` | Stop after the total duration. |
| `Idle` | Stop after no matching message for the duration. |
| `Max` | Stop after this many matching messages; zero is unlimited. |
| `Buffer` | Maximum queued matches; zero uses the default buffer. |

`InteractionCollectorOptions` uses `UserID`, `MessageID`, `Data`, and a typed
`Filter` in place of the message `Chat` and `AuthorID` fields. Its lifetime and
buffer options are the same.

`ReactionCollectorOptions` uses `Chat`, `MessageID`, `Filter`, `Time`, `Idle`,
`Max`, and `Buffer`.

The buffer is bounded. If it fills, the collector ends with
`ErrCollectorOverflow` instead of blocking the event dispatcher indefinitely.
Keep filters fast because they run in the existing bounded event worker pool.

## Form example

A form can be composed from `AwaitMessage` calls:

```go
func ask(ctx context.Context, client *osmose.Client, chat types.ChatRef, userID types.ID, prompt string) (string, error) {
	if _, err := client.Messages.Send(ctx, messages.SendParams{
		Chat:    chat,
		Content: prompt,
	}); err != nil {
		return "", err
	}

	event, err := client.AwaitMessage(ctx, osmose.MessageCollectorOptions{
		Chat:     chat,
		AuthorID: userID,
		Time:     2 * time.Minute,
	})
	if err != nil {
		return "", err
	}
	return event.Message.Content, nil
}
```

Collectors also stop automatically when the client closes. Use a context with
`AwaitMessage` for request-scoped flows; a client shutdown is reported as
`ErrCollectorClosed`. Add validation, retries, and application-specific state
around these helpers when a form needs them.

## End reasons

`CollectorResult.Reason` is one of `EndReasonTime`, `EndReasonIdle`,
`EndReasonLimit`, `EndReasonStopped`, `EndReasonContext`, or
`EndReasonOverflow`, or `EndReasonClosed`. Timeout, idle, overflow, and client
shutdown results expose typed errors for `errors.Is` checks.
