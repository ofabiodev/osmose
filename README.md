<p align="center">
  <img src="https://raw.githubusercontent.com/ofabiodev/osmose/refs/heads/main/.github/assets/logo.svg" align="center" width="200" alt="docshelf logo">
  <h1 align="center">Osmose</h1>
  <p align="center">A fast and idiomatic Go SDK for building bots on the Osmium protocol.</p>
</p>
<br>

<p align="center">
  <a href="https://github.com/ofabiodev/osmose/actions/workflows/ci_test.yml" rel="nofollow"><img alt="CI" src="https://github.com/ofabiodev/osmose/actions/workflows/ci_test.yml/badge.svg"></a>
  <a href="https://github.com/ofabiodev/osmose/releases/latest" rel="nofollow"><img alt="Latest release" src="https://img.shields.io/github/v/release/ofabiodev/osmose"></a>
  <a href="https://opensource.org/licenses/MIT" rel="nofollow"><img alt="License" src="https://img.shields.io/badge/license-MIT-brightgreen"></a>
  <a href="https://pkg.go.dev/github.com/ofabiodev/osmose" rel="nofollow"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/ofabiodev/osmose.svg"></a>
</p>
<div align="center">
  <a href="https://osm.pm/c/osmose">▪ Osmium Community ▪</a>
</div>
<br/>

## Why Osmose?

Osmose is a small, typed Go SDK for creating bots on the Osmium protocol.

The SDK hides WebSocket frames, binary Protocol Buffers, request correlation,
keepalive, reconnect, and shutdown behind a straightforward client API. The
generated protocol packages remain available for advanced integrations.

Read the [Osmose documentation](https://ofabiodev.github.io/osmose/) for the
complete guide.

## Installation

Add Osmose to a Go module:

```bash
go get github.com/ofabiodev/osmose
```

Generated protocol packages are included, so installing Osmose does not
require `protoc`.

## Development checkout

Clone the repository with its pinned Osmium protocol schema:

```bash
git clone --recurse-submodules https://github.com/ofabiodev/osmose.git
cd osmose
```

If the repository was cloned without submodules, initialize the schema with:

```bash
git submodule update --init --recursive
```

The schema submodule is used to regenerate protocol code. The generated Go
packages are committed to this repository, so installing Osmose does not
require the protobuf toolchain.

<table>
  <tr>
    <td>
      <strong>Transparency</strong><br>
      Transparency is a core pillar of my projects. AI is used as a supporting
      tool where it helps, mainly for code completion and translating technical
      documentation. The architecture, decisions, review, testing, and responsibility
      for the released code remain with the project owner.
    </td>
  </tr>
</table>

## Features

| Area          | Status | Details                                                                                                      |
| ------------- | :----: | ------------------------------------------------------------------------------------------------------------ |
| Client        |   ✅   | Small central client with sensible defaults and typed services                                               |
| Gateway       |   ✅   | Binary protobuf over WebSocket with controlled reads, writes, and keepalive                                  |
| Lifecycle     |   ✅   | Connect, initialize, authorize, ready, reconnect, and graceful shutdown                                      |
| RPC           |   ✅   | Fast request correlation, context cancellation, timeouts, and typed errors                                   |
| Events        |   ✅   | Strongly typed handlers for connection, messages, users, communities, members, interactions, and voice state |
| Collectors    |   ✅   | Typed message, interaction, and reaction collectors with filters and bounded lifetimes                       |
| Messages      |   ✅   | Send, reply, edit, delete, history, search, pinned messages, mentions, media, and buttons                    |
| Services      |   ✅   | Messages, chats, communities, users, reactions, and voice control-plane operations                           |
| Models        |   ✅   | Small public models with useful fields and raw protocol access when needed                                   |
| Raw API       |   ✅   | Generated protobuf escape hatch for endpoints not wrapped by a service                                       |
| Safety        |   ✅   | Bounded event and write queues, cancellation, reconnect backoff, and race-tested concurrency                 |
| Documentation |   ✅   | Public documentation site built with [docshelf](https://github.com/ofabiodev/docshelf)                       |

## Quick start

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/ofabiodev/osmose"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client, err := osmose.New(osmose.Config{
		Token:    os.Getenv("OSMIUM_TOKEN"),
		ClientID: 123456,
	})
	if err != nil {
		log.Fatal(err)
	}

	client.OnReady(func(_ context.Context, event *osmose.ReadyEvent) error {
		log.Printf("connected as %s", event.User.Username)
		return nil
	})

	client.OnMessageCreate(func(ctx context.Context, event *osmose.MessageCreateEvent) error {
		if event.Message.Content != "!ping" {
			return nil
		}
		return event.Reply(ctx, "Pong! 🏓")
	})

	if err := client.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
