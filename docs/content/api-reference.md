---
title: API reference
description: Public Osmose types, client methods, events, services, and errors.
group: Reference
order: 2
layout: doc
---

This page is a quick reference for the stable, bot-facing part of Osmose.
Generated protobuf packages remain available under `proto/` for advanced use.

## Client

Create a client with the two required settings:

```go
client, err := osmose.New(osmose.Config{
	Token:    os.Getenv("OSMIUM_TOKEN"),
	ClientID: 123456,
})
```

`New` validates the configuration and applies defaults. `Client` exposes these
services:

| Field | Package | Main operations |
| --- | --- | --- |
| `Messages` | `messages` | `Send`, `Reply`, `History`, `Search`, `PinnedMessages`, `UnreadMentions`, `Edit`, `Delete` |
| `Chats` | `chats` | `List`, `Get`, `Members`, `SetTyping` |
| `Communities` | `communities` | `List`, `Channels`, `ChannelMembers` |
| `Users` | `users` | `Get`, `Profile` |
| `Reactions` | `reactions` | `Add`, `Remove` |
| `Voice` | `voice` | `RequestRoom`, `RoomStates`, `DisconnectUser` |
| `Managers` | `types` (also aliased from `osmose`) | `Users`, `Communities`, `Channels`, `Members`, `Roles`, `Messages`, `Clear` |

## Lifecycle

| Method | Purpose |
| --- | --- |
| `Run(ctx)` | Connect, perform the handshake, dispatch events, reconnect, and shut down with `ctx`. |
| `Close()` | Stop the active connection and make `Run` finish. |
| `Shutdown(ctx)` | Close the client and wait for run and event-dispatch cleanup. |
| `Done()` | Channel closed after the client lifecycle and cleanup finish. |
| `WaitReady(ctx)` | Wait until the client has completed authorization. |
| `State()` | Read the current `Disconnected`, `Connecting`, `Initializing`, `Authenticating`, `Ready`, or `Closing` state. |
| `User()` | Read the current authenticated user, when available. |
| `SessionID()` | Read the current session ID. |

`Run` handles the normal lifecycle. Network failures reconnect with bounded
backoff. Known rejected-authorization failures and protocol mismatches are
permanent errors; other authorization RPC failures remain retryable. Reconnects
happen inside a single `Run`; after it finishes, the client cannot be run again.

## Events

All typed event handlers have this shape:

```go
func(context.Context, *Event) error
```

Registration returns a function that removes that handler.

| Registration method | Event payload |
| --- | --- |
| `OnConnecting` | `ConnectionEvent` before a connection attempt |
| `OnConnected` | `ConnectionEvent` after the WebSocket opens |
| `OnReady` | `ReadyEvent` |
| `OnDisconnected` | `ConnectionEvent` after an attempt ends |
| `OnReconnecting` | `ConnectionEvent` before a retry, with `RetryIn` |
| `OnConnectionError` / `OnError` | `ConnectionEvent` with `Err` |
| `OnMessageCreate` | `MessageCreateEvent` |
| `OnMessage` | Direct `*types.Message` on create |
| `OnMessageUpdate` | `MessageUpdateEvent` |
| `OnMessageEdit` | Direct `*types.Message` on update |
| `OnMessageDelete` | `MessageDeleteEvent` |
| `OnChannelUpdate` | `ChannelUpdateEvent` |
| `OnChannelDelete` | `ChannelDeleteEvent` |
| `OnUserUpdate` | `UserUpdateEvent` |
| `OnCommunityUpdate` | `CommunityUpdateEvent` |
| `OnCommunityDelete` | `CommunityDeleteEvent` |
| `OnChatTyping` | `ChatTypingEvent` |
| `OnMemberCreate` | `MemberCreateEvent` |
| `OnMemberUpdate` | `MemberUpdateEvent` |
| `OnMemberDelete` | `MemberDeleteEvent` |
| `OnMessageReactions` | `MessageReactionsEvent` |
| `OnConversationLastRead` | `ConversationLastReadEvent` |
| `OnInteraction` | `InteractionEvent` |
| `OnVoiceRoomState` | `VoiceRoomStateEvent` |
| `OnVoiceRoomParticipant` | `VoiceRoomParticipantEvent` |
| `OnUpdate` | `UpdateEvent` with the raw generated update |

Message create and interaction events provide a `Reply(ctx, content)` helper.
Interaction events also provide `Respond`, `Acknowledge`, and `Defer`. Every
event provides `Client()`. `ConnectionEvent` contains `Attempt`, `State`,
`RetryIn`, and `Err`.

