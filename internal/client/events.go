package client

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"

	eventtypes "github.com/ofabiodev/osmose/events"
	"github.com/ofabiodev/osmose/internal/dispatcher"
	"github.com/ofabiodev/osmose/proto/updates"
	"github.com/ofabiodev/osmose/types"
	"github.com/ofabiodev/osmose/voice"
)

type ReadyHandler = eventtypes.ReadyHandler
type MessageCreateHandler = eventtypes.MessageCreateHandler
type MessageUpdateHandler = eventtypes.MessageUpdateHandler
type MessageDeleteHandler = eventtypes.MessageDeleteHandler
type MemberCreateHandler = eventtypes.MemberCreateHandler
type ChannelUpdateHandler = eventtypes.ChannelUpdateHandler
type ChannelDeleteHandler = eventtypes.ChannelDeleteHandler
type UserUpdateHandler = eventtypes.UserUpdateHandler
type CommunityUpdateHandler = eventtypes.CommunityUpdateHandler
type CommunityDeleteHandler = eventtypes.CommunityDeleteHandler
type ChatTypingHandler = eventtypes.ChatTypingHandler
type MemberUpdateHandler = eventtypes.MemberUpdateHandler
type MemberDeleteHandler = eventtypes.MemberDeleteHandler
type MessageReactionsHandler = eventtypes.MessageReactionsHandler
type ConversationLastReadHandler = eventtypes.ConversationLastReadHandler
type InteractionHandler = eventtypes.InteractionHandler
type VoiceRoomStateHandler = eventtypes.VoiceRoomStateHandler
type VoiceRoomParticipantHandler = eventtypes.VoiceRoomParticipantHandler
type ConnectionHandler = eventtypes.ConnectionHandler
type UpdateHandler = eventtypes.UpdateHandler
type HandlerError = eventtypes.HandlerError
type HandlerErrorHandler = eventtypes.HandlerErrorHandler
type EventOverflowHandler = eventtypes.EventOverflowHandler
type ConnectionEvent = eventtypes.ConnectionEvent
type ReadyEvent = eventtypes.ReadyEvent
type MessageCreateEvent = eventtypes.MessageCreateEvent
type MessageUpdateEvent = eventtypes.MessageUpdateEvent
type MessageDeleteEvent = eventtypes.MessageDeleteEvent
type MemberCreateEvent = eventtypes.MemberCreateEvent
type ChannelUpdateEvent = eventtypes.ChannelUpdateEvent
type ChannelDeleteEvent = eventtypes.ChannelDeleteEvent
type UserUpdateEvent = eventtypes.UserUpdateEvent
type CommunityUpdateEvent = eventtypes.CommunityUpdateEvent
type CommunityDeleteEvent = eventtypes.CommunityDeleteEvent
type ChatTypingEvent = eventtypes.ChatTypingEvent
type MemberUpdateEvent = eventtypes.MemberUpdateEvent
type MemberDeleteEvent = eventtypes.MemberDeleteEvent
type MessageReactionsEvent = eventtypes.MessageReactionsEvent
type ConversationLastReadEvent = eventtypes.ConversationLastReadEvent
type InteractionEvent = eventtypes.InteractionEvent
type VoiceRoomStateEvent = eventtypes.VoiceRoomStateEvent
type VoiceRoomParticipantEvent = eventtypes.VoiceRoomParticipantEvent
type UpdateEvent = eventtypes.UpdateEvent

var ErrEventQueueFull = eventtypes.ErrEventQueueFull

func newEventBase(client *Client) eventtypes.Base {
	return eventtypes.NewBase(client,
		func(ctx context.Context, message *types.Message, content string) error {
			_, err := client.Messages.Reply(ctx, message, content)
			return err
		},
		client.replyInteraction,
		client.acknowledgeInteraction,
	)
}

type listener[T any] struct {
	id uint64
	fn T
}

type queuedEvent struct {
	ctx             context.Context
	update          *updates.Update
	ready           *ReadyEvent
	connectionName  string
	connectionEvent *ConnectionEvent
}

