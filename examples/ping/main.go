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
		if message.Content != "!ping" || (message.Author != nil && message.Author.Bot) {
			return nil
		}
		_, err := message.Reply(ctx, "Pong! 🏓")
		return err
	})

	if err := client.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