Handler errors and panics are logged. Configure `Config.OnHandlerError` to
observe them programmatically; it runs on the event worker after the handler
and must not block.

Event dispatch is bounded. When the queue is full, the update is dropped so
the socket reader remains responsive. `Client.DroppedEvents()` returns the
cumulative count and `Config.OnEventOverflow` receives that count after each
drop. The overflow callback runs on the socket reader and must not block.
When caching is enabled, state has already been applied before that delivery drop.

## Collectors

| Method or type | Purpose |
| --- | --- |
| `CollectMessages(options)` | Start a multi-message collector. |
| `CollectMessagesContext(ctx, options)` | Start a collector tied to a context and the client lifecycle. |
| `AwaitMessage(ctx, options)` | Wait for one matching message. |
| `MessageCollector.Events()` | Read matching events until close. |
| `MessageCollector.Next(ctx)` | Read one matching event. |
| `MessageCollector.Done()` | Wait for collector completion. |
| `MessageCollector.Result()` | Read count, end reason, and termination error. |
| `MessageCollector.Stop(reason)` | Stop explicitly and remove its handler. |
| `CollectInteractions(options)` | Start a multi-interaction collector. |
| `CollectInteractionsContext(ctx, options)` | Start an interaction collector tied to a context. |
| `AwaitInteraction(ctx, options)` | Wait for one matching interaction. |
| `InteractionCollector.Events()` / `Next(ctx)` | Read matching interactions. |
| `InteractionCollector.Done()` / `Result()` / `Stop(reason)` | Observe or stop an interaction collector. |
| `CollectReactions(options)` / `AwaitReaction(ctx, options)` | Collect protocol reaction-state updates. |

`MessageCollectorOptions` supports `Chat`, `AuthorID`, `Filter`, `Time`,
`Idle`, `Max`, and `Buffer`. The collector uses the existing bounded event
workers and never creates a goroutine per incoming message.

`InteractionCollectorOptions` supports `UserID`, `MessageID`, `Data`,
`Filter`, `Time`, `Idle`, `Max`, and `Buffer`. `ReactionCollectorOptions`
supports `Chat`, `MessageID`, `Filter`, `Time`, `Idle`, `Max`, and `Buffer`.

## Public models

The `types` package contains small models shared by events and services:

| Type | Use |
| --- | --- |
| `types.ID` | Osmium wire ID with an explicit `Uint64()` conversion. |
| `types.User` | User identity, username, status, photo, bot flag, and raw value. |
| `types.Message` | Message ID, chat, author, content, `ReplyInfo`, media, entities, bot info, and raw value. |
| `types.ChatRef` | Self, user, group, or community channel reference. |
| `types.ChannelRef` | Community channel reference. |
| `types.UserRef` | User reference for profile operations. |
| `types.Conversation` | Chat state and unread/read markers. |
| `types.Group` | Group identity and participant IDs. |
| `types.Channel` | Community channel metadata. |
| `types.Community` | Community identity, permissions, and notification preferences. |
| `types.CommunityMember` | Community membership and roles. |
| `types.CommunityRole` | Community role metadata and permission operations. |
| `types.Emoji` | Unicode or custom reaction emoji. |
| `types.ChatMember` | Chat membership and optional permissions. |
| `types.MemberListEntry` / `types.MemberListDivider` | Ordered community-channel member-list entries. |
| `types.Interaction` | Interaction IDs, action data, and the raw update value. |
| `types.MessageButton` / `types.MessageButtons` | Protocol button actions organized into rows. |
| `types.MessageBotInfo` | Optional message cloak and buttons. |
| `types.MessageQuote` / `types.MessageReply` | Rich reply metadata. |
| `types.ChatPhoto` | Photo file ID and preview bytes. |
| `types.UserStatus` / `types.UserActivity` | Presence and activity data. |
| `types.PermissionOverrides` | Positive and negative permission masks. |
| `types.MediaRef` / `types.UploadedFileRef` | Stable outbound media references. |

Use constructors for chat references:

```go
types.SelfChat()
types.UserChat(userID)
types.GroupChat(groupID)
types.ChannelChat(communityID, channelID)
```

Service responses retain the original generated value in `Raw` where a
response has a wrapper model. This keeps common code typed while preserving
access to advanced protocol fields when needed.

## Rich objects

`Community`, `Channel`, `Message`, `CommunityMember` (also named `Member`), and
`CommunityRole` (also named `Role`) can perform common operations directly.
Objects returned by `Communities`, `Chats`, `Messages`, and typed events are
bound to the client automatically. Their methods accept `context.Context` and
hide request construction; use `Raw` for unsupported protocol operations.

