package chats

import (
	"context"
	"testing"

	protoChats "github.com/ofabiodev/osmose/proto/chats"
	"github.com/ofabiodev/osmose/proto/core"
	protoTypes "github.com/ofabiodev/osmose/proto/types"
	"github.com/ofabiodev/osmose/types"
	"google.golang.org/protobuf/proto"
)

func TestListBuildsFiltersAndMapsModels(t *testing.T) {
	var got proto.Message
	service := New(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
		got = request
		return &core.RPCResult{Result: &core.RPCResult_Chats{Chats: &protoChats.Chats{
			Users:    []*protoTypes.User{{Id: 8, Name: "Author"}},
			Channels: []*protoTypes.Channel{{Id: 9, Name: "general"}},
			Messages: []*protoTypes.Message{{MessageId: 10, AuthorId: 8, Message: "hello"}},
		}}}, nil
	})

	result, err := service.List(context.Background(), ListParams{Limit: 25, MaxID: 50, MinID: 5})
	if err != nil {
		t.Fatal(err)
	}
	request, ok := got.(*protoChats.GetChats)
	if !ok || request.GetLimit() != 25 || request.GetMaxId() != 50 || request.GetMinId() != 5 {
		t.Fatalf("unexpected request: %#v", got)
	}
	if len(result.Users) != 1 || result.Users[0].ID != 8 || len(result.Channels) != 1 || result.Channels[0].Name != "general" || result.Channels[0].Raw == nil || len(result.Messages) != 1 || result.Messages[0].Content != "hello" || result.Messages[0].Author == nil || result.Messages[0].Author.ID != 8 {
		t.Fatalf("unexpected list result: %#v", result)
	}
}

func TestSetTypingBuildsRequest(t *testing.T) {
	var got proto.Message
	service := New(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
		got = request
		return &core.RPCResult{}, nil
	})

	if err := service.SetTyping(context.Background(), types.SelfChat(), true); err != nil {
		t.Fatal(err)
	}
	request, ok := got.(*protoChats.SetTyping)
	if !ok || !request.GetTyping() {
		t.Fatalf("unexpected request: %#v", got)
	}
	chat, err := types.SelfChat().ToProto()
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(request.GetChatRef(), chat) {
		t.Fatalf("unexpected chat ref: %#v", request.GetChatRef())
	}
}

func TestSetTypingRejectsNilResult(t *testing.T) {
	service := New(func(context.Context, proto.Message) (*core.RPCResult, error) {
		return nil, nil
	})
	if err := service.SetTyping(context.Background(), types.SelfChat(), true); err == nil {
		t.Fatal("nil RPC result was treated as success")
	}
}
