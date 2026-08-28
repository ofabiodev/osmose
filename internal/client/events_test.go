package client

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	protoReactions "github.com/ofabiodev/osmose/proto/reactions"
	protoRefs "github.com/ofabiodev/osmose/proto/refs"
	protoTypes "github.com/ofabiodev/osmose/proto/types"
	"github.com/ofabiodev/osmose/proto/updates"
	protoVoice "github.com/ofabiodev/osmose/proto/voice"
	modelTypes "github.com/ofabiodev/osmose/types"
)

func TestEventHandlerCanBeRemoved(t *testing.T) {
	dispatcher := newEventDispatcher(1, 1, nil)
	dispatcher.client = &Client{}
	called := false
	remove := dispatcher.onMessageCreate(func(context.Context, *MessageCreateEvent) error {
		called = true
		return nil
	})
	remove()
	dispatcher.dispatch(context.Background(), &updates.Update{Update: &updates.Update_MessageCreated{MessageCreated: &updates.UpdateMessageCreated{}}})
	if called {
		t.Fatal("removed event handler was called")
	}
}

func TestEventDispatcherIgnoresNilTypedHandler(t *testing.T) {
	dispatcher := newEventDispatcher(1, 1, nil)
	dispatcher.client = &Client{}
	reported := make(chan *HandlerError, 1)
	dispatcher.onHandlerError = func(_ context.Context, failure *HandlerError) {
		reported <- failure
	}
	var handler MessageCreateHandler
	dispatcher.onMessageCreate(handler)
	dispatcher.dispatch(context.Background(), &updates.Update{Update: &updates.Update_MessageCreated{MessageCreated: &updates.UpdateMessageCreated{}}})
	select {
	case failure := <-reported:
		t.Fatalf("nil handler was invoked: %#v", failure)
	default:
	}
}

