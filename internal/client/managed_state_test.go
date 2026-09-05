package client

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ofabiodev/osmose/internal/gateway"
	"github.com/ofabiodev/osmose/messages"
	"github.com/ofabiodev/osmose/proto/core"
	protoMessages "github.com/ofabiodev/osmose/proto/messages"
	protoTypes "github.com/ofabiodev/osmose/proto/types"
	"github.com/ofabiodev/osmose/proto/updates"
	"github.com/ofabiodev/osmose/types"
	"google.golang.org/protobuf/proto"
)

func TestGatewayStateAppliedEvenWhenEventQueueDrops(t *testing.T) {
	c, err := New(Config{Token: "token", ClientID: 1, Cache: types.CacheConfig{Enabled: true}, EventQueue: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	started, release := make(chan struct{}, 1), make(chan struct{})
	c.OnUpdate(func(context.Context, *UpdateEvent) error {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		return nil
	})
	c.events.start(context.Background())
	defer func() { close(release); c.events.close() }()
	if err := c.events.enqueue(context.Background(), &updates.Update{}); err != nil {
		t.Fatal(err)
	}
	<-started
	active := &activeConnection{ctx: context.Background()}
	ref, _ := types.ChannelChat(10, 20).ToProto()
	for _, content := range []string{"created", "edited"} {
		message := &protoTypes.Message{MessageId: 30, ChatRef: ref, Message: content}
		update := &updates.Update{Update: &updates.Update_Message{Message: &updates.UpdateMessage{Message: message}}}
		frame, _ := proto.Marshal(&core.ServerMessage{Message: &core.ServerMessage_Update{Update: update}})
		c.handleFrame(active, frame)
	}
	message, ok := c.Managers.Messages.In(types.ChannelChat(10, 20)).Get(30)
	if !ok || message.Content != "edited" || c.DroppedEvents() != 1 {
		t.Fatalf("state=%+v dropped=%d", message, c.DroppedEvents())
	}
	deletion := &updates.Update{Update: &updates.Update_ChannelDeleted{ChannelDeleted: &updates.UpdateChannelDeleted{Channel: ref.GetChannel()}}}
	frame, _ := proto.Marshal(&core.ServerMessage{Message: &core.ServerMessage_Update{Update: deletion}})
	c.handleFrame(active, frame)
	if _, ok := c.Managers.Messages.In(types.ChannelChat(10, 20)).Get(30); ok {
		t.Fatal("deleted channel retained messages")
	}
}

func TestLegacyServiceRawAndRichHandlersShareState(t *testing.T) {
	socket := newScriptedSocket()
	c, err := New(Config{Token: "token", ClientID: 1, Cache: types.CacheConfig{Enabled: true}, HeartbeatInterval: time.Hour,
		dial: func(context.Context, string) (gateway.Socket, error) { return socket, nil }})
	if err != nil {
		t.Fatal(err)
	}
	cancel, runErr := startReadyClient(t, c)
	defer stopTestClient(t, c, cancel, runErr)
	chat := types.ChannelChat(10, 20)
	sent, err := c.Messages.Send(context.Background(), messages.SendParams{Chat: chat, Content: "legacy"})
	if err != nil {
		t.Fatal(err)
	}
	cached, ok := c.Managers.Messages.In(chat).Get(sent.ID)
	if !ok || !cached.Partial || cached.Content != "legacy" {
		t.Fatal("legacy service did not populate shared cache")
	}
	ref, _ := chat.ToProto()
	_, err = c.Raw().Call(context.Background(), &protoMessages.SendMessage{ChatRef: ref, Message: "raw"})
	if err != nil {
		t.Fatal(err)
	}
	// A sent response is partial. The created event is authoritative and must
	// not be replaced by an ID-only message or an older handler snapshot.
	update := &updates.Update{Update: &updates.Update_MessageCreated{MessageCreated: &updates.UpdateMessageCreated{
		Message: &protoTypes.Message{MessageId: sent.ID.Uint64(), ChatRef: ref, AuthorId: 77, Message: "full"},
	}}}
	c.objectClient.ApplyUpdate(update)
	called := false
	want := errors.New("handler error")
	c.events.onHandlerError = func(_ context.Context, e *HandlerError) {
		if !errors.Is(e.Err, want) {
			t.Errorf("handler hook: %v", e.Err)
		}
	}
	remove := c.OnMessage(func(ctx context.Context, message *types.Message) error {
		called = true
		if message.Author == nil || message.Author.ID != 77 || message.Channel() == nil {
			t.Error("missing bound related objects")
		}
		current, ok := c.Managers.Messages.In(chat).Get(message.ID)
		if !ok || current.Partial || current.Content != "full" {
			t.Error("handler ran before state was synchronized")
		}
		return want
	})
	c.events.dispatch(context.Background(), update)
	if !called {
		t.Fatal("rich handler not invoked")
	}
	remove()
	called = false
	c.events.dispatch(context.Background(), update)
	if called {
		t.Fatal("rich handler unsubscribe failed")
	}
	c.Managers.Clear()
	if _, ok := c.Managers.Messages.In(chat).Get(sent.ID); ok {
		t.Fatal("clear retained state")
	}
}
