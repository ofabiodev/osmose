package voice

import (
	"context"
	"testing"

	"github.com/ofabiodev/osmose/proto/core"
	protoVoice "github.com/ofabiodev/osmose/proto/voice"
	"github.com/ofabiodev/osmose/types"
	"google.golang.org/protobuf/proto"
)

func TestRequestRoomMapsProtocolResponse(t *testing.T) {
	var got proto.Message
	service := New(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
		got = request
		return &core.RPCResult{Result: &core.RPCResult_RoomInfo{RoomInfo: &protoVoice.RoomInfo{
			RoomId: 4, Endpoint: "voice.example", Token: "secret",
		}}}, nil
	})
	room, err := service.RequestRoom(context.Background(), types.SelfChat())
	if err != nil {
		t.Fatal(err)
	}
	request, ok := got.(*protoVoice.RequestRoom)
	if !ok || request.GetChatRef().GetSelf() == nil {
		t.Fatalf("unexpected request: %#v", got)
	}
	if room.ID != 4 || room.Endpoint != "voice.example" || room.Token != "secret" || room.Raw == nil {
		t.Fatalf("unexpected room: %#v", room)
	}
}

func TestRoomStatesMapsParticipants(t *testing.T) {
	service := New(func(context.Context, proto.Message) (*core.RPCResult, error) {
		return &core.RPCResult{Result: &core.RPCResult_GlobalRoomStates{GlobalRoomStates: &protoVoice.GlobalRoomStates{
			RoomStates: []*protoVoice.GlobalRoomStates_Entry{{
				State: &protoVoice.RoomState{RoomId: 5, Participants: []*protoVoice.RoomParticipant{{UserId: 6, Muted: true}}},
			}},
		}}}, nil
	})
	states, err := service.RoomStates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(states.Rooms) != 1 || states.Rooms[0].State.ID != 5 || len(states.Rooms[0].State.Participants) != 1 || states.Rooms[0].State.Participants[0].UserID != 6 || !states.Rooms[0].State.Participants[0].Muted {
		t.Fatalf("unexpected room states: %#v", states)
	}
}

func TestDisconnectUserValidatesAndBuildsRequest(t *testing.T) {
	calls := 0
	service := New(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
		calls++
		value, ok := request.(*protoVoice.DisconnectUser)
		if !ok || value.GetUserId() != 7 || value.GetChatRef().GetSelf() == nil {
			t.Fatalf("unexpected request: %#v", request)
		}
		return &core.RPCResult{}, nil
	})
	if err := service.DisconnectUser(context.Background(), DisconnectParams{Chat: types.SelfChat(), UserID: 7}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d, want 1", calls)
	}
	if err := service.DisconnectUser(context.Background(), DisconnectParams{Chat: types.SelfChat()}); err == nil {
		t.Fatal("zero user ID was accepted")
	}
}