```

Set the token before running the bot:

```bash
export OSMIUM_TOKEN="your-bot-token"
go run .
```

On PowerShell:

```powershell
$env:OSMIUM_TOKEN = "your-bot-token"
go run .
```

`Run` manages the connection, handshake, event dispatch, keepalive,
reconnect, and shutdown for the client.

## Events

Handlers are strongly typed, return an error, and can be removed:

```go
remove := client.OnMessageUpdate(func(_ context.Context, event *osmose.MessageUpdateEvent) error {
	log.Printf("message %d changed", event.Message.ID)
	return nil
})
defer remove()

client.OnInteraction(func(ctx context.Context, event *osmose.InteractionEvent) error {
	if event.Data == "confirm" {
		return event.Reply(ctx, "Confirmed")
	}
	return event.Defer(ctx)
})
```

Typed events include connection lifecycle, ready, message, channel, user,
community, typing, member, reaction, read-marker, interaction, and voice room
updates. Use `OnUpdate` when an application needs the raw generated update.

Event delivery is bounded, so a slow handler cannot create an unlimited number
of goroutines. Set `EventWorkers` above one when handlers may run concurrently.
`DroppedEvents` and `OnEventOverflow` make queue drops observable.

## Collectors

Collectors are useful for confirmations, forms, and multi-step conversations:

```go
event, err := client.AwaitMessage(ctx, osmose.MessageCollectorOptions{
	Chat:     chat,
	AuthorID: userID,
	Time:     time.Minute,
})
if err != nil {
	return err
}

return event.Reply(ctx, "Recebi: "+event.Message.Content)
```

Use `CollectMessages` when more than one matching message is needed. Message,
interaction, and reaction collectors support typed filters, maximum counts,
idle timeouts, total timeouts, cancellation, and bounded buffers.

See the [collector guide](docs/content/collectors.md) for a complete form flow.

## Sending messages

Use parameter structs instead of constructing protobuf requests:

```go
import (
	"github.com/ofabiodev/osmose/messages"
	"github.com/ofabiodev/osmose/types"
)

sent, err := client.Messages.Send(ctx, messages.SendParams{
	Chat:    types.SelfChat(),
	Content: "Choose an action:",
})
if err != nil {
	return err
}

