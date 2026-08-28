package messages

import (
	"context"
	"testing"

	"github.com/ofabiodev/osmose/proto/core"
	protoMessages "github.com/ofabiodev/osmose/proto/messages"
	protoTypes "github.com/ofabiodev/osmose/proto/types"
	"github.com/ofabiodev/osmose/types"
	"google.golang.org/protobuf/proto"
)

func TestSendBuildsRequestAndMapsResult(t *testing.T) {
	var got proto.Message
	service := New(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
		got = request
		return &core.RPCResult{Result: &core.RPCResult_SentMessage{
			SentMessage: &protoMessages.SentMessage{MessageId: 99},
		}}, nil
	})

	sent, err := service.Send(context.Background(), SendParams{
		Chat:    types.UserChat(7),
		Content: "hello",
		ReplyTo: 42,
	})
	if err != nil {
		t.Fatal(err)
	}
	request, ok := got.(*protoMessages.SendMessage)
	if !ok {
		t.Fatalf("got %T, want *messages.SendMessage", got)
	}
	chat, err := types.UserChat(7).ToProto()
	if err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(request.GetChatRef(), chat) || request.GetMessage() != "hello" || request.GetReplyTo().GetMessageId() != 42 {
		t.Fatalf("unexpected request: %#v", request)
	}
	if sent.ID != 99 || sent.Raw == nil {
		t.Fatalf("unexpected result: %#v", sent)
	}
}

func TestHistoryBuildsRangeAndMapsModels(t *testing.T) {
	replyTo := uint64(3)
	pinned := true
	var got proto.Message
	service := New(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
		got = request
		return &core.RPCResult{Result: &core.RPCResult_Messages{Messages: &protoMessages.Messages{
			Messages: []*protoTypes.Message{{MessageId: 4, AuthorId: 8, Message: "hello", ReplyTo: &replyTo, Pinned: &pinned}},
			Users:    []*protoTypes.User{{Id: 8, Name: "Author"}},
		}}}, nil
	})

	history, err := service.History(context.Background(), HistoryParams{Chat: types.SelfChat(), Limit: 20, Before: 100})
	if err != nil {
		t.Fatal(err)
	}
	request, ok := got.(*protoMessages.GetHistory)
	if !ok || request.GetLimit() != 20 || request.GetBefore() != 100 {
		t.Fatalf("unexpected request: %#v", got)
	}
	if len(history.Messages) != 1 || history.Messages[0].ReplyTo != 3 || !history.Messages[0].Pinned || history.Messages[0].Author == nil || history.Messages[0].Author.ID != 8 || len(history.Users) != 1 || history.Users[0].ID != 8 {
		t.Fatalf("unexpected history: %#v", history)
	}
}

func TestMessageServiceRejectsInvalidParamsBeforeRPC(t *testing.T) {
	calls := 0
	service := New(func(context.Context, proto.Message) (*core.RPCResult, error) {
		calls++
		return &core.RPCResult{}, nil
	})
	cases := []struct {
		name string
		call func() error
	}{
		{name: "empty content", call: func() error {
			_, err := service.Send(context.Background(), SendParams{Chat: types.SelfChat()})
			return err
		}},
		{name: "reply without message ID", call: func() error {
			_, err := service.Reply(context.Background(), &types.Message{Chat: types.SelfChat()}, "reply")
			return err
		}},
		{name: "edit without message ID", call: func() error {
			return service.Edit(context.Background(), EditParams{Chat: types.SelfChat(), Content: stringPointer("edit")})
		}},
		{name: "delete without IDs", call: func() error {
			return service.Delete(context.Background(), DeleteParams{Chat: types.SelfChat()})
		}},
		{name: "delete with zero ID", call: func() error {
			return service.Delete(context.Background(), DeleteParams{Chat: types.SelfChat(), MessageIDs: []types.ID{0}})
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("invalid params were accepted")
			}
		})
	}
	if calls != 0 {
		t.Fatalf("invalid params reached RPC: %d calls", calls)
	}
}

func TestSendBuildsRichMessageRequest(t *testing.T) {
	var got proto.Message
	service := New(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
		got = request
		return &core.RPCResult{Result: &core.RPCResult_SentMessage{SentMessage: &protoMessages.SentMessage{MessageId: 10}}}, nil
	})
	media := types.EmbedMedia("https://example.com/image.png")
	buttons := types.MessageButtons{{LinkButton("Open", "https://example.com")}, {InteractionButton("Confirm", "confirm")}}
	sent, err := service.Send(context.Background(), SendParams{
		Chat:       types.SelfChat(),
		ReplyTo:    3,
		ReplyQuote: &types.MessageQuote{Content: "quoted"},
		Media:      []*types.MediaRef{media},
		BotInfo:    &types.MessageBotInfo{Cloak: &types.MessageCloak{Name: "Helper", PhotoID: 4}, Buttons: buttons},
	})
	if err != nil {
		t.Fatal(err)
	}
	request, ok := got.(*protoMessages.SendMessage)
	if !ok || len(request.GetMedia()) != 1 || request.GetMedia()[0].GetEmbed().GetUrl() != "https://example.com/image.png" {
		t.Fatalf("unexpected rich request: %#v", got)
	}
	if request.GetReplyTo().GetMessageId() != 3 || request.GetReplyTo().GetQuote().GetMessage() != "quoted" {
		t.Fatalf("unexpected reply quote: %#v", request.GetReplyTo())
	}
	if request.GetBotInfo().GetCloak().GetName() != "Helper" || request.GetBotInfo().GetCloak().GetPhotoId() != 4 || len(request.GetBotInfo().GetButtons().GetRows()) != 2 {
		t.Fatalf("unexpected bot info: %#v", request.GetBotInfo())
	}
	if request.GetBotInfo().GetButtons().GetRows()[0].GetButtons()[0].GetUrl().GetUrl() != "https://example.com" || request.GetBotInfo().GetButtons().GetRows()[1].GetButtons()[0].GetInteraction().GetData() != "confirm" {
		t.Fatalf("unexpected buttons: %#v", request.GetBotInfo().GetButtons())
	}
	if sent.ID != 10 {
		t.Fatalf("unexpected sent message: %#v", sent)
	}
}

