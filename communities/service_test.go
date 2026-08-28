package communities

import (
	"context"
	"testing"

	protoCommunities "github.com/ofabiodev/osmose/proto/communities"
	"github.com/ofabiodev/osmose/proto/core"
	protoTypes "github.com/ofabiodev/osmose/proto/types"
	protoUpdates "github.com/ofabiodev/osmose/proto/updates"
	"github.com/ofabiodev/osmose/types"
	"google.golang.org/protobuf/proto"
)

func TestListBuildsRequestAndMapsCommunities(t *testing.T) {
	var got proto.Message
	service := New(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
		got = request
		return &core.RPCResult{Result: &core.RPCResult_Communities{Communities: &protoCommunities.Communities{
			Communities: []*protoTypes.Community{{Id: 7, Name: "Osmium"}},
		}}}, nil
	})

	result, err := service.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.(*protoCommunities.GetCommunities); !ok {
		t.Fatalf("got %T, want *communities.GetCommunities", got)
	}
	if len(result.Communities) != 1 || result.Communities[0].Name != "Osmium" || result.Communities[0].Raw == nil || result.Raw == nil {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestChannelsBuildsRequestAndMapsMessages(t *testing.T) {
	var got proto.Message
	service := New(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
		got = request
		return &core.RPCResult{Result: &core.RPCResult_Channels{Channels: &protoCommunities.Channels{
			Channels: []*protoTypes.Channel{{Id: 9, Name: "general"}},
			Messages: []*protoTypes.Message{{MessageId: 10, Message: "hello"}},
		}}}, nil
	})

	result, err := service.Channels(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	request, ok := got.(*protoCommunities.GetChannels)
	if !ok || request.GetCommunityId() != 7 {
		t.Fatalf("unexpected request: %#v", got)
	}
	if len(result.Channels) != 1 || result.Channels[0].Name != "general" || len(result.Messages) != 1 || result.Messages[0].Content != "hello" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestChannelsRequiresCommunityID(t *testing.T) {
	called := false
	service := New(func(context.Context, proto.Message) (*core.RPCResult, error) {
		called = true
		return nil, nil
	})
	if _, err := service.Channels(context.Background(), 0); err == nil || called {
		t.Fatal("missing community ID was not rejected before calling RPC")
	}
}

func TestChannelMembersBuildsRequestAndMapsEntries(t *testing.T) {
	var got proto.Message
	nickname := "Helper"
	username := "helper"
	service := New(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
		got = request
		return &core.RPCResult{Result: &core.RPCResult_MemberList{MemberList: &protoCommunities.MemberList{
			Entries: []*protoUpdates.MemberListEntry{
				{Entry: &protoUpdates.MemberListEntry_User{User: &protoUpdates.MemberListEntryUser{
					User:     &protoTypes.User{Id: 8, Username: &username},
					Nickname: &nickname,
				}}},
				{Entry: &protoUpdates.MemberListEntry_Divider{Divider: &protoUpdates.MemberListEntryDivider{
					Inner:       &protoUpdates.MemberListEntryDivider_RoleId{RoleId: 9},
					MemberCount: 1,
				}}},
			},
		}}}, nil
	})

	result, err := service.ChannelMembers(context.Background(), 7, 6)
	if err != nil {
		t.Fatal(err)
	}
	request, ok := got.(*protoCommunities.GetChannelMembers)
	if !ok || request.GetCommunityId() != 7 || request.GetChannelId() != 6 {
		t.Fatalf("unexpected request: %#v", got)
	}
	if len(result.Entries) != 2 || result.Entries[0].User == nil || result.Entries[0].User.ID != 8 || result.Entries[0].Nickname == nil || *result.Entries[0].Nickname != nickname {
		t.Fatalf("unexpected user entry: %#v", result)
	}
	if result.Entries[1].Divider == nil || result.Entries[1].Divider.RoleID != 9 || result.Entries[1].Divider.MemberCount != 1 || result.Raw == nil {
		t.Fatalf("unexpected divider entry: %#v", result)
	}
}

func TestChannelMembersRequiresIDs(t *testing.T) {
	called := false
	service := New(func(context.Context, proto.Message) (*core.RPCResult, error) {
		called = true
		return nil, nil
	})
	for _, ids := range [][2]types.ID{{0, 6}, {7, 0}} {
		if _, err := service.ChannelMembers(context.Background(), ids[0], ids[1]); err == nil || called {
			t.Fatalf("invalid IDs were not rejected: community=%d channel=%d", ids[0], ids[1])
		}
	}
}
