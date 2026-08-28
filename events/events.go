// Package events contains the typed event payloads emitted by Osmose.
package events

import (
	"context"
	"errors"
	"time"

	"github.com/ofabiodev/osmose/proto/updates"
	"github.com/ofabiodev/osmose/types"
	"github.com/ofabiodev/osmose/voice"
)

// Client is the client that emitted an event. It exposes lifecycle information
// without making the events package depend on the root client package.
type Client interface {
	State() State
	Done() <-chan struct{}
}

// Base contains helpers shared by all event payloads.
type Base struct {
	client       Client
	replyMessage func(context.Context, *types.Message, string) error
	respond      func(context.Context, types.ID, string) error
	acknowledge  func(context.Context, types.ID) error
}

// NewBase associates an event with its client.
func NewBase(client Client, replyMessage func(context.Context, *types.Message, string) error, respond func(context.Context, types.ID, string) error, acknowledge func(context.Context, types.ID) error) Base {
	return Base{client: client, replyMessage: replyMessage, respond: respond, acknowledge: acknowledge}
}

// Client returns the client that emitted the event.
func (b Base) Client() Client { return b.client }

var ErrClosed = errors.New("osmose client closed")

type ReadyHandler func(context.Context, *ReadyEvent) error
type MessageCreateHandler func(context.Context, *MessageCreateEvent) error
type MessageUpdateHandler func(context.Context, *MessageUpdateEvent) error
type MessageDeleteHandler func(context.Context, *MessageDeleteEvent) error
type MemberCreateHandler func(context.Context, *MemberCreateEvent) error
type ChannelUpdateHandler func(context.Context, *ChannelUpdateEvent) error
type ChannelDeleteHandler func(context.Context, *ChannelDeleteEvent) error
type UserUpdateHandler func(context.Context, *UserUpdateEvent) error
type CommunityUpdateHandler func(context.Context, *CommunityUpdateEvent) error
type CommunityDeleteHandler func(context.Context, *CommunityDeleteEvent) error
type ChatTypingHandler func(context.Context, *ChatTypingEvent) error
type MemberUpdateHandler func(context.Context, *MemberUpdateEvent) error
type MemberDeleteHandler func(context.Context, *MemberDeleteEvent) error
type MessageReactionsHandler func(context.Context, *MessageReactionsEvent) error
type ConversationLastReadHandler func(context.Context, *ConversationLastReadEvent) error
type InteractionHandler func(context.Context, *InteractionEvent) error
type VoiceRoomStateHandler func(context.Context, *VoiceRoomStateEvent) error
type VoiceRoomParticipantHandler func(context.Context, *VoiceRoomParticipantEvent) error
type ConnectionHandler func(context.Context, *ConnectionEvent) error
type UpdateHandler func(context.Context, *UpdateEvent) error

// HandlerError describes an error or panic returned by an event handler.
type HandlerError struct {
	Event string
	Err   error
	Panic any
	Stack []byte
}

// HandlerErrorHandler observes errors and panics from typed event handlers.
type HandlerErrorHandler func(context.Context, *HandlerError)

// EventOverflowHandler observes cumulative event drops caused by a full
// bounded event queue.
type EventOverflowHandler func(dropped uint64)

var ErrEventQueueFull = errors.New("osmose event queue is full")

// ConnectionEvent describes a transport lifecycle transition.
type ConnectionEvent struct {
	Base
	Attempt int
	RetryIn time.Duration
	Err     error
	State   State
}

type ReadyEvent struct {
	Base
	User      *types.User
	SessionID types.ID
}

type MessageCreateEvent struct {
	Base
	Message *types.Message
	Author  *types.User
}

// NewMessageCreateEvent creates a message event with a reply callback.
func NewMessageCreateEvent(base Base, message *types.Message, author *types.User) *MessageCreateEvent {
	return &MessageCreateEvent{Base: base, Message: message, Author: author}
}

func (e *MessageCreateEvent) Reply(ctx context.Context, content string) error {
	if e == nil || e.Base.replyMessage == nil {
		return ErrClosed
	}
	return e.Base.replyMessage(ctx, e.Message, content)
}

type MessageUpdateEvent struct {
	Base
	Message *types.Message
}

type MessageDeleteEvent struct {
	Base
	Chat       types.ChatRef
	MessageIDs []types.ID
}

type MemberCreateEvent struct {
	Base
	CommunityID types.ID
	MemberID    types.ID
	Member      *types.CommunityMember
	User        *types.User
}

type ChannelUpdateEvent struct {
	Base
	Channel *types.Channel
}

type ChannelDeleteEvent struct {
	Base
	Channel types.ChannelRef
}

type UserUpdateEvent struct {
	Base
	UserID types.ID
	User   *types.User
}

type CommunityUpdateEvent struct {
	Base
	CommunityID types.ID
	Community   *types.Community
}

type CommunityDeleteEvent struct {
	Base
	CommunityID types.ID
}

type ChatTypingEvent struct {
	Base
	Chat   types.ChatRef
	UserID types.ID
	Typing bool
	Action *types.TypingAction
}

type MemberUpdateEvent struct {
	Base
	CommunityID types.ID
	MemberID    types.ID
	Member      *types.CommunityMember
}

type MemberDeleteEvent struct {
	Base
	CommunityID types.ID
	MemberID    types.ID
}

type MessageReactionsEvent struct {
	Base
	Chat      types.ChatRef
	MessageID types.ID
	Reactions *types.MessageReactions
}

type ConversationLastReadEvent struct {
	Base
	Chat                types.ChatRef
	LastReadMessageID   types.ID
	UnreadCount         *uint32
	UnreadMentionsCount *uint32
}

type InteractionEvent struct {
	Base
	ID          types.ID
	UserID      types.ID
	MessageID   types.ID
	Data        string
	Interaction *types.Interaction
}

// NewInteractionEvent creates an interaction event with response callbacks.
func NewInteractionEvent(base Base, interaction *types.Interaction) *InteractionEvent {
	event := &InteractionEvent{Base: base, Interaction: interaction}
	if interaction != nil {
		event.ID = interaction.ID
		event.UserID = interaction.UserID
		event.MessageID = interaction.MessageID
		event.Data = interaction.Data
	}
	return event
}

func (e *InteractionEvent) Respond(ctx context.Context, content string) error {
	if e == nil || e.Base.respond == nil {
		return ErrClosed
	}
	return e.Base.respond(ctx, e.ID, content)
}

func (e *InteractionEvent) Reply(ctx context.Context, content string) error {
	return e.Respond(ctx, content)
}

// Acknowledge accepts an interaction without sending a message.
func (e *InteractionEvent) Acknowledge(ctx context.Context) error {
	if e == nil || e.Base.acknowledge == nil {
		return ErrClosed
	}
	return e.Base.acknowledge(ctx, e.ID)
}

// Defer is an alias for Acknowledge.
func (e *InteractionEvent) Defer(ctx context.Context) error {
	return e.Acknowledge(ctx)
}

// VoiceRoomStateEvent reports a complete voice room state update.
type VoiceRoomStateEvent struct {
	Base
	Chat  types.ChatRef
	State *voice.RoomState
}

// VoiceRoomParticipantEvent reports a participant change in a voice room.
type VoiceRoomParticipantEvent struct {
	Base
	Chat        types.ChatRef
	Participant *voice.Participant
}

type UpdateEvent struct {
	Base
	Raw *updates.Update
}
