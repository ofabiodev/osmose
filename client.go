// Package osmose is the idiomatic Go SDK for Osmium bots.
//
// Osmose hides the WebSocket, binary protobuf, handshake, reconnect, and RPC
// correlation details behind a small typed client API. Advanced users can use
// Client.Raw to send generated protobuf requests directly.
package osmose

import (
	"github.com/ofabiodev/osmose/collectors"
	"github.com/ofabiodev/osmose/events"
	coreclient "github.com/ofabiodev/osmose/internal/client"
	"github.com/ofabiodev/osmose/types"
)

type Client = coreclient.Client
type Config = coreclient.Config
type CacheConfig = types.CacheConfig
type Managers = types.Managers
type UserManager = types.UserManager
type CommunityManager = types.CommunityManager
type ChannelManager = types.ChannelManager
type MemberManager = types.MemberManager
type RoleManager = types.RoleManager
type MessageManager = types.MessageManager
type RawClient = coreclient.RawClient

func New(config Config) (*Client, error) { return coreclient.New(config) }

type State = coreclient.State

const (
	Disconnected   = coreclient.Disconnected
	Connecting     = coreclient.Connecting
	Initializing   = coreclient.Initializing
	Authenticating = coreclient.Authenticating
	Ready          = coreclient.Ready
	Closing        = coreclient.Closing
)

type PermanentError = coreclient.PermanentError
type RPCError = coreclient.RPCError
type UnexpectedResultError = coreclient.UnexpectedResultError

func IsPermanent(err error) bool { return coreclient.IsPermanent(err) }

type ReadyHandler = events.ReadyHandler
type MessageCreateHandler = events.MessageCreateHandler
type MessageUpdateHandler = events.MessageUpdateHandler
type MessageDeleteHandler = events.MessageDeleteHandler
type MemberCreateHandler = events.MemberCreateHandler
type ChannelUpdateHandler = events.ChannelUpdateHandler
type ChannelDeleteHandler = events.ChannelDeleteHandler
type UserUpdateHandler = events.UserUpdateHandler
type CommunityUpdateHandler = events.CommunityUpdateHandler
type CommunityDeleteHandler = events.CommunityDeleteHandler
type ChatTypingHandler = events.ChatTypingHandler
type MemberUpdateHandler = events.MemberUpdateHandler
type MemberDeleteHandler = events.MemberDeleteHandler
type MessageReactionsHandler = events.MessageReactionsHandler
type ConversationLastReadHandler = events.ConversationLastReadHandler
type InteractionHandler = events.InteractionHandler
type VoiceRoomStateHandler = events.VoiceRoomStateHandler
type VoiceRoomParticipantHandler = events.VoiceRoomParticipantHandler
type ConnectionHandler = events.ConnectionHandler
type UpdateHandler = events.UpdateHandler
type HandlerError = events.HandlerError
type HandlerErrorHandler = events.HandlerErrorHandler
type EventOverflowHandler = events.EventOverflowHandler
type ConnectionEvent = events.ConnectionEvent
type ReadyEvent = events.ReadyEvent
type MessageCreateEvent = events.MessageCreateEvent
type MessageUpdateEvent = events.MessageUpdateEvent
type MessageDeleteEvent = events.MessageDeleteEvent
type MemberCreateEvent = events.MemberCreateEvent
type ChannelUpdateEvent = events.ChannelUpdateEvent
type ChannelDeleteEvent = events.ChannelDeleteEvent
type UserUpdateEvent = events.UserUpdateEvent
type CommunityUpdateEvent = events.CommunityUpdateEvent
type CommunityDeleteEvent = events.CommunityDeleteEvent
type ChatTypingEvent = events.ChatTypingEvent
type MemberUpdateEvent = events.MemberUpdateEvent
type MemberDeleteEvent = events.MemberDeleteEvent
type MessageReactionsEvent = events.MessageReactionsEvent
type ConversationLastReadEvent = events.ConversationLastReadEvent
type InteractionEvent = events.InteractionEvent
type VoiceRoomStateEvent = events.VoiceRoomStateEvent
type VoiceRoomParticipantEvent = events.VoiceRoomParticipantEvent
type UpdateEvent = events.UpdateEvent

type EndReason = collectors.EndReason

const (
	EndReasonTime     = collectors.EndReasonTime
	EndReasonIdle     = collectors.EndReasonIdle
	EndReasonLimit    = collectors.EndReasonLimit
	EndReasonStopped  = collectors.EndReasonStopped
	EndReasonContext  = collectors.EndReasonContext
	EndReasonOverflow = collectors.EndReasonOverflow
	EndReasonClosed   = collectors.EndReasonClosed
)

type CollectorError = collectors.CollectorError
type CollectorResult = collectors.CollectorResult
type MessageCollectorOptions = collectors.MessageCollectorOptions
type MessageCollector = collectors.MessageCollector
type InteractionCollectorOptions = collectors.InteractionCollectorOptions
type InteractionCollector = collectors.InteractionCollector
type ReactionCollectorOptions = collectors.ReactionCollectorOptions
type ReactionCollector = collectors.ReactionCollector

var (
	ErrClosed              = coreclient.ErrClosed
	ErrNotConnected        = coreclient.ErrNotConnected
	ErrNotReady            = coreclient.ErrNotReady
	ErrAlreadyRunning      = coreclient.ErrAlreadyRunning
	ErrRunCompleted        = coreclient.ErrRunCompleted
	ErrPermanent           = coreclient.ErrPermanent
	ErrAuthorizationFailed = coreclient.ErrAuthorizationFailed
	ErrProtocolMismatch    = coreclient.ErrProtocolMismatch
	ErrDisconnected        = coreclient.ErrDisconnected
	ErrUnsupportedRequest  = coreclient.ErrUnsupportedRequest
	ErrEventQueueFull      = events.ErrEventQueueFull
	ErrCollectorEnded      = collectors.ErrCollectorEnded
	ErrCollectorTimeout    = collectors.ErrCollectorTimeout
	ErrCollectorIdle       = collectors.ErrCollectorIdle
	ErrCollectorOverflow   = collectors.ErrCollectorOverflow
	ErrCollectorClosed     = collectors.ErrCollectorClosed
)