log.Printf("sent message %d", sent.ID)
```

Replying to a message event is shorter:

```go
return event.Reply(ctx, "Pong!")
```

Common chat references are `types.SelfChat()`, `types.UserChat(id)`,
`types.GroupChat(id)`, and `types.ChannelChat(communityID, channelID)`.

## Services

| Service       | Operations                                                                                 |
| ------------- | ------------------------------------------------------------------------------------------ |
| `Messages`    | `Send`, `Reply`, `History`, `Search`, `PinnedMessages`, `UnreadMentions`, `Edit`, `Delete` |
| `Chats`       | `List`, `Get`, `Members`, `SetTyping`                                                      |
| `Communities` | `List`, `Channels`, `ChannelMembers`                                                       |
| `Users`       | `Get`, `Profile`                                                                           |
| `Reactions`   | `Add`, `Remove`                                                                            |
| `Voice`       | `RequestRoom`, `RoomStates`, `DisconnectUser`                                              |

Every network operation accepts `context.Context`.

## Models

The `types` package contains the public models shared by services and events:

| Type                                              | Use                                                                  |
| ------------------------------------------------- | -------------------------------------------------------------------- |
| `types.ID`                                        | Explicit Osmium identifier type                                      |
| `types.User`                                      | User identity, status, photo, and bot information                    |
| `types.Message`                                   | Message content, author, chat, replies, media, entities, and buttons |
| `types.ChatRef`                                   | Self, user, group, or community channel reference                    |
| `types.Conversation`                              | Chat state and read markers                                          |
| `types.Group`, `types.Channel`, `types.Community` | Conversation and community information                               |
| `types.CommunityMember`, `types.ChatMember`       | Membership and permissions                                           |
| `types.MemberListEntry`, `types.MemberListDivider` | Ordered community-channel member list entries                       |
| `types.Interaction`                               | Interaction IDs and action data                                      |

Models expose useful fields directly. Where it is useful for advanced code,
the original generated value remains available through `Raw`.

## Errors

Use `errors.Is` and `errors.As` instead of matching error strings:

```go
if errors.Is(err, osmose.ErrPermanent) {
	log.Fatal("the server rejected the connection permanently")
}

var rpcErr *osmose.RPCError
if errors.As(err, &rpcErr) {
	log.Printf("RPC %d: %s", rpcErr.Code, rpcErr.Message)
}
```

Osmose exposes typed errors for closed clients, connection state, permanent
authorization failures, protocol mismatches, RPC failures, and collector
termination reasons.

## Configuration

Only `Token` and `ClientID` are required:

```go
client, err := osmose.New(osmose.Config{
	Token:    token,
	ClientID: clientID,
})
```

Optional settings include:

| Setting                             | Purpose                                          |
| ----------------------------------- | ------------------------------------------------ |
| `ServerURL`                         | Use a different Osmium WebSocket endpoint        |
| `Logger`                            | Configure `log/slog` output                      |
| `RequestTimeout`                    | Set the default RPC timeout                      |
| `RequestInterval`                   | Add a minimum interval between outbound requests |
| `HeartbeatInterval`                 | Configure the session keepalive interval         |
| `EventQueue`, `EventWorkers`        | Control bounded event delivery                   |
| `OnHandlerError`, `OnEventOverflow` | Observe handler failures and dropped events      |
| `WriteQueue`, `WriteTimeout`        | Control outbound backpressure                    |
| `BackoffMin`, `BackoffMax`          | Bound reconnect delays                           |

Zero values use sensible defaults.

## Advanced / raw API

When a service does not cover an endpoint, use a generated protocol request:

```go
import protoCommunities "github.com/ofabiodev/osmose/proto/communities"

result, err := client.Raw().Call(ctx, &protoCommunities.GetCommunities{})
if err != nil {
	return err
}

communities := result.GetCommunities()
```

The generated protocol packages are included in the module and the raw API is
kept separate from the common service API.

## Protocol

Osmose follows Osmium's RPC-over-WebSocket protocol:

```text
WebSocket
  → binary protobuf ServerMessage
  → RPC result → waiting request
  → update → typed event handler
```

The connection handshake is:

```text
Connect → Initialize → Initialized → Authorize → Authorization → Ready
```

Connection failures are retried with bounded backoff. Pending requests are
completed when a connection ends, and reconnect performs the handshake again.
Osmose uses the current Osmium protocol rather than adding a REST layer.

## Documentation

The complete documentation is published at
[ofabiodev.github.io/osmose](https://ofabiodev.github.io/osmose/).

To preview the documentation locally:

```bash
cd docs
bun install
bun run docs:dev
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution and development
instructions.

## License

[MIT](LICENSE) © [ofabiodev](https://github.com/ofabiodev)
