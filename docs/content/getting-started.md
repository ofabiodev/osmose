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
	"github.com/ofabiodev/osmose/types"
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

	client.OnMessage(func(ctx context.Context, message *types.Message) error {
		if message.Content != "!ping" {
			return nil
		}
		_, err := message.Reply(ctx, "Pong! 🏓")
		return err
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
- Use [managers and rich objects](../state-management/) for stateful entity operations.
- Use [specialized services](../services/) for chats and voice.
- Use [Protocol and raw API](../protocol/) when a high-level API does not cover an endpoint yet.
