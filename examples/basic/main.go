package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/ofabiodev/osmose"
	"github.com/ofabiodev/osmose/messages"
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
	// Services require a ready connection. This one-shot example closes after
	// demonstrating the original service API, which remains supported in v0.3.
	client.OnReady(func(ctx context.Context, event *osmose.ReadyEvent) error {
		defer client.Close()
		sent, err := client.Messages.Send(ctx, messages.SendParams{
			Chat:    types.SelfChat(),
			Content: "Hello from Osmose",
		})
		if err != nil {
			return err
		}
		log.Printf("sent %d", sent.ID)

		history, err := client.Messages.History(ctx, messages.HistoryParams{
			Chat:  types.SelfChat(),
			Limit: 50,
		})
		if err != nil {
			return err
		}
		log.Printf("received %d messages", len(history.Messages))

		user, err := client.Users.Get(ctx, event.User.Username)
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
