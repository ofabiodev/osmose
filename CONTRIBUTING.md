# Contributing to Osmose

Thanks for helping improve Osmose. Keep changes focused, readable, idiomatic,
and easy to verify.

## Repository layout

| Folder | Purpose |
| --- | --- |
| `internal/gateway/` | WebSocket connection ownership, writes, keepalive, and reconnect helpers |
| `internal/dispatcher/` | Bounded event delivery and worker lifecycle |
| `internal/rpc/` | Request IDs, pending calls, result correlation, and generated wrappers |
| `internal/scheduler/` | Optional outbound request scheduling |
| `events/` | Typed event payloads and handler signatures |
| `collectors/` | Bounded message, interaction, and reaction collectors |
| `messages/`, `chats/`, `communities/`, `users/`, `reactions/`, `voice/` | Public bot-facing services |
| `proto/` | Generated Go protobuf packages used by the raw API |
| `schema/` | Pinned `osmiumchat/proto` source of truth |
| `tools/` | Cross-platform protobuf and wrapper generators |
| `docs/` | Documentation project and docshelf configuration |
| `docs/content/` | Markdown documentation source |
| `.github/workflows/` | CI and GitHub Pages workflows |

## Development setup

Requirements:

- Go version declared by `go.mod`;
- `protoc` and `protoc-gen-go` when changing or regenerating the schema;
- Bun and Node.js when changing or previewing the documentation site.

Clone the repository with its protocol schema:

```bash
git clone --recurse-submodules https://github.com/ofabiodev/osmose.git
cd osmose
```

For an existing checkout:

```bash
git submodule update --init --recursive
```

`schema/` is a pinned submodule of
[`osmiumchat/proto`](https://github.com/osmiumchat/proto). Keep it checked out
when running generation or changing protocol-facing code. The generated
protobuf files are already present, so ordinary SDK development does not need
the protobuf toolchain.

## Useful commands

| Command | Purpose |
| --- | --- |
| `make generate` | Regenerate protobuf and request-wrapper code |
| `make fmt` | Format Go packages |
| `make test` | Run unit tests |
| `make race` | Run tests with the race detector |
| `make vet` | Run `go vet` |
| `make check` | Run formatting, generation, tests, race tests, and vet |
| `make build` | Build-check all Go packages |
| `cd docs; bun install` | Install documentation dependencies |
| `cd docs; bun run docs:dev` | Start the local docshelf server |
| `cd docs; bun run docs:check` | Validate documentation |
| `cd docs; bun run docs:build` | Build the static documentation site |
| `cd docs; bun run docs:preview` | Preview an existing docs build |

The generated documentation output is written to `docs/dist/`.

## Protocol and generated files

Osmium proto is the protocol source of truth. Read the pinned files under
`schema/` before changing protocol-facing code. Do not copy old examples
blindly; verify field names and oneofs against the checked-out schema.

Never edit these files manually:

- `proto/*/*.pb.go`;
- `internal/rpc/wrap_gen.go`.

After intentionally updating the schema, first move the submodule to the
desired upstream commit:

```bash
git submodule update --remote schema
```

Then regenerate and verify the SDK:

```bash
make generate
make check
```

Generated output must stay deterministic, gofmt-formatted, and marked
`DO NOT EDIT` where applicable.

## API and architecture

Keep the boundary clear:

```text
Osmium WebSocket / protobuf
        ↓
internal/gateway → internal/rpc broker → internal/dispatcher/scheduler
        ↓
Client → typed services and events
        ↓
bot code
```

The public API should hide frames, request IDs, locks, channels, and protobuf
oneofs. Add an exported type only when it removes real developer friction and
can remain stable. Public API stability is more important than internal
implementation stability.

Keep `context.Context` on operations that can block. Preserve one reader and
one controlled writer, bounded queues, pending-call cleanup, and cancellation
through shutdown.

## Pull requests

Before opening a pull request:

1. Run `make check`.
2. Run `make build` when changing Go code, protocol generation, or public API.
3. From `docs/`, run `bun run docs:check` and `bun run docs:build` when changing docs.
4. Update README or documentation when changing public behavior.
5. Keep commits and pull requests focused on one change.

Use a clear description with the problem, the change, and the verification
performed. Do not claim a real Osmium integration works unless it was tested
with valid credentials.

## Releases

Releases are managed by Release Please from commits merged into `main`. Use
Conventional Commits such as `fix: correct member lookup` or
`feat: add a service`. Release Please opens a release pull request, updates
`CHANGELOG.md`, and creates the version tag and GitHub Release when that pull
request is merged.

The first release is bootstrapped manually as described in the
[release guide](docs/content/releases.md). Do not edit generated changelog
entries or the release manifest casually.

## Scope discipline

Prefer the smallest abstraction that removes real developer friction. Do not
add a command framework, file discovery, cache, plugin system, REST layer,
dependency-injection framework, or broad endpoint surface without a concrete
requirement.

Do not introduce unbounded goroutines, unbounded channels, reflection in hot
paths, runtime filesystem scans, or speculative compatibility layers.

## Issues

When reporting a bug, include the Go version, operating system, relevant
Osmose version or commit, a small reproduction, and the command output from
`make check` when possible.
