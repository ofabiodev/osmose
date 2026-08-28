package main

import (
	"context"
	"log"
	"os"

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

	ctx := context.Background()
	sent, err := client.Messages.Send(ctx, messages.SendParams{
		Chat:    types.SelfChat(),
		Content: "Hello from Osmose",
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("sent %d", sent.ID)

	history, err := client.Messages.History(ctx, messages.HistoryParams{
		Chat:  types.SelfChat(),
		Limit: 50,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("received %d messages", len(history.Messages))

	user, err := client.Users.Get(ctx, "some-user")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("found user %s", user.Name)
}