func TestEditBuildsRichMessageRequest(t *testing.T) {
	var got proto.Message
	service := New(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
		got = request
		return &core.RPCResult{}, nil
	})
	buttons := types.MessageButtons{{ClipboardButton("Copy", "value")}}
	if err := service.Edit(context.Background(), EditParams{
		Chat:           types.SelfChat(),
		MessageID:      8,
		Content:        stringPointer("updated"),
		RemoveMedia:    true,
		SuppressEmbeds: true,
		Buttons:        &buttons,
	}); err != nil {
		t.Fatal(err)
	}
	request, ok := got.(*protoMessages.EditMessage)
	if !ok || !request.GetRemoveMedia() || !request.GetSuppressEmbeds() || request.GetMessage() != "updated" || request.GetButtons().GetRows()[0].GetButtons()[0].GetClipboard().GetText() != "value" {
		t.Fatalf("unexpected edit request: %#v", got)
	}
}

func TestEditCanLeaveContentUnchanged(t *testing.T) {
	var got proto.Message
	service := New(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
		got = request
		return &core.RPCResult{}, nil
	})
	if err := service.Edit(context.Background(), EditParams{Chat: types.SelfChat(), MessageID: 8}); err != nil {
		t.Fatal(err)
	}
	request, ok := got.(*protoMessages.EditMessage)
	if !ok {
		t.Fatalf("got %T, want *messages.EditMessage", got)
	}
	if request.Message != nil {
		t.Fatalf("content was sent when it was omitted: %q", request.GetMessage())
	}
}

func stringPointer(value string) *string { return &value }

func TestSearchBuildsRequestAndMapsResult(t *testing.T) {
	var got proto.Message
	service := New(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
		got = request
		return &core.RPCResult{Result: &core.RPCResult_Messages{Messages: &protoMessages.Messages{
			Messages: []*protoTypes.Message{{MessageId: 5, Message: "match"}},
		}}}, nil
	})
	result, err := service.Search(context.Background(), SearchParams{Query: "match", Before: 20})
	if err != nil {
		t.Fatal(err)
	}
	request, ok := got.(*protoMessages.Search)
	if !ok || request.GetQuery() != "match" || request.GetBefore() != 20 || request.GetScoped() {
		t.Fatalf("unexpected search request: %#v", got)
	}
	if len(result.Messages) != 1 || result.Messages[0].Content != "match" {
		t.Fatalf("unexpected search result: %#v", result)
	}
}

func TestFetchesPinnedMessagesAndUnreadMentions(t *testing.T) {
	requests := make([]proto.Message, 0, 2)
	service := New(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
		requests = append(requests, request)
		switch request.(type) {
		case *protoMessages.GetPinnedMessages:
			return &core.RPCResult{Result: &core.RPCResult_Messages{Messages: &protoMessages.Messages{Messages: []*protoTypes.Message{{MessageId: 1}}}}}, nil
		default:
			return &core.RPCResult{Result: &core.RPCResult_UnreadMentions{UnreadMentions: &protoMessages.UnreadMentions{MessageIds: []uint64{2, 3}}}}, nil
		}
	})
	pinned, err := service.PinnedMessages(context.Background(), types.SelfChat())
	if err != nil || len(pinned.Messages) != 1 || pinned.Messages[0].ID != 1 {
		t.Fatalf("unexpected pinned messages: %#v, %v", pinned, err)
	}
	mentions, err := service.UnreadMentions(context.Background(), types.SelfChat())
	if err != nil || len(mentions) != 2 || mentions[1] != 3 || len(requests) != 2 {
		t.Fatalf("unexpected mentions: %#v, %v", mentions, err)
	}
}

func TestRichMessageValidation(t *testing.T) {
	service := New(func(context.Context, proto.Message) (*core.RPCResult, error) {
		t.Fatal("invalid rich message reached RPC")
		return nil, nil
	})
	_, err := service.Send(context.Background(), SendParams{
		Chat:    types.SelfChat(),
		Content: "text",
		BotInfo: &types.MessageBotInfo{Buttons: types.MessageButtons{{{Label: "bad"}}}},
	})
	if err == nil {
		t.Fatal("button without action was accepted")
	}
}