func TestEventDispatcherDispatchesTypedUpdates(t *testing.T) {
	dispatcher := newEventDispatcher(1, 1, nil)
	dispatcher.client = &Client{}
	var (
		channelUpdate    *ChannelUpdateEvent
		channelDelete    *ChannelDeleteEvent
		userUpdate       *UserUpdateEvent
		communityUpdate  *CommunityUpdateEvent
		communityDelete  *CommunityDeleteEvent
		chatTyping       *ChatTypingEvent
		memberUpdate     *MemberUpdateEvent
		memberDelete     *MemberDeleteEvent
		messageReactions *MessageReactionsEvent
		lastRead         *ConversationLastReadEvent
		interaction      *InteractionEvent
		voiceState       *VoiceRoomStateEvent
		voiceParticipant *VoiceRoomParticipantEvent
	)
	dispatcher.onChannelUpdate(func(_ context.Context, event *ChannelUpdateEvent) error { channelUpdate = event; return nil })
	dispatcher.onChannelDelete(func(_ context.Context, event *ChannelDeleteEvent) error { channelDelete = event; return nil })
	dispatcher.onUserUpdate(func(_ context.Context, event *UserUpdateEvent) error { userUpdate = event; return nil })
	dispatcher.onCommunityUpdate(func(_ context.Context, event *CommunityUpdateEvent) error { communityUpdate = event; return nil })
	dispatcher.onCommunityDelete(func(_ context.Context, event *CommunityDeleteEvent) error { communityDelete = event; return nil })
	dispatcher.onChatTyping(func(_ context.Context, event *ChatTypingEvent) error { chatTyping = event; return nil })
	dispatcher.onMemberUpdate(func(_ context.Context, event *MemberUpdateEvent) error { memberUpdate = event; return nil })
	dispatcher.onMemberDelete(func(_ context.Context, event *MemberDeleteEvent) error { memberDelete = event; return nil })
	dispatcher.onMessageReactions(func(_ context.Context, event *MessageReactionsEvent) error { messageReactions = event; return nil })
	dispatcher.onConversationLastRead(func(_ context.Context, event *ConversationLastReadEvent) error { lastRead = event; return nil })
	dispatcher.onInteraction(func(_ context.Context, event *InteractionEvent) error { interaction = event; return nil })
	dispatcher.onVoiceRoomState(func(_ context.Context, event *VoiceRoomStateEvent) error { voiceState = event; return nil })
	dispatcher.onVoiceRoomParticipant(func(_ context.Context, event *VoiceRoomParticipantEvent) error { voiceParticipant = event; return nil })

	chat, err := modelTypes.SelfChat().ToProto()
	if err != nil {
		t.Fatal(err)
	}
	for _, update := range []*updates.Update{
		{Update: &updates.Update_Channel{Channel: &updates.UpdateChannel{Channel: &protoTypes.Channel{Id: 1}}}},
		{Update: &updates.Update_ChannelDeleted{ChannelDeleted: &updates.UpdateChannelDeleted{Channel: &protoRefs.ChannelRef{CommunityId: 2, ChannelId: 3}}}},
		{Update: &updates.Update_User{User: &updates.UpdateUser{UserId: 4, User: &protoTypes.User{Id: 4, Name: "User"}}}},
		{Update: &updates.Update_Community{Community: &updates.UpdateCommunity{CommunityId: 5, Community: &protoTypes.Community{Id: 5, Name: "Community"}}}},
		{Update: &updates.Update_CommunityDeleted{CommunityDeleted: &updates.UpdateCommunityDeleted{CommunityId: 6}}},
		{Update: &updates.Update_ChatTyping{ChatTyping: &updates.UpdateChatTyping{ChatRef: chat, UserId: 7, Typing: true}}},
		{Update: &updates.Update_CommunityMember{CommunityMember: &updates.UpdateCommunityMember{CommunityId: 8, MemberId: 9, Member: &protoTypes.CommunityMember{Id: 9}}}},
		{Update: &updates.Update_CommunityMemberDeleted{CommunityMemberDeleted: &updates.UpdateCommunityMemberDeleted{CommunityId: 8, MemberId: 9}}},
		{Update: &updates.Update_MessageReactions{MessageReactions: &updates.UpdateMessageReactions{ChatRef: chat, Reactions: &protoReactions.MessageReactions{}}}},
		{Update: &updates.Update_ConversationLastRead{ConversationLastRead: &updates.UpdateConversationLastRead{ChatRef: chat, LastReadMessageId: 10}}},
		{Update: &updates.Update_Interaction{Interaction: &updates.UpdateInteraction{InteractionId: 11, UserId: 12, MessageId: 13, Data: protoString("confirm")}}},
		{Update: &updates.Update_RoomState{RoomState: &protoVoice.UpdateRoomState{ChatRef: chat, State: &protoVoice.RoomState{RoomId: 14, Participants: []*protoVoice.RoomParticipant{{UserId: 15}}}}}},
		{Update: &updates.Update_RoomParticipant{RoomParticipant: &protoVoice.UpdateRoomParticipant{ChatRef: chat, Participant: &protoVoice.RoomParticipant{RoomId: 14, UserId: 16, Muted: true}}}},
	} {
		dispatcher.dispatch(context.Background(), update)
	}

	if channelUpdate == nil || channelUpdate.Channel.ID != 1 || channelUpdate.Channel.Raw == nil {
		t.Fatal("channel update was not dispatched")
	}
	if channelDelete == nil || channelDelete.Channel.CommunityID != 2 || channelDelete.Channel.ChannelID != 3 {
		t.Fatal("channel delete was not dispatched")
	}
	if userUpdate == nil || userUpdate.UserID != 4 || userUpdate.User.ID != 4 {
		t.Fatal("user update was not dispatched")
	}
	if communityUpdate == nil || communityUpdate.CommunityID != 5 || communityUpdate.Community.Name != "Community" || communityUpdate.Community.Raw == nil {
		t.Fatal("community update was not dispatched")
	}
	if communityDelete == nil || communityDelete.CommunityID != 6 {
		t.Fatal("community delete was not dispatched")
	}
	if chatTyping == nil || !chatTyping.Chat.Self || chatTyping.UserID != 7 || !chatTyping.Typing {
		t.Fatal("typing update was not dispatched")
	}
	if memberUpdate == nil || memberUpdate.CommunityID != 8 || memberUpdate.MemberID != 9 || memberUpdate.Member.ID != 9 || memberUpdate.Member.Raw == nil {
		t.Fatal("member update was not dispatched")
	}
	if memberDelete == nil || memberDelete.CommunityID != 8 || memberDelete.MemberID != 9 {
		t.Fatal("member delete was not dispatched")
	}
	if messageReactions == nil || !messageReactions.Chat.Self || messageReactions.Reactions == nil {
		t.Fatal("message reactions update was not dispatched")
	}
	if lastRead == nil || !lastRead.Chat.Self || lastRead.LastReadMessageID != 10 {
		t.Fatal("last read update was not dispatched")
	}
	if interaction == nil || interaction.Interaction == nil || interaction.ID != 11 || interaction.Interaction.UserID != 12 || interaction.Data != "confirm" || interaction.Interaction.Raw == nil {
		t.Fatal("interaction update was not dispatched")
	}
	if voiceState == nil || !voiceState.Chat.Self || voiceState.State == nil || voiceState.State.ID != 14 || len(voiceState.State.Participants) != 1 {
		t.Fatal("voice room state update was not dispatched")
	}
	if voiceParticipant == nil || !voiceParticipant.Chat.Self || voiceParticipant.Participant == nil || voiceParticipant.Participant.UserID != 16 || !voiceParticipant.Participant.Muted {
		t.Fatal("voice participant update was not dispatched")
	}
}

func protoString(value string) *string { return &value }

