// Package voice exposes the voice control-plane operations present in the
// Osmium protocol. It does not implement an audio transport.
package voice

import (
	"context"
	"fmt"

	"github.com/ofabiodev/osmose/internal/rpc"
	"github.com/ofabiodev/osmose/proto/core"
	protoVoice "github.com/ofabiodev/osmose/proto/voice"
	"github.com/ofabiodev/osmose/types"
	"google.golang.org/protobuf/proto"
)

// Service provides voice room and participant operations.
type Service struct {
	call func(context.Context, proto.Message) (*core.RPCResult, error)
}

func New(call func(context.Context, proto.Message) (*core.RPCResult, error)) *Service {
	return &Service{call: call}
}

// DisconnectParams identifies a user to remove from a voice room.
type DisconnectParams struct {
	Chat   types.ChatRef
	UserID types.ID
}

// RequestRoom asks Osmium for the room connection details for a chat.
func (s *Service) RequestRoom(ctx context.Context, chat types.ChatRef) (*Room, error) {
	ref, err := chat.ToProto()
	if err != nil {
		return nil, err
	}
	result, err := s.do(ctx, &protoVoice.RequestRoom{ChatRef: ref})
	if err != nil {
		return nil, err
	}
	value := result.GetRoomInfo()
	if value == nil {
		return nil, &rpc.UnexpectedResultError{Method: "voice.requestRoom"}
	}
	return RoomFromProto(value), nil
}

// RoomStates returns the current voice room state visible to the session.
func (s *Service) RoomStates(ctx context.Context) (*RoomStates, error) {
	result, err := s.do(ctx, &protoVoice.GetRoomStates{})
	if err != nil {
		return nil, err
	}
	value := result.GetGlobalRoomStates()
	if value == nil {
		return nil, &rpc.UnexpectedResultError{Method: "voice.getRoomStates"}
	}
	return RoomStatesFromProto(value), nil
}

// DisconnectUser removes a participant from a voice room.
func (s *Service) DisconnectUser(ctx context.Context, params DisconnectParams) error {
	if params.UserID == 0 {
		return fmt.Errorf("user ID is required")
	}
	ref, err := params.Chat.ToProto()
	if err != nil {
		return err
	}
	result, err := s.do(ctx, &protoVoice.DisconnectUser{ChatRef: ref, UserId: uint64(params.UserID)})
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "voice.disconnectUser")
}

func (s *Service) do(ctx context.Context, request proto.Message) (*core.RPCResult, error) {
	if s == nil || s.call == nil {
		return nil, fmt.Errorf("voice service is not initialized")
	}
	return s.call(ctx, request)
}
