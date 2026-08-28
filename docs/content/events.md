---
title: Events
description: Handle typed Osmium updates with removable Go handlers.
group: Guides
order: 1
layout: doc
---

Register strongly typed handlers on the client:

```go
client.OnReady(func(_ context.Context, event *osmose.ReadyEvent) error {
	log.Printf("connected as %s", event.User.Username)
	return nil
})

client.OnMessageCreate(func(ctx context.Context, event *osmose.MessageCreateEvent) error {
	if event.Message.Content == "!ping" {
		return event.Reply(ctx, "Pong!")
	}
	return nil
})
```

## Available events

| Handler | Event |
| --- | --- |
| `OnConnecting` | A connection attempt is starting |
| `OnConnected` | The WebSocket transport is open and handshake is starting |
| `OnReady` | Authorization completed and the session is ready |
| `OnDisconnected` | A connection attempt or active connection ended |
| `OnReconnecting` | The client is waiting before another connection attempt |
| `OnConnectionError` / `OnError` | A connection attempt failed |
| `OnMessageCreate` | A message was created |
| `OnMessageUpdate` | A message was updated |
| `OnMessageDelete` | One or more messages were deleted |
| `OnChannelUpdate` | A community channel was updated |
| `OnChannelDelete` | A community channel was deleted |
| `OnUserUpdate` | A user was updated |
| `OnCommunityUpdate` | A community was updated |
| `OnCommunityDelete` | A community was deleted |
| `OnChatTyping` | A user started or stopped typing |
| `OnMemberCreate` | A community member was created |
| `OnMemberUpdate` | A community member was updated |
| `OnMemberDelete` | A community member was deleted |
| `OnMessageReactions` | Message reactions changed |
| `OnConversationLastRead` | A conversation read marker changed |
| `OnInteraction` | An interaction update was received |
| `OnVoiceRoomState` | A complete voice room state changed |
| `OnVoiceRoomParticipant` | A voice room participant changed |
| `OnUpdate` | The raw generated update for advanced handling |

Every handler receives a `context.Context` and returns an `error`. Registration
returns a removal function:

```go
remove := client.OnMessageUpdate(handler)
defer remove()
```

Message events expose `Message`, `Author`, `Client()`, and `Reply(...)` where
appropriate. Interaction events expose the typed `Interaction` model,
`Respond`, `Reply`, `Acknowledge`, and `Defer`. The underlying protobuf value
remains available through `Raw` on the model. Event payload fields are derived
from the current Osmium update types, so absent optional protocol fields remain
nil where relevant.

Connection events use `ConnectionEvent`:

```go
client.OnReconnecting(func(_ context.Context, event *osmose.ConnectionEvent) error {
	log.Printf("retrying connection %d in %s", event.Attempt, event.RetryIn)
	return nil
})
```

`Attempt` starts at one. `RetryIn` is populated for `OnReconnecting`, and
`Err` is populated for failed connection attempts. Voice events expose the
room control-plane models; they do not carry audio packets.

Configure `Config.OnHandlerError` to observe returned handler errors and
panics. The callback runs on the event worker and should not block.

## Dispatch behavior

Updates are delivered through a bounded worker pool with one worker by
default, so a slow handler cannot create an unbounded number of goroutines.
Set `EventWorkers` above one when handlers may run concurrently. If the queue
is full, the update is dropped and logged so the client stays responsive.
`Client.DroppedEvents()` exposes the cumulative drop count and
`Config.OnEventOverflow` can report it to application metrics or alerts. Keep
the overflow callback short.

For waiting on messages inside a confirmation or form flow, use the typed
[message collector](/collectors/) instead of managing a temporary handler by
hand.
