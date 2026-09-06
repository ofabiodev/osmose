---
title: Development
description: Run Osmose examples and preview the documentation locally.
group: Reference
order: 1
layout: doc
---

## Run a bot locally

Create a Go module, add Osmose, and place your bot in `main.go`:

```bash
go mod init example.com/my-bot
go get github.com/ofabiodev/osmose
go run .
```

Set `OSMIUM_TOKEN` before starting the bot. On PowerShell:

```powershell
$env:OSMIUM_TOKEN = "your-bot-token"
go run .
```

The [ping example](https://github.com/ofabiodev/osmose/tree/main/examples/ping)
shows the smallest complete event-driven bot.

The [stateful example](https://github.com/ofabiodev/osmose/tree/main/examples/stateful)
demonstrates bounded caching and direct member resolution. From a development checkout:

```bash
export OSMIUM_TOKEN="your-bot-token"
export OSMIUM_CLIENT_ID="your-numeric-client-id"
go run ./examples/stateful
```

Send `!member <user ID>` in a community. See [member lookup performance](../member-lookup/)
for a benchmark and the difference between a cache hit and a fresh network fetch.

## Preview the documentation

The documentation site is built with
[docshelf](https://github.com/ofabiodev/docshelf):

```bash
cd docs
bun install
bun run docs:dev
```

From `docs/`, use `bun run docs:check` to validate the documentation or
`bun run docs:build` to create a static build.

## Clone the repository

The Osmium protocol schema is included as a pinned Git submodule. Clone
Osmose recursively when working on the SDK:

```bash
git clone --recurse-submodules https://github.com/ofabiodev/osmose.git
cd osmose
```

For an existing checkout, initialize it with:

```bash
git submodule update --init --recursive
```

The generated Go protobuf packages are committed to the repository. The
submodule is needed when regenerating protocol code, not when using Osmose as
a dependency.

## Contributing

Bug reports, improvements, and documentation fixes are welcome. Read
[CONTRIBUTING.md](https://github.com/ofabiodev/osmose/blob/main/CONTRIBUTING.md)
for the development workflow and verification commands.
