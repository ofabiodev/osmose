---
title: Getting started
description: Create and run your first Osmium bot with Osmose.
group: Start here
order: 2
layout: doc
---

## Install

Create a Go module and add Osmose:

```bash
go mod init example.com/my-bot
go get github.com/ofabiodev/osmose
```

Set the bot token in the environment:

```bash
export OSMIUM_TOKEN="your-token"
```

On PowerShell:

```powershell
$env:OSMIUM_TOKEN = "your-token"
```

## A ping bot

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

`Run` handles connect, initialize, authorize, event dispatch, keepalive,
reconnect, and shutdown.

## Next steps

- Read [Events](../events/) to handle messages and interactions.
- Use [Services](../services/) to send messages and query Osmium.
- Use [Protocol and raw API](../protocol/) when a service does not cover an endpoint yet.
