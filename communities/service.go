package communities

import (
	"context"
	"fmt"

	"github.com/ofabiodev/osmose/internal/rpc"
	protoCommunities "github.com/ofabiodev/osmose/proto/communities"
	"github.com/ofabiodev/osmose/proto/core"
	"github.com/ofabiodev/osmose/types"
	"google.golang.org/protobuf/proto"
)

// Service provides community and channel lookups.
type Service struct {
	call func(context.Context, proto.Message) (*core.RPCResult, error)
}

func New(call func(context.Context, proto.Message) (*core.RPCResult, error)) *Service {
	return &Service{call: call}
}

type ListResult struct {
	Communities []*types.Community
	Raw         *protoCommunities.Communities
}

type ChannelsResult struct {
	Conversations []*types.Conversation
	Channels      []*types.Channel
	Messages      []*types.Message
	Raw           *protoCommunities.Channels
}

// ChannelMembersResult contains the ordered member-list entries for a
// community channel.
type ChannelMembersResult struct {
	Entries []*types.MemberListEntry
	Raw     *protoCommunities.MemberList
}

func (s *Service) List(ctx context.Context) (*ListResult, error) {
	result, err := s.do(ctx, &protoCommunities.GetCommunities{})
	if err != nil {
		return nil, err
	}
	value := result.GetCommunities()
	if value == nil {
		return nil, &rpc.UnexpectedResultError{Method: "communities.getCommunities"}
	}
	response := &ListResult{Raw: value}
	for _, community := range value.GetCommunities() {
		response.Communities = append(response.Communities, types.CommunityFromProto(community))
	}
	return response, nil
}

func (s *Service) Channels(ctx context.Context, communityID types.ID) (*ChannelsResult, error) {
	if communityID == 0 {
		return nil, fmt.Errorf("community ID is required")
	}
	result, err := s.do(ctx, &protoCommunities.GetChannels{CommunityId: uint64(communityID)})
	if err != nil {
		return nil, err
	}
	value := result.GetChannels()
	if value == nil {
		return nil, &rpc.UnexpectedResultError{Method: "communities.getChannels"}
	}
	response := &ChannelsResult{Raw: value}
	for _, conversation := range value.GetConversations() {
		response.Conversations = append(response.Conversations, types.ConversationFromProto(conversation))
	}
	for _, channel := range value.GetChannels() {
		response.Channels = append(response.Channels, types.ChannelFromProto(channel))
	}
	for _, message := range value.GetMessages() {
		response.Messages = append(response.Messages, types.MessageFromProto(message))
	}
	return response, nil
}

// ChannelMembers returns the member list visible in a community channel.
func (s *Service) ChannelMembers(ctx context.Context, communityID, channelID types.ID) (*ChannelMembersResult, error) {
	if communityID == 0 {
		return nil, fmt.Errorf("community ID is required")
	}
	if channelID == 0 {
		return nil, fmt.Errorf("channel ID is required")
	}
	result, err := s.do(ctx, &protoCommunities.GetChannelMembers{
		CommunityId: uint64(communityID),
		ChannelId:   uint64(channelID),
	})
	if err != nil {
		return nil, err
	}
	value := result.GetMemberList()
	if value == nil {
		return nil, &rpc.UnexpectedResultError{Method: "communities.getChannelMembers"}
	}
	response := &ChannelMembersResult{Raw: value}
	for _, entry := range value.GetEntries() {
		response.Entries = append(response.Entries, types.MemberListEntryFromProto(entry))
	}
	return response, nil
}

func (s *Service) do(ctx context.Context, request proto.Message) (*core.RPCResult, error) {
	if s == nil || s.call == nil {
		return nil, fmt.Errorf("communities service is not initialized")
	}
	return s.call(ctx, request)
}
