package reactions

import (
	"context"
	"fmt"

	"github.com/ofabiodev/osmose/internal/rpc"
	"github.com/ofabiodev/osmose/proto/core"
	protoReactions "github.com/ofabiodev/osmose/proto/reactions"
	"github.com/ofabiodev/osmose/types"
	"google.golang.org/protobuf/proto"
)

// Service provides adding and removing message reactions.
type Service struct {
	call func(context.Context, proto.Message) (*core.RPCResult, error)
}

func New(call func(context.Context, proto.Message) (*core.RPCResult, error)) *Service {
	return &Service{call: call}
}

type Emoji struct {
	Unicode string
	Custom  types.ID
}

type Params struct {
	Chat      types.ChatRef
	MessageID types.ID
	Emoji     Emoji
}

func (s *Service) Add(ctx context.Context, params Params) error {
	if params.MessageID == 0 {
		return fmt.Errorf("message ID is required")
	}
	chat, err := params.Chat.ToProto()
	if err != nil {
		return err
	}
	emoji, err := params.Emoji.toProto()
	if err != nil {
		return err
	}
	result, err := s.do(ctx, &protoReactions.AddReaction{ChatRef: chat, MessageId: uint64(params.MessageID), Emoji: emoji})
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "reactions.addReaction")
}

func (s *Service) Remove(ctx context.Context, params Params) error {
	if params.MessageID == 0 {
		return fmt.Errorf("message ID is required")
	}
	chat, err := params.Chat.ToProto()
	if err != nil {
		return err
	}
	emoji, err := params.Emoji.toProto()
	if err != nil {
		return err
	}
	result, err := s.do(ctx, &protoReactions.RemoveReaction{ChatRef: chat, MessageId: uint64(params.MessageID), Emoji: emoji})
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "reactions.removeReaction")
}

func (e Emoji) toProto() (*protoReactions.ReactionEmoji, error) {
	if e.Unicode != "" && e.Custom != 0 {
		return nil, fmt.Errorf("reaction emoji must be unicode or custom")
	}
	if e.Unicode != "" {
		return &protoReactions.ReactionEmoji{Emoji: &protoReactions.ReactionEmoji_UnicodeEmoji{UnicodeEmoji: e.Unicode}}, nil
	}
	if e.Custom != 0 {
		return &protoReactions.ReactionEmoji{Emoji: &protoReactions.ReactionEmoji_CustomEmoji{CustomEmoji: uint64(e.Custom)}}, nil
	}
	return nil, fmt.Errorf("reaction emoji is required")
}

func (s *Service) do(ctx context.Context, request proto.Message) (*core.RPCResult, error) {
	if s == nil || s.call == nil {
		return nil, fmt.Errorf("reactions service is not initialized")
	}
	return s.call(ctx, request)
}
