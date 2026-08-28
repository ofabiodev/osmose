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
