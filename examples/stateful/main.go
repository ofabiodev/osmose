// Run with OSMIUM_TOKEN and OSMIUM_CLIENT_ID, then send !member <user ID> in a
// community. The first lookup fetches only that member; later lookups use state.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/ofabiodev/osmose"
	"github.com/ofabiodev/osmose/types"
)

func main() {
	id, err := strconv.ParseUint(os.Getenv("OSMIUM_CLIENT_ID"), 10, 32)
	if err != nil || id == 0 {
		log.Fatal("set OSMIUM_CLIENT_ID to your bot's numeric client ID")
	}
	client, err := osmose.New(osmose.Config{
		Token: os.Getenv("OSMIUM_TOKEN"), ClientID: uint32(id),
		Cache: osmose.CacheConfig{Enabled: true},
	})
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	client.OnMessage(func(ctx context.Context, message *types.Message) error {
		if message.AuthorID == client.User().ID || (message.Author != nil && message.Author.Bot) {
			return nil
		}
		args := strings.Fields(message.Content)
		if len(args) == 0 || args[0] != "!member" {
			return nil
		}
		community := message.Community()
		if community == nil {
			return nil
		}
		if len(args) != 2 {
			_, err := message.Reply(ctx, "Usage: !member <user ID>")
			return err
		}
		id, err := strconv.ParseUint(args[1], 10, 64)
		if err != nil || id == 0 {
			_, err := message.Reply(ctx, "Provide a numeric user ID.")
			return err
		}
		member, err := community.Collections().Members.Resolve(ctx, types.ID(id))
		if err != nil {
			return fmt.Errorf("resolve member: %w", err)
		}
		_, err = message.Reply(ctx, fmt.Sprintf("Member %d has %d role(s).", member.ID, len(member.RoleIDs)))
		return err
	})
	if err := client.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