`Message.ReplyInfo` contains reply metadata. It is named `ReplyInfo` because
`Message.Reply(ctx, content)` is the object reply method.

## Managers and partial objects

`Client.Managers` provides `UserManager`, `CommunityManager`, `ChannelManager`,
`MemberManager`, `RoleManager`, and `MessageManager`. Use `In(communityID)` for
channels/members/roles and `In(chatRef)` for messages. `Community.Collections()`
and `Channel.Collections()` provide scoped views without changing existing methods.

`Get(id)` returns a cached snapshot and a found flag. `Resolve(ctx, id)` uses a
complete cache hit, while `Fetch(ctx, id)` always requests fresh data. Network
operations return errors. `ListCached`, `Invalidate(id)`, and `Clear` perform no
I/O. See the full [manager operation table](../state-management/#managers-and-collections).

`Ref(id)` makes a partial object. All six entity types expose `Partial` and
`Fetch(ctx) error`; object Fetch refreshes the receiver only on success. Cache
snapshots are isolated from caller mutations. Services and known raw operations
share cache synchronization; unsupported raw mutations need manual invalidation.

## Errors

Use `errors.Is` and `errors.As` instead of matching error strings.

| Error | Meaning |
| --- | --- |
| `osmose.ErrClosed` | The client is closed or shutting down. |
| `osmose.ErrNotConnected` | No active connection is available. |
| `osmose.ErrDisconnected` | An active connection was lost. |
| `osmose.ErrNotReady` | A normal request was made before authorization completed. |
| `osmose.ErrAlreadyRunning` | `Run` is already active. |
| `osmose.ErrRunCompleted` | The client's single lifecycle has finished. |
| `osmose.ErrPermanent` | The error must not be retried by `Client.Run`. |
| `osmose.ErrAuthorizationFailed` | The server rejected authorization. |
| `osmose.ErrProtocolMismatch` | The server response does not match the current protocol contract. |
| `osmose.ErrCollectorEnded` | A collector has no more events. |
| `osmose.ErrCollectorTimeout` | A collector reached its `Time` limit. |
| `osmose.ErrCollectorIdle` | A collector reached its `Idle` limit. |
| `osmose.ErrCollectorOverflow` | A collector buffer filled before it was consumed. |
| `osmose.ErrCollectorClosed` | The client closed while the collector was active. |
| `osmose.ErrEventQueueFull` | The bounded event queue could not accept an update. |
| `types.ErrNotFound` | A valid response did not contain the requested entity. |
| `types.ErrIncompleteObject` | An operation needs complete object data. |
| `types.ErrObjectClientUnavailable` | A rich object is not bound to a usable client. |

```go
if errors.Is(err, osmose.ErrPermanent) {
	// Fix credentials or the protocol/client version before retrying.
}

var rpcErr *osmose.RPCError
if errors.As(err, &rpcErr) {
	log.Printf("RPC %d: %s", rpcErr.Code, rpcErr.Message)
}
```

`RPCError` preserves the server code, message, and optional trace ID.

## Raw API

When a high-level service does not cover an endpoint, call it with a generated
protobuf request:

```go
import protoCommunities "github.com/ofabiodev/osmose/proto/communities"

result, err := client.Raw().Call(ctx, &protoCommunities.GetCommunities{})
if err != nil {
	return err
}

communities := result.GetCommunities()
```

The raw API is intentionally separate from the common service API. Request
wrapping is generated and runtime dispatch does not use reflection.

## Configuration

`Config` requires `Token` and `ClientID`. Optional fields include:

| Field | Purpose |
| --- | --- |
| `ServerURL` | Override the Osmium WebSocket endpoint. |
| `Logger` | Set a `log/slog` logger. |
| `RequestTimeout` | Default timeout for RPC calls. |
| `Cache` | Opt-in `CacheConfig`: `Enabled`, `TTL`, and limits for `Users`, `Communities`, `Channels`, `Members`, `Roles`, `Messages`. |
| `HeartbeatInterval` | Keepalive interval. |
| `EventQueue`, `EventWorkers` | Bounded event dispatch capacity. `EventWorkers` defaults to one for predictable ordering. |
| `OnHandlerError` | Observe errors and panics returned by event handlers. |
| `OnEventOverflow` | Observe cumulative drops when the event queue is full. |
| `RequestInterval` | Optional minimum interval between outbound RPC requests. |
| `WriteQueue`, `WriteTimeout` | Controlled outbound write capacity. |
| `BackoffMin`, `BackoffMax` | Reconnect backoff bounds. |

Zero values use sensible defaults.