func TestConnectionLifecycleListeners(t *testing.T) {
	dispatcher := newEventDispatcher(1, 1, nil)
	dispatcher.client = &Client{}
	var got []string
	dispatcher.onConnecting(func(context.Context, *ConnectionEvent) error { got = append(got, "connecting"); return nil })
	dispatcher.onConnected(func(context.Context, *ConnectionEvent) error { got = append(got, "connected"); return nil })
	dispatcher.onDisconnected(func(context.Context, *ConnectionEvent) error { got = append(got, "disconnected"); return nil })
	dispatcher.onReconnecting(func(context.Context, *ConnectionEvent) error { got = append(got, "reconnecting"); return nil })
	dispatcher.onConnectionError(func(context.Context, *ConnectionEvent) error { got = append(got, "error"); return nil })
	ctx := context.Background()
	event := &ConnectionEvent{Attempt: 2, RetryIn: time.Second, State: Disconnected}
	dispatcher.emitConnecting(ctx, event)
	dispatcher.emitConnected(ctx, event)
	dispatcher.emitConnectionError(ctx, event)
	dispatcher.emitDisconnected(ctx, event)
	dispatcher.emitReconnecting(ctx, event)
	if want := "connecting,connected,error,disconnected,reconnecting"; strings.Join(got, ",") != want {
		t.Fatalf("lifecycle events=%v, want %s", got, want)
	}
}

func BenchmarkEventDispatch(b *testing.B) {
	dispatcher := newEventDispatcher(8, 1, nil)
	dispatcher.client = &Client{}
	dispatcher.onMessageCreate(func(context.Context, *MessageCreateEvent) error { return nil })
	update := &updates.Update{Update: &updates.Update_MessageCreated{MessageCreated: &updates.UpdateMessageCreated{Message: &protoTypes.Message{MessageId: 1, Message: "hello"}}}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		dispatcher.dispatch(context.Background(), update)
	}
}

func TestEventDispatcherReportsHandlerErrors(t *testing.T) {
	dispatcher := newEventDispatcher(1, 1, nil)
	dispatcher.client = &Client{}
	wantErr := errors.New("handler failed")
	reported := make(chan *HandlerError, 2)
	dispatcher.onHandlerError = func(_ context.Context, failure *HandlerError) {
		reported <- failure
	}
	dispatcher.onMessageCreate(func(_ context.Context, _ *MessageCreateEvent) error {
		return wantErr
	})
	dispatcher.dispatch(context.Background(), &updates.Update{Update: &updates.Update_MessageCreated{MessageCreated: &updates.UpdateMessageCreated{}}})
	select {
	case failure := <-reported:
		if failure.Event != "message_create" || !errors.Is(failure.Err, wantErr) || failure.Panic != nil {
			t.Fatalf("unexpected handler failure: %#v", failure)
		}
	case <-time.After(time.Second):
		t.Fatal("handler error was not reported")
	}

	dispatcher.onMessageUpdate(func(context.Context, *MessageUpdateEvent) error {
		panic("boom")
	})
	dispatcher.dispatch(context.Background(), &updates.Update{Update: &updates.Update_Message{Message: &updates.UpdateMessage{}}})
	select {
	case failure := <-reported:
		if failure.Event != "message_update" || failure.Panic != "boom" || len(failure.Stack) == 0 {
			t.Fatalf("unexpected panic report: %#v", failure)
		}
	case <-time.After(time.Second):
		t.Fatal("handler panic was not reported")
	}
}

func TestEventDispatcherReportsQueueOverflow(t *testing.T) {
	dispatcher := newEventDispatcher(1, 1, nil)
	entered := make(chan struct{})
	release := make(chan struct{})
	overflow := make(chan uint64, 1)
	var first sync.Once
	dispatcher.onEventOverflow = func(dropped uint64) { overflow <- dropped }
	dispatcher.onMessageCreate(func(context.Context, *MessageCreateEvent) error {
		first.Do(func() {
			close(entered)
			<-release
		})
		return nil
	})
	dispatcher.start(context.Background())
	update := &updates.Update{Update: &updates.Update_MessageCreated{MessageCreated: &updates.UpdateMessageCreated{}}}
	if err := dispatcher.enqueue(context.Background(), update); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("event worker did not start")
	}
	if err := dispatcher.enqueue(context.Background(), update); err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.enqueue(context.Background(), update); !errors.Is(err, ErrEventQueueFull) {
		t.Fatalf("unexpected overflow error: %v", err)
	}
	if got := dispatcher.dropped.Load(); got != 1 {
		t.Fatalf("dropped=%d, want 1", got)
	}
	select {
	case dropped := <-overflow:
		if dropped != 1 {
			t.Fatalf("callback dropped=%d, want 1", dropped)
		}
	case <-time.After(time.Second):
		t.Fatal("overflow callback was not called")
	}
	close(release)
	dispatcher.close()
}

func TestEventDispatcherCanCloseWhileHandlerRegisters(t *testing.T) {
	dispatcher := newEventDispatcher(1, 1, nil)
	dispatcher.start(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		dispatcher.onMessageCreate(func(context.Context, *MessageCreateEvent) error { return nil })
	}()
	dispatcher.close()
	wg.Wait()
}
