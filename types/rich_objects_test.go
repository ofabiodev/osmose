package types

import (
	"context"
	"errors"
	"testing"

	protoCommunities "github.com/ofabiodev/osmose/proto/communities"
	"github.com/ofabiodev/osmose/proto/core"
	protoMessages "github.com/ofabiodev/osmose/proto/messages"
	protoReactions "github.com/ofabiodev/osmose/proto/reactions"
	protoRefs "github.com/ofabiodev/osmose/proto/refs"
	protoTypes "github.com/ofabiodev/osmose/proto/types"
	"google.golang.org/protobuf/proto"
)

func newRichTestClient(t *testing.T, handler func(proto.Message) *core.RPCResult) (*ObjectClient, *[]proto.Message) {
	t.Helper()
	requests := new([]proto.Message)
	return NewObjectClient(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
		*requests = append(*requests, request)
		return handler(request), nil
	}), requests
}

func voidResult() *core.RPCResult { return &core.RPCResult{} }

func sentResult(id ID) *core.RPCResult {
	return &core.RPCResult{Result: &core.RPCResult_SentMessage{
		SentMessage: &protoMessages.SentMessage{MessageId: uint64(id)},
	}}
}

func requestOf[T proto.Message](requests []proto.Message) T {
	var zero T
	for i := len(requests) - 1; i >= 0; i-- {
		if request, ok := requests[i].(T); ok {
			return request
		}
	}
	return zero
}

func requestsOf[T proto.Message](requests []proto.Message) []T {
	var result []T
	for _, request := range requests {
		if typed, ok := request.(T); ok {
			result = append(result, typed)
		}
	}
	return result
}

func TestCommunityObjectsLoadRelatedRichObjects(t *testing.T) {
	client, requests := newRichTestClient(t, func(request proto.Message) *core.RPCResult {
		switch request.(type) {
		case *protoCommunities.GetChannels:
			return &core.RPCResult{Result: &core.RPCResult_Channels{Channels: &protoCommunities.Channels{
				Channels: []*protoTypes.Channel{{Id: 20, CommunityId: 10, Name: "general"}},
			}}}
		case *protoCommunities.GetMembers:
			return &core.RPCResult{Result: &core.RPCResult_Members{Members: &protoCommunities.Members{
				Members: []*protoTypes.CommunityMember{{Id: 30, CommunityId: 10}},
				Users:   []*protoTypes.User{{Id: 30, Username: stringPtr("member")}},
			}}}
		case *protoCommunities.GetRoles:
			return &core.RPCResult{Result: &core.RPCResult_CommunityRoles{CommunityRoles: &protoCommunities.CommunityRoles{
				Roles: []*protoTypes.CommunityRole{{Id: 40, CommunityId: 10, Name: "moderator"}},
			}}}
		default:
			return voidResult()
		}
	})

	community := CommunityFromProto(&protoTypes.Community{Id: 10, Name: "Osmium"}, client)
	channels, err := community.Channels(context.Background())
	if err != nil || len(channels) != 1 || channels[0].Name != "general" {
		t.Fatalf("channels=%#v err=%v", channels, err)
	}
	members, err := community.Members(context.Background())
	if err != nil || len(members) != 1 || members[0].User == nil || members[0].User.Username != "member" {
		t.Fatalf("members=%#v err=%v", members, err)
	}
	roles, err := community.Roles(context.Background())
	if err != nil || len(roles) != 1 || roles[0].Name != "moderator" {
		t.Fatalf("roles=%#v err=%v", roles, err)
	}

	getChannels := requestOf[*protoCommunities.GetChannels](*requests)
	if getChannels == nil || getChannels.CommunityId != 10 {
		t.Fatalf("unexpected channels request: %#v", getChannels)
	}
	getMembers := requestOf[*protoCommunities.GetMembers](*requests)
	if getMembers == nil || getMembers.CommunityId != 10 {
		t.Fatalf("unexpected members request: %#v", getMembers)
	}
	getRoles := requestOf[*protoCommunities.GetRoles](*requests)
	if getRoles == nil || getRoles.CommunityId != 10 {
		t.Fatalf("unexpected roles request: %#v", getRoles)
	}
}

