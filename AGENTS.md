# AGENTS.md

This repository is Osmose, the small and idiomatic Go SDK for Osmium bots.

## Core rules

- Read the existing code and the current schema before changing protocol code.
- Make the smallest change that removes real developer friction.
- Osmium proto is the protocol source of truth.
- Public API stability is more important than internal implementation stability.
- Prefer the smallest abstraction that removes real developer friction.
- Do not copy old examples blindly; verify field names and oneofs against the
  pinned `schema/` revision.

## Architecture

```text
schema/ (pinned upstream proto)
        ↓ go generate
proto/ generated Go messages
        ↓
internal/gateway → internal/rpc → Client → typed services/events/collectors
```

- `internal/gateway/` owns one WebSocket reader, one controlled writer, keepalive, and
  reconnect helpers.
- `internal/dispatcher/` owns the bounded event queue and worker lifecycle.
- `internal/rpc/` owns request IDs, pending calls, result correlation, and generated
  `ClientMessage` wrapping.
- `internal/scheduler/` owns the optional client-wide outbound request interval.
- `internal/client/` owns lifecycle, handshake, state, event registration, and
  service wiring behind the public facade.
- `events/` contains the typed event payloads and handler signatures.
- `collectors/` contains the bounded collector implementations used by the root
  client facade.
- `client.go` is the only production Go file in the root package and re-exports
  the stable public API from the internal implementation.
- `messages/`, `chats/`, `communities/`, `users/`, `reactions/`, and `voice/` are the
  small public bot-facing services.
- `proto/` is public generated protocol data for the raw escape hatch.

Do not expose WebSocket frames, request IDs, protobuf oneofs, locks, or
internal channels through normal service APIs. Advanced callers may use
`Client.Raw().Call` and the generated `proto/` packages.

## Clone and schema checkout

The protocol source is a real Git submodule. Clone the repository with it:

```bash
git clone --recurse-submodules https://github.com/ofabiodev/osmose.git
```

For an existing checkout, run:

```bash
git submodule update --init --recursive
```

`schema/` must remain pinned to the commit recorded by the superproject. Do
not replace it with copied `.proto` files. Update it intentionally with
`git submodule update --remote schema`, then regenerate and review the
generated changes.

## Protocol and generated files

The checked-in protobuf packages are generated from the pinned `schema/`
submodule. Run this after updating the submodule or changing generation tools:

```bash
go generate ./...
```

Never manually edit:

- `proto/*/*.pb.go`
- `internal/rpc/wrap_gen.go`

Generated code must be deterministic, gofmt-formatted, and marked `DO NOT
EDIT`. Do not add runtime reflection, AST parsing, filesystem scans, plugins,
or dynamic loading to the SDK.

## API and concurrency

- Keep `context.Context` on every operation that can block.
- Keep the socket reader independent from user handlers.
- The default event worker count is one, so event order is predictable. More
  workers are opt-in and may run handlers concurrently.
- Do not add unbounded goroutines or unbounded channels.
- Preserve the single-writer ownership rule.
- Fail pending RPCs when a connection dies.
- Treat `Client.Run` as a single lifecycle; reconnect inside it, and use
  `Shutdown(ctx)` when a caller must wait for cleanup.
- Keep event delivery bounded. Drops must remain observable through the public
  counter/callback instead of being silently hidden.
- Route handler errors and panics through the configured error hook as well as
  the logger.
- Collectors must stop when their context or the client lifecycle ends.
- Validate public service parameters before creating an RPC request.
- Keep request scheduling opt-in and small. The current schema has no
  rate-limit buckets or retry-after metadata; do not invent automatic retries.
- Add an exported API only after checking that a bot developer needs it and
  that a smaller stable shape is not enough.
- Do not add a command framework, auto-registration, manifests, or platform
  concepts to Osmose; those belong in a bot template above the SDK.
- Do not add a cache or broad endpoint coverage without a concrete bot use
  case.
- Voice currently means Osmium's room control plane. Do not add an audio or
  WebRTC abstraction until the protocol provides the required transport.

Top-level bot models in `types/` are stable SDK structs with a `Raw` protobuf
escape hatch. Keep small nested protocol values as aliases when wrapping them
would only duplicate oneofs or enums.

## Verification

Before finishing a Go change, run:

```bash
make check
make build
```

For protocol or concurrency changes, also inspect relevant benchmarks and run
`go test -race ./...` directly when needed. Do not claim a real Osmium
integration works unless it was run with valid credentials.

Review the diff, remove temporary code, and update generated files through
`go generate`, never by hand.
