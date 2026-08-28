package reactions

import (
	"context"
	"testing"

	"github.com/ofabiodev/osmose/proto/core"
	protoReactions "github.com/ofabiodev/osmose/proto/reactions"
	"github.com/ofabiodev/osmose/types"
	"google.golang.org/protobuf/proto"
)

func TestAddBuildsUnicodeReaction(t *testing.T) {
	var got proto.Message
	service := New(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
		got = request
		return &core.RPCResult{}, nil
	})

	if err := service.Add(context.Background(), Params{Chat: types.SelfChat(), MessageID: 4, Emoji: Emoji{Unicode: "👍"}}); err != nil {
		t.Fatal(err)
	}
	request, ok := got.(*protoReactions.AddReaction)
	if !ok || request.GetMessageId() != 4 || request.GetEmoji().GetUnicodeEmoji() != "👍" {
		t.Fatalf("unexpected request: %#v", got)
	}
}

func TestEmojiRejectsAmbiguousAndEmptyValues(t *testing.T) {
	service := New(func(context.Context, proto.Message) (*core.RPCResult, error) {
		t.Fatal("RPC should not be called")
		return nil, nil
	})
	for _, emoji := range []Emoji{{}, {Unicode: "👍", Custom: 9}} {
		if err := service.Add(context.Background(), Params{Chat: types.SelfChat(), MessageID: 4, Emoji: emoji}); err == nil {
			t.Fatalf("emoji %#v was accepted", emoji)
		}
	}
}

func TestReactionRejectsZeroMessageID(t *testing.T) {
	service := New(func(context.Context, proto.Message) (*core.RPCResult, error) {
		t.Fatal("RPC should not be called")
		return nil, nil
	})
	if err := service.Add(context.Background(), Params{Chat: types.SelfChat(), Emoji: Emoji{Unicode: "👍"}}); err == nil {
		t.Fatal("zero message ID was accepted")
	}
}

func TestReactionRejectsNilVoidResult(t *testing.T) {
	service := New(func(context.Context, proto.Message) (*core.RPCResult, error) {
		return nil, nil
	})
	if err := service.Add(context.Background(), Params{Chat: types.SelfChat(), MessageID: 4, Emoji: Emoji{Unicode: "👍"}}); err == nil {
		t.Fatal("nil RPC result was treated as success")
	}
}