func TestMessageObjectHidesMessageOperations(t *testing.T) {
	client, requests := newRichTestClient(t, func(request proto.Message) *core.RPCResult {
		switch request.(type) {
		case *protoMessages.SendMessage:
			return sentResult(12)
		default:
			return voidResult()
		}
	})

	message := MessageFromProto(&protoTypes.Message{
		MessageId: 9,
		ChatRef:   mustChatProto(t, ChannelChat(10, 20)),
		Message:   "hello",
	}, client)

	reply, err := message.Reply(context.Background(), "reply")
	if err != nil || reply == nil || reply.ID != 12 {
		t.Fatalf("reply=%#v err=%v", reply, err)
	}
	if err := message.Edit(context.Background(), "edited"); err != nil {
		t.Fatal(err)
	}
	if err := message.React(context.Background(), Emoji{Unicode: "👍"}); err != nil {
		t.Fatal(err)
	}
	if err := message.Pin(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := message.Delete(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := message.Forward(context.Background(), UserChat(50)); err != nil {
		t.Fatal(err)
	}

	sends := requestsOf[*protoMessages.SendMessage](*requests)
	if len(sends) != 1 || sends[0].GetReplyTo().GetMessageId() != 9 || sends[0].GetMessage() != "reply" {
		t.Fatalf("unexpected send requests: %#v", sends)
	}
	edit := requestOf[*protoMessages.EditMessage](*requests)
	if edit == nil || edit.GetMessage() != "edited" || edit.GetMessageId() != 9 {
		t.Fatalf("unexpected edit request: %#v", edit)
	}
	reaction := requestOf[*protoReactions.AddReaction](*requests)
	if reaction == nil || reaction.GetMessageId() != 9 || reaction.GetEmoji().GetUnicodeEmoji() != "👍" {
		t.Fatalf("unexpected reaction request: %#v", reaction)
	}
	pin := requestOf[*protoMessages.SetMessagePin](*requests)
	if pin == nil || pin.GetMessageId() != 9 || !pin.GetPin() || !message.Pinned {
		t.Fatalf("unexpected pin state/request: %#v pinned=%v", pin, message.Pinned)
	}
	delete := requestOf[*protoMessages.DeleteMessage](*requests)
	if delete == nil || len(delete.GetMessageIds()) != 1 || delete.GetMessageIds()[0] != 9 {
		t.Fatalf("unexpected delete request: %#v", delete)
	}
	forward := requestOf[*protoMessages.ForwardMessage](*requests)
	if forward == nil || len(forward.GetMessageIds()) != 1 || forward.GetMessageIds()[0] != 9 || forward.GetChatRef().GetUser().GetUserId() != 50 {
		t.Fatalf("unexpected forward request: %#v", forward)
	}
}

func TestMemberAndRoleObjectsManagePermissions(t *testing.T) {
	client, requests := newRichTestClient(t, func(request proto.Message) *core.RPCResult {
		if _, ok := request.(*protoMessages.SendMessage); ok {
			return sentResult(80)
		}
		return voidResult()
	})

	member := CommunityMemberFromProto(&protoTypes.CommunityMember{Id: 30, CommunityId: 10, RoleIds: []uint64{40}}, client)
	if err := member.Edit(context.Background(), MemberEditOptions{Nickname: stringPtr("new name"), RoleIDs: []ID{40, 41}}); err != nil {
		t.Fatal(err)
	}
	if err := member.Ban(context.Background(), BanOptions{Reason: "rule violation"}); err != nil {
		t.Fatal(err)
	}
	if err := member.Kick(context.Background(), "left"); err != nil {
		t.Fatal(err)
	}
	sent, err := member.Send(context.Background(), MessageSendParams{Content: "hello"})
	if err != nil || sent == nil || sent.ID != 80 {
		t.Fatalf("sent=%#v err=%v", sent, err)
	}

	editMember := requestOf[*protoCommunities.EditMember](*requests)
	if editMember == nil || editMember.GetNickname() != "new name" || len(editMember.GetRoleIds().GetRoleIds()) != 2 {
		t.Fatalf("unexpected member edit request: %#v", editMember)
	}
	removals := requestsOf[*protoCommunities.RemoveMembers](*requests)
	if len(removals) != 2 || removals[0].GetUntil() != 0 || removals[0].GetReason() != "rule violation" || removals[1].Until != nil {
		t.Fatalf("unexpected removals: %#v", removals)
	}
	send := requestOf[*protoMessages.SendMessage](*requests)
	if send == nil || send.GetChatRef().GetUser().GetUserId() != 30 {
		t.Fatalf("unexpected member send request: %#v", send)
	}

	role := CommunityRoleFromProto(&protoTypes.CommunityRole{Id: 40, CommunityId: 10, Name: "moderator", Permissions: 1}, client)
	if err := role.SetPermissions(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if err := role.Delete(context.Background()); err != nil {
		t.Fatal(err)
	}
	editRole := requestOf[*protoCommunities.EditRole](*requests)
	if editRole == nil || editRole.GetPermissions() != 7 || editRole.GetName() != "moderator" {
		t.Fatalf("unexpected role edit request: %#v", editRole)
	}
	deleteRole := requestOf[*protoCommunities.DeleteRole](*requests)
	if deleteRole == nil || deleteRole.GetId() != 40 || deleteRole.GetCommunityId() != 10 {
		t.Fatalf("unexpected role delete request: %#v", deleteRole)
	}
}

func TestRichObjectsRequireAnAttachedClient(t *testing.T) {
	community := CommunityFromProto(&protoTypes.Community{Id: 10})
	if !errors.Is(community.Leave(context.Background()), ErrObjectClientUnavailable) {
		t.Fatal("unbound community should reject mutations")
	}
	message := MessageFromProto(&protoTypes.Message{MessageId: 1, ChatRef: mustChatProto(t, SelfChat())})
	if _, err := message.Reply(context.Background(), "hello"); !errors.Is(err, ErrObjectClientUnavailable) {
		t.Fatal("unbound message should reject mutations")
	}
}

func mustChatProto(t *testing.T, ref ChatRef) *protoRefs.ChatRef {
	t.Helper()
	value, err := ref.ToProto()
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func stringPtr(value string) *string { return &value }