type eventDispatcher struct {
	queue  *dispatcher.Dispatcher[queuedEvent]
	logger *slog.Logger
	client *Client
	base   eventtypes.Base

	mu               sync.RWMutex
	ready            []listener[ReadyHandler]
	messageCreate    []listener[MessageCreateHandler]
	messageUpdate    []listener[MessageUpdateHandler]
	messageDelete    []listener[MessageDeleteHandler]
	memberCreate     []listener[MemberCreateHandler]
	channelUpdate    []listener[ChannelUpdateHandler]
	channelDelete    []listener[ChannelDeleteHandler]
	userUpdate       []listener[UserUpdateHandler]
	communityUpdate  []listener[CommunityUpdateHandler]
	communityDelete  []listener[CommunityDeleteHandler]
	chatTyping       []listener[ChatTypingHandler]
	memberUpdate     []listener[MemberUpdateHandler]
	memberDelete     []listener[MemberDeleteHandler]
	messageReactions []listener[MessageReactionsHandler]
	lastRead         []listener[ConversationLastReadHandler]
	interaction      []listener[InteractionHandler]
	voiceRoomState   []listener[VoiceRoomStateHandler]
	voiceParticipant []listener[VoiceRoomParticipantHandler]
	connecting       []listener[ConnectionHandler]
	connected        []listener[ConnectionHandler]
	disconnected     []listener[ConnectionHandler]
	reconnecting     []listener[ConnectionHandler]
	connectionError  []listener[ConnectionHandler]
	update           []listener[UpdateHandler]
	nextID           atomic.Uint64
	dropped          atomic.Uint64
	onHandlerError   HandlerErrorHandler
	onEventOverflow  EventOverflowHandler
}

func newEventDispatcher(queueSize, workers int, logger *slog.Logger) *eventDispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	d := &eventDispatcher{logger: logger}
	d.queue = dispatcher.New(queueSize, workers, d.dispatchQueued)
	return d
}

func (d *eventDispatcher) setClient(client *Client) {
	d.client = client
	d.base = newEventBase(client)
}

func (d *eventDispatcher) start(parent context.Context) {
	d.queue.Start(parent)
}

func (d *eventDispatcher) enqueue(ctx context.Context, update *updates.Update) error {
	if update == nil {
		return nil
	}
	return d.enqueueItem(ctx, queuedEvent{ctx: ctx, update: update})
}

func (d *eventDispatcher) enqueueReady(ctx context.Context, event *ReadyEvent) error {
	if event == nil {
		return nil
	}
	return d.enqueueItem(ctx, queuedEvent{ctx: ctx, ready: event})
}

func (d *eventDispatcher) enqueueConnection(ctx context.Context, name string, event *ConnectionEvent) error {
	if event == nil {
		return nil
	}
	return d.enqueueItem(ctx, queuedEvent{ctx: ctx, connectionName: name, connectionEvent: event})
}

func (d *eventDispatcher) enqueueItem(ctx context.Context, event queuedEvent) error {
	event.ctx = ctx
	err := d.queue.Enqueue(ctx, event)
	if errors.Is(err, dispatcher.ErrQueueFull) {
		dropped := d.dropped.Add(1)
		d.reportOverflow(dropped)
		return ErrEventQueueFull
	}
	return err
}

func (d *eventDispatcher) close() {
	d.queue.Close()
}

func (d *eventDispatcher) onReady(fn ReadyHandler) func() {
	return addListener(d, &d.ready, fn, fn != nil)
}

func (d *eventDispatcher) onMessageCreate(fn MessageCreateHandler) func() {
	return addListener(d, &d.messageCreate, fn, fn != nil)
}

func (d *eventDispatcher) onMessageUpdate(fn MessageUpdateHandler) func() {
	return addListener(d, &d.messageUpdate, fn, fn != nil)
}

