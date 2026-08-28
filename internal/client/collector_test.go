package client

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	protoReactions "github.com/ofabiodev/osmose/proto/reactions"
	protoRefs "github.com/ofabiodev/osmose/proto/refs"
	protoTypes "github.com/ofabiodev/osmose/proto/types"
	"github.com/ofabiodev/osmose/proto/updates"
	"github.com/ofabiodev/osmose/types"
)

func newCollectorTestClient(t *testing.T) *Client {
	t.Helper()
	client, err := New(Config{
		Token:    "token",
		ClientID: 1,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func dispatchMessage(t *testing.T, client *Client, chat types.ChatRef, authorID types.ID, content string) {
	t.Helper()
	chatRef, err := chat.ToProto()
	if err != nil {
		t.Fatal(err)
	}
	client.events.dispatch(context.Background(), &updates.Update{Update: &updates.Update_MessageCreated{
		MessageCreated: &updates.UpdateMessageCreated{
			Message: &protoTypes.Message{ChatRef: chatRef, MessageId: uint64(authorID + 100), AuthorId: uint64(authorID), Message: content},
		},
	}})
}

func TestMessageCollectorFiltersAndStopsAtMax(t *testing.T) {
	client := newCollectorTestClient(t)
	chat := types.SelfChat()
	collector, err := client.CollectMessages(MessageCollectorOptions{
		Chat:     chat,
		AuthorID: 7,
		Max:      2,
		Buffer:   2,
		Filter: func(event *MessageCreateEvent) bool {
			return event.Message.Content == "keep"
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	dispatchMessage(t, client, chat, 8, "keep")
	dispatchMessage(t, client, types.UserChat(2), 7, "keep")
	dispatchMessage(t, client, chat, 7, "skip")
	dispatchMessage(t, client, chat, 7, "keep")
	dispatchMessage(t, client, chat, 7, "keep")

	var collected int
	for range collector.Events() {
		collected++
	}
	result := collector.Result()
	if collected != 2 || result.Collected != 2 || result.Reason != EndReasonLimit || result.Err != nil {
		t.Fatalf("unexpected collector result: %#v", result)
	}
}

func TestMessageCollectorTimeout(t *testing.T) {
	client := newCollectorTestClient(t)
	collector, err := client.CollectMessages(MessageCollectorOptions{Time: 15 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-collector.Done():
	case <-time.After(time.Second):
		t.Fatal("collector did not stop after its time limit")
	}
	result := collector.Result()
	if result.Reason != EndReasonTime || !errors.Is(result.Err, ErrCollectorTimeout) {
		t.Fatalf("unexpected timeout result: %#v", result)
	}
}

func TestMessageCollectorIdleTimerResetsAfterMatch(t *testing.T) {
	client := newCollectorTestClient(t)
	collector, err := client.CollectMessages(MessageCollectorOptions{Idle: 30 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	dispatchMessage(t, client, types.SelfChat(), 7, "hello")
	select {
	case <-collector.Done():
		t.Fatal("collector became idle before the reset interval")
	case <-time.After(10 * time.Millisecond):
	}
	select {
	case <-collector.Done():
	case <-time.After(time.Second):
		t.Fatal("collector did not stop after becoming idle")
	}
	if result := collector.Result(); result.Reason != EndReasonIdle || !errors.Is(result.Err, ErrCollectorIdle) || result.Collected != 1 {
		t.Fatalf("unexpected idle result: %#v", result)
	}
}

func TestMessageCollectorReportsOverflow(t *testing.T) {
	client := newCollectorTestClient(t)
	collector, err := client.CollectMessages(MessageCollectorOptions{Buffer: 1})
	if err != nil {
		t.Fatal(err)
	}
	dispatchMessage(t, client, types.SelfChat(), 7, "first")
	dispatchMessage(t, client, types.SelfChat(), 7, "second")

	result := collector.Result()
	if result.Reason != EndReasonOverflow || !errors.Is(result.Err, ErrCollectorOverflow) || result.Collected != 1 {
		t.Fatalf("unexpected overflow result: %#v", result)
	}
	if event, ok := <-collector.Events(); !ok || event.Message.Content != "first" {
		t.Fatalf("buffered event was lost: %#v, %v", event, ok)
	}
	if _, ok := <-collector.Events(); ok {
		t.Fatal("collector events channel remained open")
	}
}

func TestAwaitMessageUsesContextAndRemovesHandler(t *testing.T) {
	client := newCollectorTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.AwaitMessage(ctx, MessageCollectorOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected await error: %v", err)
	}
	client.events.mu.RLock()
	handlers := len(client.events.messageCreate)
	client.events.mu.RUnlock()
	if handlers != 0 {
		t.Fatalf("collector handler was not removed: %d", handlers)
	}
}

func TestMessageCollectorStopIsIdempotent(t *testing.T) {
	client := newCollectorTestClient(t)
	collector, err := client.CollectMessages(MessageCollectorOptions{})
	if err != nil {
		t.Fatal(err)
	}
	collector.Stop(EndReasonStopped)
	collector.Stop(EndReasonTime)
	result := collector.Result()
	if result.Reason != EndReasonStopped || result.Err != nil {
		t.Fatalf("unexpected stopped result: %#v", result)
	}
	if _, ok := <-collector.Events(); ok {
		t.Fatal("stopped collector events channel remained open")
	}
}

func TestMessageCollectorStopsWithClient(t *testing.T) {
	client := newCollectorTestClient(t)
	collector, err := client.CollectMessages(MessageCollectorOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-collector.Done():
	case <-time.After(time.Second):
		t.Fatal("collector did not stop with client")
	}
	result := collector.Result()
	if result.Reason != EndReasonClosed || !errors.Is(result.Err, ErrCollectorClosed) {
		t.Fatalf("unexpected collector result: %#v", result)
	}
	client.events.mu.RLock()
	handlers := len(client.events.messageCreate)
	client.events.mu.RUnlock()
	if handlers != 0 {
		t.Fatalf("collector handler was not removed: %d", handlers)
	}
}

func TestMessageCollectorContextStopsCollection(t *testing.T) {
	client := newCollectorTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	collector, err := client.CollectMessagesContext(ctx, MessageCollectorOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case <-collector.Done():
	case <-time.After(time.Second):
		t.Fatal("collector did not stop with context")
	}
	result := collector.Result()
	if result.Reason != EndReasonContext || !errors.Is(result.Err, context.Canceled) {
		t.Fatalf("unexpected collector result: %#v", result)
	}
}

func dispatchInteraction(t *testing.T, client *Client, id, userID, messageID types.ID, data string) {
	t.Helper()
	client.events.dispatch(context.Background(), &updates.Update{Update: &updates.Update_Interaction{
		Interaction: &updates.UpdateInteraction{
			InteractionId: uint64(id),
			UserId:        uint64(userID),
			MessageId:     uint64(messageID),
			Data:          &data,
		},
	}})
}

func TestInteractionCollectorFiltersAndStopsAtMax(t *testing.T) {
	client := newCollectorTestClient(t)
	collector, err := client.CollectInteractions(InteractionCollectorOptions{
		UserID:    7,
		MessageID: 9,
		Data:      "confirm",
		Max:       1,
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatchInteraction(t, client, 1, 7, 9, "cancel")
	dispatchInteraction(t, client, 2, 7, 9, "confirm")
	dispatchInteraction(t, client, 3, 7, 9, "confirm")

	event, err := collector.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if event.ID != 2 || event.Interaction == nil || event.Interaction.Data != "confirm" {
		t.Fatalf("unexpected interaction: %#v", event)
	}
	if _, ok := <-collector.Events(); ok {
		t.Fatal("interaction collector events channel remained open")
	}
	if result := collector.Result(); result.Collected != 1 || result.Reason != EndReasonLimit || result.Err != nil {
		t.Fatalf("unexpected collector result: %#v", result)
	}
}

func TestAwaitInteractionUsesContextAndRemovesHandler(t *testing.T) {
	client := newCollectorTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.AwaitInteraction(ctx, InteractionCollectorOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected await error: %v", err)
	}
	client.events.mu.RLock()
	handlers := len(client.events.interaction)
	client.events.mu.RUnlock()
	if handlers != 0 {
		t.Fatalf("interaction collector handler was not removed: %d", handlers)
	}
}

func TestReactionCollectorFiltersSnapshots(t *testing.T) {
	client := newCollectorTestClient(t)
	collector, err := client.CollectReactions(ReactionCollectorOptions{Chat: types.SelfChat(), MessageID: 5, Max: 1})
	if err != nil {
		t.Fatal(err)
	}
	client.events.dispatch(context.Background(), &updates.Update{Update: &updates.Update_MessageReactions{
		MessageReactions: &updates.UpdateMessageReactions{
			ChatRef: func() *protoRefs.ChatRef {
				ref, _ := types.SelfChat().ToProto()
				return ref
			}(),
			Reactions: &protoReactions.MessageReactions{MessageId: 5},
		},
	}})
	event, err := collector.Next(context.Background())
	if err != nil || event.MessageID != 5 || event.Reactions == nil {
		t.Fatalf("unexpected reaction event: %#v, %v", event, err)
	}
	if result := collector.Result(); result.Reason != EndReasonLimit || result.Collected != 1 {
		t.Fatalf("unexpected reaction result: %#v", result)
	}
}
