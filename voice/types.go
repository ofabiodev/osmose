package voice

import (
	protoVoice "github.com/ofabiodev/osmose/proto/voice"
	"github.com/ofabiodev/osmose/types"
)

// Room contains the control-plane details returned when joining a voice room.
type Room struct {
	ID       types.ID
	Chat     types.ChatRef
	Endpoint string
	Token    string
	Raw      *protoVoice.RoomInfo
}

// Participant is a participant's current voice state.
type Participant struct {
	RoomID          types.ID
	SessionID       string
	UserID          types.ID
	Muted           bool
	Deafened        bool
	VideoAvailable  bool
	ScreenAvailable bool
	Banner          *types.ChatPhoto
	Raw             *protoVoice.RoomParticipant
}

// RoomState is the current state of one voice room.
type RoomState struct {
	ID           types.ID
	Participants []*Participant
	Raw          *protoVoice.RoomState
}

// RoomStateEntry associates a room state with its chat.
type RoomStateEntry struct {
	Chat  types.ChatRef
	State *RoomState
	Raw   *protoVoice.GlobalRoomStates_Entry
}

// RoomStates is the set of visible voice rooms.
type RoomStates struct {
	Rooms []*RoomStateEntry
	Raw   *protoVoice.GlobalRoomStates
}

func RoomFromProto(value *protoVoice.RoomInfo) *Room {
	if value == nil {
		return nil
	}
	return &Room{
		ID:       types.ID(value.GetRoomId()),
		Chat:     types.ChatRefFromProto(value.GetChatRef()),
		Endpoint: value.GetEndpoint(),
		Token:    value.GetToken(),
		Raw:      value,
	}
}

func ParticipantFromProto(value *protoVoice.RoomParticipant) *Participant {
	if value == nil {
		return nil
	}
	return &Participant{
		RoomID:          types.ID(value.GetRoomId()),
		SessionID:       value.GetSessionId(),
		UserID:          types.ID(value.GetUserId()),
		Muted:           value.GetMuted(),
		Deafened:        value.GetDeafened(),
		VideoAvailable:  value.GetVideoAvailable(),
		ScreenAvailable: value.GetScreenAvailable(),
		Banner:          types.ChatPhotoFromProto(value.GetBanner()),
		Raw:             value,
	}
}

func RoomStateFromProto(value *protoVoice.RoomState) *RoomState {
	if value == nil {
		return nil
	}
	state := &RoomState{ID: types.ID(value.GetRoomId()), Raw: value}
	for _, participant := range value.GetParticipants() {
		state.Participants = append(state.Participants, ParticipantFromProto(participant))
	}
	return state
}

func RoomStatesFromProto(value *protoVoice.GlobalRoomStates) *RoomStates {
	if value == nil {
		return nil
	}
	states := &RoomStates{Raw: value}
	for _, entry := range value.GetRoomStates() {
		if entry == nil {
			continue
		}
		states.Rooms = append(states.Rooms, &RoomStateEntry{
			Chat:  types.ChatRefFromProto(entry.GetChatRef()),
			State: RoomStateFromProto(entry.GetState()),
			Raw:   entry,
		})
	}
	return states
}