func (d *eventDispatcher) onMessageDelete(fn MessageDeleteHandler) func() {
	return addListener(d, &d.messageDelete, fn, fn != nil)
}

func (d *eventDispatcher) onMemberCreate(fn MemberCreateHandler) func() {
	return addListener(d, &d.memberCreate, fn, fn != nil)
}

func (d *eventDispatcher) onChannelUpdate(fn ChannelUpdateHandler) func() {
	return addListener(d, &d.channelUpdate, fn, fn != nil)
}

func (d *eventDispatcher) onChannelDelete(fn ChannelDeleteHandler) func() {
	return addListener(d, &d.channelDelete, fn, fn != nil)
}

func (d *eventDispatcher) onUserUpdate(fn UserUpdateHandler) func() {
	return addListener(d, &d.userUpdate, fn, fn != nil)
}

func (d *eventDispatcher) onCommunityUpdate(fn CommunityUpdateHandler) func() {
	return addListener(d, &d.communityUpdate, fn, fn != nil)
}

func (d *eventDispatcher) onCommunityDelete(fn CommunityDeleteHandler) func() {
	return addListener(d, &d.communityDelete, fn, fn != nil)
}

func (d *eventDispatcher) onChatTyping(fn ChatTypingHandler) func() {
	return addListener(d, &d.chatTyping, fn, fn != nil)
}

func (d *eventDispatcher) onMemberUpdate(fn MemberUpdateHandler) func() {
	return addListener(d, &d.memberUpdate, fn, fn != nil)
}

func (d *eventDispatcher) onMemberDelete(fn MemberDeleteHandler) func() {
	return addListener(d, &d.memberDelete, fn, fn != nil)
}

func (d *eventDispatcher) onMessageReactions(fn MessageReactionsHandler) func() {
	return addListener(d, &d.messageReactions, fn, fn != nil)
}

func (d *eventDispatcher) onConversationLastRead(fn ConversationLastReadHandler) func() {
	return addListener(d, &d.lastRead, fn, fn != nil)
}

func (d *eventDispatcher) onInteraction(fn InteractionHandler) func() {
	return addListener(d, &d.interaction, fn, fn != nil)
}

func (d *eventDispatcher) onVoiceRoomState(fn VoiceRoomStateHandler) func() {
	return addListener(d, &d.voiceRoomState, fn, fn != nil)
}

func (d *eventDispatcher) onVoiceRoomParticipant(fn VoiceRoomParticipantHandler) func() {
	return addListener(d, &d.voiceParticipant, fn, fn != nil)
}

func (d *eventDispatcher) onConnecting(fn ConnectionHandler) func() {
	return addListener(d, &d.connecting, fn, fn != nil)
}

func (d *eventDispatcher) onConnected(fn ConnectionHandler) func() {
	return addListener(d, &d.connected, fn, fn != nil)
}

func (d *eventDispatcher) onDisconnected(fn ConnectionHandler) func() {
	return addListener(d, &d.disconnected, fn, fn != nil)
}

func (d *eventDispatcher) onReconnecting(fn ConnectionHandler) func() {
	return addListener(d, &d.reconnecting, fn, fn != nil)
}

func (d *eventDispatcher) onConnectionError(fn ConnectionHandler) func() {
	return addListener(d, &d.connectionError, fn, fn != nil)
}

func (d *eventDispatcher) onUpdate(fn UpdateHandler) func() {
	return addListener(d, &d.update, fn, fn != nil)
}

func (d *eventDispatcher) emitReady(ctx context.Context, event *ReadyEvent) {
	d.dispatchReady(ctx, event)
}

func (d *eventDispatcher) dispatchReady(ctx context.Context, event *ReadyEvent) {
	d.mu.RLock()
	listeners := append([]listener[ReadyHandler](nil), d.ready...)
	d.mu.RUnlock()
	for _, item := range listeners {
		d.call("ready", ctx, func(ctx context.Context) error { return item.fn(ctx, event) })
	}
}

