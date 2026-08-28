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

## Contributing

Bug reports, improvements, and documentation fixes are welcome. Read
[CONTRIBUTING.md](https://github.com/ofabiodev/osmose/blob/main/CONTRIBUTING.md)
for the development workflow and verification commands.
