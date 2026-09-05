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
	client, err := osmose.New(osmose.Config{
		Token:    os.Getenv("OSMIUM_TOKEN"),
		ClientID: 123456,
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	// Network operations require a ready connection. This one-shot example closes
	// after demonstrating the manager and object APIs.
	client.OnReady(func(ctx context.Context, event *osmose.ReadyEvent) error {
		defer client.Close()
		messages := client.Managers.Messages.In(types.SelfChat())
		sent, err := messages.Create(ctx, "Hello from Osmose")
		if err != nil {
			return err
		}
		log.Printf("sent %d", sent.ID)

		history, err := messages.List(ctx, types.MessageHistoryParams{Limit: 50})
		if err != nil {
			return err
		}
		log.Printf("received %d messages", len(history.Messages))

		user, err := client.Managers.Users.Fetch(ctx, event.User.ID)
		if err != nil {
			return err
		}
		log.Printf("found user %s", user.Name)
		return nil
	})
	if err := client.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