func (d *eventDispatcher) emitConnecting(ctx context.Context, event *ConnectionEvent) {
	d.dispatchConnection("connecting", ctx, event)
}

func (d *eventDispatcher) emitConnected(ctx context.Context, event *ConnectionEvent) {
	d.dispatchConnection("connected", ctx, event)
}

func (d *eventDispatcher) emitDisconnected(ctx context.Context, event *ConnectionEvent) {
	d.dispatchConnection("disconnected", ctx, event)
}

func (d *eventDispatcher) emitReconnecting(ctx context.Context, event *ConnectionEvent) {
	d.dispatchConnection("reconnecting", ctx, event)
}

func (d *eventDispatcher) emitConnectionError(ctx context.Context, event *ConnectionEvent) {
	d.dispatchConnection("connection_error", ctx, event)
}

func (d *eventDispatcher) dispatchConnection(name string, ctx context.Context, event *ConnectionEvent) {
	d.mu.RLock()
	var source []listener[ConnectionHandler]
	switch name {
	case "connecting":
		source = d.connecting
	case "connected":
		source = d.connected
	case "disconnected":
		source = d.disconnected
	case "reconnecting":
		source = d.reconnecting
	case "connection_error":
		source = d.connectionError
	}
	listeners := append([]listener[ConnectionHandler](nil), source...)
	d.mu.RUnlock()
	for _, item := range listeners {
		d.call(name, ctx, func(ctx context.Context) error { return item.fn(ctx, event) })
	}
}

func addListener[T any](d *eventDispatcher, list *[]listener[T], fn T, valid bool) func() {
	if !valid {
		return func() {}
	}
	id := d.nextID.Add(1)
	d.mu.Lock()
	*list = append(*list, listener[T]{id: id, fn: fn})
	d.mu.Unlock()
	return func() {
		d.mu.Lock()
		removeListener(list, id)
		d.mu.Unlock()
	}
}

func removeListener[T any](list *[]listener[T], id uint64) {
	items := *list
	for i, item := range items {
		if item.id == id {
			copy(items[i:], items[i+1:])
			*list = items[:len(items)-1]
			return
		}
	}
}

func (d *eventDispatcher) dispatchQueued(ctx context.Context, event queuedEvent) {
	switch {
	case event.update != nil:
		d.dispatch(ctx, event.update)
	case event.ready != nil:
		d.dispatchReady(ctx, event.ready)
	case event.connectionEvent != nil:
		d.dispatchConnection(event.connectionName, ctx, event.connectionEvent)
	}
}

