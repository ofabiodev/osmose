package chats

import (
	"context"
	"fmt"

	"github.com/ofabiodev/osmose/internal/rpc"
	protoChats "github.com/ofabiodev/osmose/proto/chats"
	"github.com/ofabiodev/osmose/proto/core"
	"github.com/ofabiodev/osmose/types"
	"google.golang.org/protobuf/proto"
)

// Service provides chat lookups and the small set of chat mutations useful to
// bots.
type Service struct {
	call func(context.Context, proto.Message) (*core.RPCResult, error)
}

func New(call func(context.Context, proto.Message) (*core.RPCResult, error)) *Service {
	return &Service{call: call}
}

type ListParams struct {
	Limit uint32
	MaxID types.ID
	MinID types.ID
}

type ListResult struct {
	Chats    []*types.Conversation
	Users    []*types.User
	Groups   []*types.Group
	Channels []*types.Channel
	Messages []*types.Message
	Raw      *protoChats.Chats
}

type Chat struct {
	Conversation *types.Conversation
	Message      *types.Message
	Users        []*types.User
	Group        *types.Group
	Channel      *types.Channel
	Raw          *protoChats.Chat
}

type Members struct {
	Members []*types.ChatMember
	Users   []*types.User
	Raw     *protoChats.ChatMembers
}

func (s *Service) List(ctx context.Context, params ListParams) (*ListResult, error) {
	request := &protoChats.GetChats{}
	if params.Limit != 0 {
		request.Limit = &params.Limit
	}
	if params.MaxID != 0 {
		maxID := uint64(params.MaxID)
		request.MaxId = &maxID
	}
	if params.MinID != 0 {
		minID := uint64(params.MinID)
		request.MinId = &minID
	}
	result, err := s.do(ctx, request)
	if err != nil {
		return nil, err
	}
	value := result.GetChats()
	if value == nil {
		return nil, &rpc.UnexpectedResultError{Method: "chats.getChats"}
	}
	response := &ListResult{Raw: value}
	users := make(map[types.ID]*types.User, len(value.GetUsers()))
	for _, user := range value.GetUsers() {
		model := types.UserFromProto(user)
		response.Users = append(response.Users, model)
		if model != nil {
			users[model.ID] = model
		}
	}
	for _, chat := range value.GetChats() {
		response.Chats = append(response.Chats, types.ConversationFromProto(chat))
	}
	for _, group := range value.GetGroups() {
		response.Groups = append(response.Groups, types.GroupFromProto(group))
	}
	for _, channel := range value.GetChannels() {
		response.Channels = append(response.Channels, types.ChannelFromProto(channel))
	}
	for _, message := range value.GetMessages() {
		model := types.MessageFromProto(message)
		if model != nil {
			model.Author = users[model.AuthorID]
		}
		response.Messages = append(response.Messages, model)
	}
	return response, nil
}

func (s *Service) Get(ctx context.Context, ref types.ChatRef) (*Chat, error) {
	chat, err := ref.ToProto()
	if err != nil {
		return nil, err
	}
	result, err := s.do(ctx, &protoChats.GetChat{ChatRef: chat})
	if err != nil {
		return nil, err
	}
	value := result.GetChat()
	if value == nil {
		return nil, &rpc.UnexpectedResultError{Method: "chats.getChat"}
	}
	response := &Chat{
		Conversation: types.ConversationFromProto(value.GetChat()),
		Message:      types.MessageFromProto(value.GetMessage()),
		Group:        types.GroupFromProto(value.GetGroup()),
		Channel:      types.ChannelFromProto(value.GetChannel()),
		Raw:          value,
	}
	users := make(map[types.ID]*types.User, len(value.GetUsers()))
	for _, user := range value.GetUsers() {
		model := types.UserFromProto(user)
		response.Users = append(response.Users, model)
		if model != nil {
			users[model.ID] = model
		}
	}
	if response.Message != nil {
		response.Message.Author = users[response.Message.AuthorID]
	}
	return response, nil
}

// Members returns members for private or group chats. For a community
// channel, use communities.Service.ChannelMembers instead.
func (s *Service) Members(ctx context.Context, ref types.ChatRef) (*Members, error) {
	chat, err := ref.ToProto()
	if err != nil {
		return nil, err
	}
	result, err := s.do(ctx, &protoChats.GetChatMembers{ChatRef: chat})
	if err != nil {
		return nil, err
	}
	value := result.GetChatMembers()
	if value == nil {
		return nil, &rpc.UnexpectedResultError{Method: "chats.getChatMembers"}
	}
	response := &Members{Raw: value}
	for _, member := range value.GetMembers() {
		response.Members = append(response.Members, types.ChatMemberFromProto(member))
	}
	for _, user := range value.GetUsers() {
		response.Users = append(response.Users, types.UserFromProto(user))
	}
	return response, nil
}

func (s *Service) SetTyping(ctx context.Context, ref types.ChatRef, typing bool) error {
	chat, err := ref.ToProto()
	if err != nil {
		return err
	}
	result, err := s.do(ctx, &protoChats.SetTyping{ChatRef: chat, Typing: typing})
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "chats.setTyping")
}

func (s *Service) do(ctx context.Context, request proto.Message) (*core.RPCResult, error) {
	if s == nil || s.call == nil {
		return nil, fmt.Errorf("chats service is not initialized")
	}
	return s.call(ctx, request)
}