func (d *eventDispatcher) dispatch(ctx context.Context, update *updates.Update) {
	if update == nil {
		return
	}
	d.mu.RLock()
	generic := append([]listener[UpdateHandler](nil), d.update...)
	var (
		messageCreate    []listener[MessageCreateHandler]
		messageUpdate    []listener[MessageUpdateHandler]
		messageDelete    []listener[MessageDeleteHandler]
		memberCreate     []listener[MemberCreateHandler]
		channelUpdate    []listener[ChannelUpdateHandler]
		channelDelete    []listener[ChannelDeleteHandler]
		userUpdate       []listener[UserUpdateHandler]
		communityUpdate  []listener[CommunityUpdateHandler]
		communityDelete  []listener[CommunityDeleteHandler]
		chatTyping       []listener[ChatTypingHandler]
		memberUpdate     []listener[MemberUpdateHandler]
		memberDelete     []listener[MemberDeleteHandler]
		messageReactions []listener[MessageReactionsHandler]
		lastRead         []listener[ConversationLastReadHandler]
		interactions     []listener[InteractionHandler]
		voiceRoomState   []listener[VoiceRoomStateHandler]
		voiceParticipant []listener[VoiceRoomParticipantHandler]
	)
	switch update.GetUpdate().(type) {
	case *updates.Update_MessageCreated:
		messageCreate = append([]listener[MessageCreateHandler](nil), d.messageCreate...)
	case *updates.Update_Message:
		messageUpdate = append([]listener[MessageUpdateHandler](nil), d.messageUpdate...)
	case *updates.Update_MessageDeleted:
		messageDelete = append([]listener[MessageDeleteHandler](nil), d.messageDelete...)
	case *updates.Update_CommunityMemberCreated:
		memberCreate = append([]listener[MemberCreateHandler](nil), d.memberCreate...)
	case *updates.Update_Channel:
		channelUpdate = append([]listener[ChannelUpdateHandler](nil), d.channelUpdate...)
	case *updates.Update_ChannelDeleted:
		channelDelete = append([]listener[ChannelDeleteHandler](nil), d.channelDelete...)
	case *updates.Update_User:
		userUpdate = append([]listener[UserUpdateHandler](nil), d.userUpdate...)
	case *updates.Update_Community:
		communityUpdate = append([]listener[CommunityUpdateHandler](nil), d.communityUpdate...)
	case *updates.Update_CommunityDeleted:
		communityDelete = append([]listener[CommunityDeleteHandler](nil), d.communityDelete...)
	case *updates.Update_ChatTyping:
		chatTyping = append([]listener[ChatTypingHandler](nil), d.chatTyping...)
	case *updates.Update_CommunityMember:
		memberUpdate = append([]listener[MemberUpdateHandler](nil), d.memberUpdate...)
	case *updates.Update_CommunityMemberDeleted:
		memberDelete = append([]listener[MemberDeleteHandler](nil), d.memberDelete...)
	case *updates.Update_MessageReactions:
		messageReactions = append([]listener[MessageReactionsHandler](nil), d.messageReactions...)
	case *updates.Update_ConversationLastRead:
		lastRead = append([]listener[ConversationLastReadHandler](nil), d.lastRead...)
	case *updates.Update_Interaction:
		interactions = append([]listener[InteractionHandler](nil), d.interaction...)
	case *updates.Update_RoomState:
		voiceRoomState = append([]listener[VoiceRoomStateHandler](nil), d.voiceRoomState...)
	case *updates.Update_RoomParticipant:
		voiceParticipant = append([]listener[VoiceRoomParticipantHandler](nil), d.voiceParticipant...)
	}
	d.mu.RUnlock()

	for _, item := range generic {
		event := &UpdateEvent{Base: d.base, Raw: update}
		d.call("update", ctx, func(ctx context.Context) error { return item.fn(ctx, event) })
	}
	switch value := update.GetUpdate().(type) {
	case *updates.Update_MessageCreated:
		author := types.UserFromProto(value.MessageCreated.GetAuthor())
		message := types.MessageFromProto(value.MessageCreated.GetMessage())
		if message != nil {
			message.Author = author
		}
		event := eventtypes.NewMessageCreateEvent(d.base, message, author)
		for _, item := range messageCreate {
			d.call("message_create", ctx, func(ctx context.Context) error { return item.fn(ctx, event) })
		}
	case *updates.Update_Message:
		event := &MessageUpdateEvent{Base: d.base, Message: types.MessageFromProto(value.Message.GetMessage())}
		for _, item := range messageUpdate {
			d.call("message_update", ctx, func(ctx context.Context) error { return item.fn(ctx, event) })
		}
	case *updates.Update_MessageDeleted:
		ids := make([]types.ID, len(value.MessageDeleted.GetMessageIds()))
		for i, id := range value.MessageDeleted.GetMessageIds() {
			ids[i] = types.ID(id)
		}
		event := &MessageDeleteEvent{Base: d.base, Chat: types.ChatRefFromProto(value.MessageDeleted.GetChatRef()), MessageIDs: ids}
		for _, item := range messageDelete {
			d.call("message_delete", ctx, func(ctx context.Context) error { return item.fn(ctx, event) })
		}
	case *updates.Update_CommunityMemberCreated:
		event := &MemberCreateEvent{Base: d.base, CommunityID: types.ID(value.CommunityMemberCreated.GetCommunityId()), MemberID: types.ID(value.CommunityMemberCreated.GetMemberId()), Member: types.CommunityMemberFromProto(value.CommunityMemberCreated.GetMember()), User: types.UserFromProto(value.CommunityMemberCreated.GetUser())}
		for _, item := range memberCreate {
			d.call("member_create", ctx, func(ctx context.Context) error { return item.fn(ctx, event) })
		}
	case *updates.Update_Channel:
		event := &ChannelUpdateEvent{Base: d.base, Channel: types.ChannelFromProto(value.Channel.GetChannel())}
		for _, item := range channelUpdate {
			d.call("channel_update", ctx, func(ctx context.Context) error { return item.fn(ctx, event) })
		}
	case *updates.Update_ChannelDeleted:
		event := &ChannelDeleteEvent{Base: d.base, Channel: types.ChannelRefFromProto(value.ChannelDeleted.GetChannel())}
		for _, item := range channelDelete {
			d.call("channel_delete", ctx, func(ctx context.Context) error { return item.fn(ctx, event) })
		}
	case *updates.Update_User:
		event := &UserUpdateEvent{Base: d.base, UserID: types.ID(value.User.GetUserId()), User: types.UserFromProto(value.User.GetUser())}
		for _, item := range userUpdate {
			d.call("user_update", ctx, func(ctx context.Context) error { return item.fn(ctx, event) })
		}
	case *updates.Update_Community:
		event := &CommunityUpdateEvent{Base: d.base, CommunityID: types.ID(value.Community.GetCommunityId()), Community: types.CommunityFromProto(value.Community.GetCommunity())}
		for _, item := range communityUpdate {
			d.call("community_update", ctx, func(ctx context.Context) error { return item.fn(ctx, event) })
		}
	case *updates.Update_CommunityDeleted:
		event := &CommunityDeleteEvent{Base: d.base, CommunityID: types.ID(value.CommunityDeleted.GetCommunityId())}
		for _, item := range communityDelete {
			d.call("community_delete", ctx, func(ctx context.Context) error { return item.fn(ctx, event) })
		}
	case *updates.Update_ChatTyping:
		var action *types.TypingAction
		if value.ChatTyping != nil && value.ChatTyping.Action != nil {
			action = value.ChatTyping.Action
		}
		event := &ChatTypingEvent{Base: d.base, Chat: types.ChatRefFromProto(value.ChatTyping.GetChatRef()), UserID: types.ID(value.ChatTyping.GetUserId()), Typing: value.ChatTyping.GetTyping(), Action: action}
		for _, item := range chatTyping {
			d.call("chat_typing", ctx, func(ctx context.Context) error { return item.fn(ctx, event) })
		}
	case *updates.Update_CommunityMember:
		event := &MemberUpdateEvent{Base: d.base, CommunityID: types.ID(value.CommunityMember.GetCommunityId()), MemberID: types.ID(value.CommunityMember.GetMemberId()), Member: types.CommunityMemberFromProto(value.CommunityMember.GetMember())}
		for _, item := range memberUpdate {
			d.call("member_update", ctx, func(ctx context.Context) error { return item.fn(ctx, event) })
		}
	case *updates.Update_CommunityMemberDeleted:
		event := &MemberDeleteEvent{Base: d.base, CommunityID: types.ID(value.CommunityMemberDeleted.GetCommunityId()), MemberID: types.ID(value.CommunityMemberDeleted.GetMemberId())}
		for _, item := range memberDelete {
			d.call("member_delete", ctx, func(ctx context.Context) error { return item.fn(ctx, event) })
		}
	case *updates.Update_MessageReactions:
		reactions := value.MessageReactions.GetReactions()
		var messageID types.ID
		if reactions != nil {
			messageID = types.ID(reactions.GetMessageId())
		}
		event := &MessageReactionsEvent{Base: d.base, Chat: types.ChatRefFromProto(value.MessageReactions.GetChatRef()), MessageID: messageID, Reactions: reactions}
		for _, item := range messageReactions {
			d.call("message_reactions", ctx, func(ctx context.Context) error { return item.fn(ctx, event) })
		}
	case *updates.Update_ConversationLastRead:
		var unreadCount, unreadMentionsCount *uint32
		if value.ConversationLastRead != nil {
			unreadCount = value.ConversationLastRead.UnreadCount
			unreadMentionsCount = value.ConversationLastRead.UnreadMentionsCount
		}
		event := &ConversationLastReadEvent{Base: d.base, Chat: types.ChatRefFromProto(value.ConversationLastRead.GetChatRef()), LastReadMessageID: types.ID(value.ConversationLastRead.GetLastReadMessageId()), UnreadCount: unreadCount, UnreadMentionsCount: unreadMentionsCount}
		for _, item := range lastRead {
			d.call("conversation_last_read", ctx, func(ctx context.Context) error { return item.fn(ctx, event) })
		}
	case *updates.Update_Interaction:
		interaction := value.Interaction
		model := types.InteractionFromProto(interaction)
		event := eventtypes.NewInteractionEvent(d.base, model)
		for _, item := range interactions {
			d.call("interaction", ctx, func(ctx context.Context) error { return item.fn(ctx, event) })
		}
	case *updates.Update_RoomState:
		update := value.RoomState
		event := &VoiceRoomStateEvent{Base: d.base, Chat: types.ChatRefFromProto(update.GetChatRef()), State: voice.RoomStateFromProto(update.GetState())}
		for _, item := range voiceRoomState {
			d.call("voice_room_state", ctx, func(ctx context.Context) error { return item.fn(ctx, event) })
		}
	case *updates.Update_RoomParticipant:
		update := value.RoomParticipant
		event := &VoiceRoomParticipantEvent{Base: d.base, Chat: types.ChatRefFromProto(update.GetChatRef()), Participant: voice.ParticipantFromProto(update.GetParticipant())}
		for _, item := range voiceParticipant {
			d.call("voice_room_participant", ctx, func(ctx context.Context) error { return item.fn(ctx, event) })
		}
	}
}

func (d *eventDispatcher) call(name string, ctx context.Context, fn func(context.Context) error) {
	var failure *HandlerError
	defer func() {
		if recovered := recover(); recovered != nil {
			failure = &HandlerError{Event: name, Panic: recovered, Stack: debug.Stack()}
			d.logger.Error("event handler panic", "event", name, "panic", recovered, "stack", string(debug.Stack()))
		}
		if failure != nil {
			d.reportHandlerError(ctx, failure)
		}
	}()
	if err := fn(ctx); err != nil {
		failure = &HandlerError{Event: name, Err: err}
		d.logger.Error("event handler failed", "event", name, "error", err)
	}
}

func (d *eventDispatcher) reportHandlerError(ctx context.Context, failure *HandlerError) {
	if d.onHandlerError == nil {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			d.logger.Error("handler error callback panic", "panic", recovered, "stack", string(debug.Stack()))
		}
	}()
	d.onHandlerError(ctx, failure)
}

func (d *eventDispatcher) reportOverflow(dropped uint64) {
	if d.onEventOverflow == nil {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			d.logger.Error("event overflow callback panic", "panic", recovered, "stack", string(debug.Stack()))
		}
	}()
	d.onEventOverflow(dropped)
}

func (c *Client) OnReady(handler ReadyHandler) func() { return c.events.onReady(handler) }
func (c *Client) OnMessageCreate(handler MessageCreateHandler) func() {
	remove := c.events.onMessageCreate(handler)
	return func() { remove() }
}
func (c *Client) OnMessageUpdate(handler MessageUpdateHandler) func() {
	return c.events.onMessageUpdate(handler)
}
func (c *Client) OnMessageDelete(handler MessageDeleteHandler) func() {
	return c.events.onMessageDelete(handler)
}
func (c *Client) OnMemberCreate(handler MemberCreateHandler) func() {
	return c.events.onMemberCreate(handler)
}
func (c *Client) OnChannelUpdate(handler ChannelUpdateHandler) func() {
	return c.events.onChannelUpdate(handler)
}
func (c *Client) OnChannelDelete(handler ChannelDeleteHandler) func() {
	return c.events.onChannelDelete(handler)
}
func (c *Client) OnUserUpdate(handler UserUpdateHandler) func() {
	return c.events.onUserUpdate(handler)
}
func (c *Client) OnCommunityUpdate(handler CommunityUpdateHandler) func() {
	return c.events.onCommunityUpdate(handler)
}
func (c *Client) OnCommunityDelete(handler CommunityDeleteHandler) func() {
	return c.events.onCommunityDelete(handler)
}
func (c *Client) OnChatTyping(handler ChatTypingHandler) func() {
	return c.events.onChatTyping(handler)
}
func (c *Client) OnMemberUpdate(handler MemberUpdateHandler) func() {
	return c.events.onMemberUpdate(handler)
}
func (c *Client) OnMemberDelete(handler MemberDeleteHandler) func() {
	return c.events.onMemberDelete(handler)
}
func (c *Client) OnMessageReactions(handler MessageReactionsHandler) func() {
	return c.events.onMessageReactions(handler)
}
func (c *Client) OnConversationLastRead(handler ConversationLastReadHandler) func() {
	return c.events.onConversationLastRead(handler)
}
func (c *Client) OnInteraction(handler InteractionHandler) func() {
	return c.events.onInteraction(handler)
}
func (c *Client) OnVoiceRoomState(handler VoiceRoomStateHandler) func() {
	return c.events.onVoiceRoomState(handler)
}
func (c *Client) OnVoiceRoomParticipant(handler VoiceRoomParticipantHandler) func() {
	return c.events.onVoiceRoomParticipant(handler)
}
func (c *Client) OnConnecting(handler ConnectionHandler) func() {
	return c.events.onConnecting(handler)
}
func (c *Client) OnConnected(handler ConnectionHandler) func() {
	return c.events.onConnected(handler)
}
func (c *Client) OnDisconnected(handler ConnectionHandler) func() {
	return c.events.onDisconnected(handler)
}
func (c *Client) OnReconnecting(handler ConnectionHandler) func() {
	return c.events.onReconnecting(handler)
}
func (c *Client) OnConnectionError(handler ConnectionHandler) func() {
	return c.events.onConnectionError(handler)
}

// OnError is a short alias for connection errors. Handler failures remain
// observable through Config.OnHandlerError.
func (c *Client) OnError(handler ConnectionHandler) func() {
	return c.OnConnectionError(handler)
}
func (c *Client) OnUpdate(handler UpdateHandler) func() { return c.events.onUpdate(handler) }

func (c *Client) emitReady(ctx context.Context, event *ReadyEvent) {
	_ = c.events.enqueueReady(ctx, event)
}
func (c *Client) emitConnecting(ctx context.Context, event *ConnectionEvent) {
	_ = c.events.enqueueConnection(ctx, "connecting", event)
}
func (c *Client) emitConnected(ctx context.Context, event *ConnectionEvent) {
	_ = c.events.enqueueConnection(ctx, "connected", event)
}
func (c *Client) emitDisconnected(ctx context.Context, event *ConnectionEvent) {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil || c.isClosed() {
		// The final disconnect must remain observable even after Run's context
		// is canceled; the running lifecycle uses the bounded queue below.
		c.events.emitDisconnected(context.WithoutCancel(ctx), event)
		return
	}
	_ = c.events.enqueueConnection(ctx, "disconnected", event)
}
func (c *Client) emitReconnecting(ctx context.Context, event *ConnectionEvent) {
	_ = c.events.enqueueConnection(ctx, "reconnecting", event)
}
func (c *Client) emitConnectionError(ctx context.Context, event *ConnectionEvent) {
	_ = c.events.enqueueConnection(ctx, "connection_error", event)
}
