package types

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ofabiodev/osmose/internal/rpc"
	"github.com/ofabiodev/osmose/internal/state"
	protoAuth "github.com/ofabiodev/osmose/proto/auth"
	protoChats "github.com/ofabiodev/osmose/proto/chats"
	protoCommunities "github.com/ofabiodev/osmose/proto/communities"
	"github.com/ofabiodev/osmose/proto/core"
	protoMedia "github.com/ofabiodev/osmose/proto/media"
	protoMessages "github.com/ofabiodev/osmose/proto/messages"
	protoReactions "github.com/ofabiodev/osmose/proto/reactions"
	protoRefs "github.com/ofabiodev/osmose/proto/refs"
	protoTypes "github.com/ofabiodev/osmose/proto/types"
	"google.golang.org/protobuf/proto"
)

// ObjectClient is the private call boundary attached to models returned by
// services. Rich object methods use it to keep protocol details out of bot
// code while preserving Raw for callers that need the escape hatch.
type ObjectClient struct {
	call     func(context.Context, proto.Message) (*core.RPCResult, error)
	cache    *state.Cache
	flightMu sync.Mutex
	flights  map[string]*objectFlight
	managers *Managers
}

// NewObjectClient binds rich models to an Osmium call function. It is mainly
// used by Osmose services and is not needed when using the root Client.
func NewObjectClient(call func(context.Context, proto.Message) (*core.RPCResult, error), configs ...CacheConfig) *ObjectClient {
	var config CacheConfig
	if len(configs) != 0 {
		config = configs[0]
	}
	c := &ObjectClient{call: call, cache: state.New(config), flights: make(map[string]*objectFlight)}
	c.managers = newManagers(c)
	return c
}

var ErrObjectClientUnavailable = errors.New("rich object client unavailable")

func firstObjectClient(clients ...*ObjectClient) *ObjectClient {
	if len(clients) == 0 {
		return nil
	}
	return clients[0]
}

func callObject(client *ObjectClient, ctx context.Context, request proto.Message) (*core.RPCResult, error) {
	if client == nil || client.call == nil {
		return nil, ErrObjectClientUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return client.Call(ctx, request)
}

func requireObjectID(id ID, name string) error {
	if id == 0 {
		return fmt.Errorf("%s ID is required", name)
	}
	return nil
}

func requireObjectClient(client *ObjectClient) error {
	if client == nil || client.call == nil {
		return ErrObjectClientUnavailable
	}
	return nil
}

// MessageSendParams controls a message sent by a rich object.
type MessageSendParams struct {
	Content        string
	ReplyTo        ID
	ReplyQuote     *MessageQuote
	Media          []*MediaRef
	Entities       []*MessageEntity
	SuppressEmbeds bool
	BotInfo        *MessageBotInfo
}

// MessageEditParams controls an existing message edit.
type MessageEditParams struct {
	Chat           ChatRef
	MessageID      ID
	Content        *string
	RemoveMedia    bool
	Media          []*MediaRef
	Entities       []*MessageEntity
	SuppressEmbeds bool
	Buttons        *MessageButtons
}

// MessageDeleteParams controls deletion of one or more messages.
type MessageDeleteParams struct {
	Chat       ChatRef
	MessageIDs []ID
}

// MessageHistoryParams controls channel history pagination.
type MessageHistoryParams struct {
	Chat   ChatRef
	Limit  uint32
	Before ID
}

// MessageSearchParams controls chat-scoped message search.
type MessageSearchParams struct {
	Chat   ChatRef
	Query  string
	Scoped bool
	Since  ID
	Before ID
}

// MessageHistory contains messages and the users referenced by them.
type MessageHistory struct {
	Messages []*Message
	Users    []*User
	Raw      *protoMessages.Messages
}

// CommunityEditOptions controls the optional community fields supported by
// the Osmium protocol.
type CommunityEditOptions struct {
	Name     *string
	Username *string
}

// ChannelCreateOptions controls a new community channel.
type ChannelCreateOptions struct {
	Name            string
	Type            ChannelType
	ParentID        *ID
	PreferredRegion *string
}

// ChannelEditOptions controls the optional channel fields supported by the
// Osmium protocol. A non-nil ID of zero clears a parent or highlighted message.
type ChannelEditOptions struct {
	Name                 *string
	Position             *uint32
	ParentID             *ID
	Explicit             *bool
	HighlightedMessageID *ID
	Description          *string
	SlowmodeSeconds      *uint32
	PreferredRegion      *string
}

// RoleCreateOptions controls a new community role.
type RoleCreateOptions struct {
	Name        string
	Permissions uint64
	Priority    uint32
	Color       uint32
	Separated   bool
	Public      bool
}

// RoleEditOptions controls a role update. Omitted values keep their current
// value because the protocol edit method requires a complete role payload.
type RoleEditOptions struct {
	Name        *string
	Permissions *uint64
	Priority    *uint32
	Color       *uint32
	Separated   *bool
	Public      *bool
}

// MemberEditOptions controls a community member update. A nil RoleIDs leaves
// roles unchanged; a non-nil empty slice removes every role.
type MemberEditOptions struct {
	Nickname *string
	RoleIDs  []ID
}

// BanOptions controls a member removal. A nil Until means a permanent ban.
type BanOptions struct {
	Until               *uint64
	DeleteMessagesSince *uint64
	Reason              string
}

// InviteOptions controls a channel invite.
type InviteOptions struct {
	ExpiresAt uint64
	MaxUses   uint32
}

// Invite is the created invite returned by the Osmium API.
type Invite struct {
	Code string
	Raw  *protoAuth.CreatedInvite
}

// InvitePreview is an invite returned by a channel invite listing.
type InvitePreview struct {
	Code       string
	CreatorID  ID
	TargetID   ID
	TargetType protoAuth.InviteType
	ExpiresAt  *uint64
	Raw        *protoAuth.InvitePreview
}

// CommunityRole is a role that belongs to a community.
type CommunityRole struct {
	Partial     bool
	ID          ID
	CommunityID ID
	Name        string
	Permissions uint64
	Priority    uint32
	Color       uint32
	Separated   bool
	Public      bool
	Raw         *protoTypes.CommunityRole
	client      *ObjectClient
}

func CommunityRoleFromProto(value *protoTypes.CommunityRole, clients ...*ObjectClient) *CommunityRole {
	if value == nil {
		return nil
	}
	return &CommunityRole{
		ID:          ID(value.GetId()),
		CommunityID: ID(value.GetCommunityId()),
		Name:        value.GetName(),
		Permissions: value.GetPermissions(),
		Priority:    value.GetPriority(),
		Color:       value.GetColor(),
		Separated:   value.GetSeparated(),
		Public:      value.GetPublic(),
		Raw:         value,
		client:      firstObjectClient(clients...),
	}
}

// Member and Role are the concise names used by the rich object API.
type Member = CommunityMember
type Role = CommunityRole

// Channels returns the channels visible in the community.
func (c *Community) Channels(ctx context.Context) ([]*Channel, error) {
	if c == nil {
		return nil, ErrObjectClientUnavailable
	}
	if err := requireObjectID(c.ID, "community"); err != nil {
		return nil, err
	}
	result, err := callObject(c.client, ctx, &protoCommunities.GetChannels{CommunityId: uint64(c.ID)})
	if err != nil {
		return nil, err
	}
	value := result.GetChannels()
	if value == nil {
		return nil, &rpc.UnexpectedResultError{Method: "communities.getChannels"}
	}
	channels := make([]*Channel, 0, len(value.GetChannels()))
	for _, channel := range value.GetChannels() {
		channels = append(channels, ChannelFromProto(channel, c.client))
	}
	return channels, nil
}

// Members returns all community members, or only the requested member IDs.
func (c *Community) Members(ctx context.Context, memberIDs ...ID) ([]*Member, error) {
	if c == nil {
		return nil, ErrObjectClientUnavailable
	}
	if err := requireObjectID(c.ID, "community"); err != nil {
		return nil, err
	}
	ids := make([]uint64, len(memberIDs))
	for i, id := range memberIDs {
		if err := requireObjectID(id, "member"); err != nil {
			return nil, err
		}
		ids[i] = uint64(id)
	}
	result, err := callObject(c.client, ctx, &protoCommunities.GetMembers{CommunityId: uint64(c.ID), MemberIds: ids})
	if err != nil {
		return nil, err
	}
	value := result.GetMembers()
	if value == nil {
		return nil, &rpc.UnexpectedResultError{Method: "communities.getMembers"}
	}
	users := make(map[ID]*User, len(value.GetUsers()))
	for _, user := range value.GetUsers() {
		model := UserFromProto(user, c.client)
		if model != nil {
			users[model.ID] = model
		}
	}
	members := make([]*Member, 0, len(value.GetMembers()))
	for _, member := range value.GetMembers() {
		model := CommunityMemberFromProto(member, c.client)
		if model == nil {
			continue
		}
		if model != nil {
			if model.CommunityID == 0 {
				model.CommunityID = c.ID
			}
			model.User = users[model.ID]
			if model.User == nil {
				model.User, _ = c.client.Managers().Users.Get(model.ID)
			}
		}
		members = append(members, model)
	}
	return members, nil
}

// Roles returns the roles visible in the community.
func (c *Community) Roles(ctx context.Context) ([]*Role, error) {
	if c == nil {
		return nil, ErrObjectClientUnavailable
	}
	if err := requireObjectID(c.ID, "community"); err != nil {
		return nil, err
	}
	result, err := callObject(c.client, ctx, &protoCommunities.GetRoles{CommunityId: uint64(c.ID)})
	if err != nil {
		return nil, err
	}
	value := result.GetCommunityRoles()
	if value == nil {
		return nil, &rpc.UnexpectedResultError{Method: "communities.getRoles"}
	}
	roles := make([]*Role, 0, len(value.GetRoles()))
	for _, role := range value.GetRoles() {
		model := CommunityRoleFromProto(role, c.client)
		if model != nil {
			if model.CommunityID == 0 {
				model.CommunityID = c.ID
			}
			roles = append(roles, model)
		}
	}
	return roles, nil
}

// Edit updates the community and applies successful changes to the object.
func (c *Community) Edit(ctx context.Context, options CommunityEditOptions) error {
	if c == nil {
		return ErrObjectClientUnavailable
	}
	if err := requireObjectID(c.ID, "community"); err != nil {
		return err
	}
	result, err := callObject(c.client, ctx, &protoCommunities.EditCommunity{
		CommunityId: uint64(c.ID),
		Name:        options.Name,
		Username:    options.Username,
	})
	if err != nil {
		return err
	}
	if err := rpc.EnsureVoid(result, "communities.editCommunity"); err != nil {
		return err
	}
	if options.Name != nil {
		c.Name = *options.Name
	}
	if options.Username != nil {
		c.Username = cloneString(options.Username)
	}
	return nil
}

// Delete permanently deletes the community.
func (c *Community) Delete(ctx context.Context) error {
	if c == nil {
		return ErrObjectClientUnavailable
	}
	if err := requireObjectID(c.ID, "community"); err != nil {
		return err
	}
	result, err := callObject(c.client, ctx, &protoCommunities.DeleteCommunity{CommunityId: uint64(c.ID)})
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "communities.deleteCommunity")
}

// Leave removes the current user from the community.
func (c *Community) Leave(ctx context.Context) error {
	if c == nil {
		return ErrObjectClientUnavailable
	}
	if err := requireObjectID(c.ID, "community"); err != nil {
		return err
	}
	result, err := callObject(c.client, ctx, &protoCommunities.LeaveCommunity{CommunityId: uint64(c.ID)})
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "communities.leaveCommunity")
}

// CreateChannel creates a channel in the community.
func (c *Community) CreateChannel(ctx context.Context, options ChannelCreateOptions) error {
	if c == nil {
		return ErrObjectClientUnavailable
	}
	if err := requireObjectID(c.ID, "community"); err != nil {
		return err
	}
	if options.Name == "" {
		return fmt.Errorf("channel name is required")
	}
	request := &protoCommunities.CreateChannel{
		CommunityId:     uint64(c.ID),
		Name:            options.Name,
		Type:            options.Type,
		PreferredRegion: options.PreferredRegion,
	}
	if options.ParentID != nil {
		request.ParentId = uint64Pointer(*options.ParentID)
	}
	result, err := callObject(c.client, ctx, request)
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "communities.createChannel")
}

// CreateRole creates a role in the community.
func (c *Community) CreateRole(ctx context.Context, options RoleCreateOptions) error {
	if c == nil {
		return ErrObjectClientUnavailable
	}
	if err := requireObjectID(c.ID, "community"); err != nil {
		return err
	}
	if options.Name == "" {
		return fmt.Errorf("role name is required")
	}
	result, err := callObject(c.client, ctx, &protoCommunities.CreateRole{
		CommunityId: uint64(c.ID),
		Name:        options.Name,
		Permissions: options.Permissions,
		Priority:    options.Priority,
		Color:       options.Color,
		Separated:   options.Separated,
		Public:      options.Public,
	})
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "communities.createRole")
}

// AddMember adds a user to the community.
func (c *Community) AddMember(ctx context.Context, userID ID) error {
	if c == nil {
		return ErrObjectClientUnavailable
	}
	if err := requireObjectID(c.ID, "community"); err != nil {
		return err
	}
	if err := requireObjectID(userID, "user"); err != nil {
		return err
	}
	result, err := callObject(c.client, ctx, &protoCommunities.AddMember{CommunityId: uint64(c.ID), UserId: uint64(userID)})
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "communities.addMember")
}

// Unban removes the ban for the requested users.
func (c *Community) Unban(ctx context.Context, userIDs ...ID) error {
	if c == nil {
		return ErrObjectClientUnavailable
	}
	if err := requireObjectID(c.ID, "community"); err != nil {
		return err
	}
	ids, err := objectIDs(userIDs, "user")
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return fmt.Errorf("user IDs are required")
	}
	result, err := callObject(c.client, ctx, &protoCommunities.UnbanMembers{CommunityId: uint64(c.ID), MemberIds: ids})
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "communities.unbanMembers")
}

// SetDefaultPermissions updates the permissions inherited by new members.
func (c *Community) SetDefaultPermissions(ctx context.Context, permissions uint64) error {
	if c == nil {
		return ErrObjectClientUnavailable
	}
	if err := requireObjectID(c.ID, "community"); err != nil {
		return err
	}
	result, err := callObject(c.client, ctx, &protoCommunities.EditDefaultPermissions{CommunityId: uint64(c.ID), Permissions: permissions})
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "communities.editDefaultPermissions")
}

func (c *Channel) channelRef() (ChannelRef, error) {
	if c == nil {
		return ChannelRef{}, ErrObjectClientUnavailable
	}
	ref := ChannelRef{CommunityID: c.CommunityID, ChannelID: c.ID}
	if err := requireObjectID(ref.CommunityID, "community"); err != nil {
		return ChannelRef{}, err
	}
	if err := requireObjectID(ref.ChannelID, "channel"); err != nil {
		return ChannelRef{}, err
	}
	return ref, nil
}

// Send sends a message to the channel and returns a usable message object.
func (c *Channel) Send(ctx context.Context, params MessageSendParams) (*Message, error) {
	ref, err := c.channelRef()
	if err != nil {
		return nil, err
	}
	return sendMessage(ctx, c.client, ChannelChat(ref.CommunityID, ref.ChannelID), params)
}

// Messages returns channel history.
func (c *Channel) Messages(ctx context.Context, params MessageHistoryParams) (*MessageHistory, error) {
	ref, err := c.channelRef()
	if err != nil {
		return nil, err
	}
	params.Chat = ChannelChat(ref.CommunityID, ref.ChannelID)
	return getMessageHistory(ctx, c.client, params)
}

// History is an explicit alias for Messages.
func (c *Channel) History(ctx context.Context, params MessageHistoryParams) (*MessageHistory, error) {
	return c.Messages(ctx, params)
}

// PinnedMessages returns the pinned messages in the channel.
func (c *Channel) PinnedMessages(ctx context.Context) (*MessageHistory, error) {
	ref, err := c.channelRef()
	if err != nil {
		return nil, err
	}
	chat, err := refChat(ref)
	if err != nil {
		return nil, err
	}
	result, err := callObject(c.client, ctx, &protoMessages.GetPinnedMessages{ChatRef: chat})
	if err != nil {
		return nil, err
	}
	value := result.GetMessages()
	if value == nil {
		return nil, &rpc.UnexpectedResultError{Method: "messages.getPinnedMessages"}
	}
	return messageHistoryFromProto(value, c.client), nil
}

// Search finds messages in this channel.
func (c *Channel) Search(ctx context.Context, params MessageSearchParams) (*MessageHistory, error) {
	ref, err := c.channelRef()
	if err != nil {
		return nil, err
	}
	params.Chat = ChannelChat(ref.CommunityID, ref.ChannelID)
	params.Scoped = true
	return searchMessages(ctx, c.client, params)
}

// Members returns the ordered member list visible in the channel.
func (c *Channel) Members(ctx context.Context) ([]*MemberListEntry, error) {
	ref, err := c.channelRef()
	if err != nil {
		return nil, err
	}
	result, err := callObject(c.client, ctx, &protoCommunities.GetChannelMembers{CommunityId: uint64(ref.CommunityID), ChannelId: uint64(ref.ChannelID)})
	if err != nil {
		return nil, err
	}
	value := result.GetMemberList()
	if value == nil {
		return nil, &rpc.UnexpectedResultError{Method: "communities.getChannelMembers"}
	}
	entries := make([]*MemberListEntry, 0, len(value.GetEntries()))
	for _, entry := range value.GetEntries() {
		entries = append(entries, MemberListEntryFromProto(entry, c.client))
	}
	return entries, nil
}

// Edit updates the channel and applies successful changes to the object.
func (c *Channel) Edit(ctx context.Context, options ChannelEditOptions) error {
	ref, err := c.channelRef()
	if err != nil {
		return err
	}
	channel, err := ref.ToProto()
	if err != nil {
		return err
	}
	request := &protoCommunities.EditChannel{
		Channel:         channel,
		Name:            options.Name,
		Position:        options.Position,
		Explicit:        options.Explicit,
		Description:     options.Description,
		SlowmodeSeconds: options.SlowmodeSeconds,
		PreferredRegion: options.PreferredRegion,
	}
	if options.ParentID != nil {
		request.ParentId = uint64Pointer(*options.ParentID)
	}
	if options.HighlightedMessageID != nil {
		request.HighlightedMsgId = &protoCommunities.EditChannel_HighlightedMessage{
			HighlightedMsgId: uint64Pointer(*options.HighlightedMessageID),
		}
	}
	result, err := callObject(c.client, ctx, request)
	if err != nil {
		return err
	}
	if err := rpc.EnsureVoid(result, "communities.editChannel"); err != nil {
		return err
	}
	if options.Name != nil {
		c.Name = *options.Name
	}
	if options.Position != nil {
		c.Position = *options.Position
	}
	if options.ParentID != nil {
		c.ParentID = idPointer(*options.ParentID)
	}
	if options.Explicit != nil {
		if *options.Explicit {
			c.Flags |= uint64(protoTypes.Channel_EXPLICIT)
		} else {
			c.Flags &^= uint64(protoTypes.Channel_EXPLICIT)
		}
	}
	if options.HighlightedMessageID != nil {
		c.HighlightedMessageID = idPointer(*options.HighlightedMessageID)
	}
	if options.Description != nil {
		c.Description = cloneString(options.Description)
	}
	if options.SlowmodeSeconds != nil {
		c.SlowmodeSeconds = cloneUint32(options.SlowmodeSeconds)
	}
	if options.PreferredRegion != nil {
		c.PreferredRegion = cloneString(options.PreferredRegion)
	}
	return nil
}

// Delete deletes the channel.
func (c *Channel) Delete(ctx context.Context) error {
	ref, err := c.channelRef()
	if err != nil {
		return err
	}
	channel, err := ref.ToProto()
	if err != nil {
		return err
	}
	result, err := callObject(c.client, ctx, &protoCommunities.DeleteChannel{Channel: channel})
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "communities.deleteChannel")
}

// CreateInvite creates an invite for the channel.
func (c *Channel) CreateInvite(ctx context.Context, options InviteOptions) (*Invite, error) {
	ref, err := c.channelRef()
	if err != nil {
		return nil, err
	}
	chat, err := refChat(ref)
	if err != nil {
		return nil, err
	}
	request := &protoChats.CreateChatInvite{ChatRef: chat}
	if options.ExpiresAt != 0 {
		request.ExpiresAt = &options.ExpiresAt
	}
	if options.MaxUses != 0 {
		request.MaxUses = &options.MaxUses
	}
	result, err := callObject(c.client, ctx, request)
	if err != nil {
		return nil, err
	}
	value := result.GetCreatedInvite()
	if value == nil {
		return nil, &rpc.UnexpectedResultError{Method: "chats.createChatInvite"}
	}
	return &Invite{Code: value.GetCode(), Raw: value}, nil
}

// Invites lists the channel's active invites.
func (c *Channel) Invites(ctx context.Context) ([]*InvitePreview, error) {
	ref, err := c.channelRef()
	if err != nil {
		return nil, err
	}
	chat, err := refChat(ref)
	if err != nil {
		return nil, err
	}
	result, err := callObject(c.client, ctx, &protoChats.ListChatInvites{ChatRef: chat})
	if err != nil {
		return nil, err
	}
	value := result.GetInviteList()
	if value == nil {
		return nil, &rpc.UnexpectedResultError{Method: "chats.listChatInvites"}
	}
	invites := make([]*InvitePreview, 0, len(value.GetInvites()))
	for _, invite := range value.GetInvites() {
		if invite == nil {
			continue
		}
		invites = append(invites, &InvitePreview{
			Code:       invite.GetCode(),
			CreatorID:  ID(invite.GetCreatorId()),
			TargetID:   ID(invite.GetTargetId()),
			TargetType: invite.GetTargetType(),
			ExpiresAt:  cloneUint64(invite.ExpiresAt),
			Raw:        invite,
		})
	}
	return invites, nil
}

// DeleteInvite revokes an invite by code.
func (c *Channel) DeleteInvite(ctx context.Context, code string) error {
	if code == "" {
		return fmt.Errorf("invite code is required")
	}
	if _, err := c.channelRef(); err != nil {
		return err
	}
	result, err := callObject(c.client, ctx, &protoChats.DeleteChatInvite{Code: code})
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "chats.deleteChatInvite")
}

// Reply replies to the message with plain text.
func (m *Message) Reply(ctx context.Context, content string) (*Message, error) {
	if err := m.requireMessage(); err != nil {
		return nil, err
	}
	return sendMessage(ctx, m.client, m.Chat, MessageSendParams{Content: content, ReplyTo: m.ID})
}

// ReplyWith replies using the full send options.
func (m *Message) ReplyWith(ctx context.Context, params MessageSendParams) (*Message, error) {
	if err := m.requireMessage(); err != nil {
		return nil, err
	}
	params.ReplyTo = m.ID
	return sendMessage(ctx, m.client, m.Chat, params)
}

// Edit updates a message's text.
func (m *Message) Edit(ctx context.Context, content string) error {
	return m.EditWith(ctx, MessageEditParams{Content: &content})
}

// EditWith updates a message using the full edit options.
func (m *Message) EditWith(ctx context.Context, params MessageEditParams) error {
	if err := m.requireMessage(); err != nil {
		return err
	}
	params.Chat = m.Chat
	params.MessageID = m.ID
	if err := editMessage(ctx, m.client, params); err != nil {
		return err
	}
	if params.Content != nil {
		m.Content = *params.Content
	}
	return nil
}

// Delete deletes the message.
func (m *Message) Delete(ctx context.Context) error {
	if err := m.requireMessage(); err != nil {
		return err
	}
	return deleteMessages(ctx, m.client, MessageDeleteParams{Chat: m.Chat, MessageIDs: []ID{m.ID}})
}

// React adds a unicode or custom emoji reaction.
func (m *Message) React(ctx context.Context, emoji Emoji) error {
	if err := m.requireMessage(); err != nil {
		return err
	}
	value, err := emoji.ToProto()
	if err != nil {
		return err
	}
	chat, err := m.Chat.ToProto()
	if err != nil {
		return err
	}
	result, err := callObject(m.client, ctx, &protoReactions.AddReaction{ChatRef: chat, MessageId: uint64(m.ID), Emoji: value})
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "reactions.addReaction")
}

// Unreact removes a unicode or custom emoji reaction.
func (m *Message) Unreact(ctx context.Context, emoji Emoji) error {
	if err := m.requireMessage(); err != nil {
		return err
	}
	value, err := emoji.ToProto()
	if err != nil {
		return err
	}
	chat, err := m.Chat.ToProto()
	if err != nil {
		return err
	}
	result, err := callObject(m.client, ctx, &protoReactions.RemoveReaction{ChatRef: chat, MessageId: uint64(m.ID), Emoji: value})
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "reactions.removeReaction")
}

// Pin pins the message.
func (m *Message) Pin(ctx context.Context) error { return m.SetPinned(ctx, true) }

// Unpin removes the message pin.
func (m *Message) Unpin(ctx context.Context) error { return m.SetPinned(ctx, false) }

// SetPinned explicitly sets the message pin state.
func (m *Message) SetPinned(ctx context.Context, pinned bool) error {
	if err := m.requireMessage(); err != nil {
		return err
	}
	chat, err := m.Chat.ToProto()
	if err != nil {
		return err
	}
	result, err := callObject(m.client, ctx, &protoMessages.SetMessagePin{ChatRef: chat, MessageId: uint64(m.ID), Pin: pinned})
	if err != nil {
		return err
	}
	if err := rpc.EnsureVoid(result, "messages.setMessagePin"); err != nil {
		return err
	}
	m.Pinned = pinned
	return nil
}

// Forward forwards the message to another chat.
func (m *Message) Forward(ctx context.Context, destination ChatRef) error {
	if err := m.requireMessage(); err != nil {
		return err
	}
	from, err := m.Chat.ToProto()
	if err != nil {
		return err
	}
	to, err := destination.ToProto()
	if err != nil {
		return err
	}
	result, err := callObject(m.client, ctx, &protoMessages.ForwardMessage{ChatRef: to, From: from, MessageIds: []uint64{uint64(m.ID)}})
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "messages.forwardMessage")
}

func (m *Message) requireMessage() error {
	if m == nil {
		return ErrObjectClientUnavailable
	}
	if err := requireObjectID(m.ID, "message"); err != nil {
		return err
	}
	if !m.Chat.Valid() {
		return fmt.Errorf("message chat reference is invalid")
	}
	return requireObjectClient(m.client)
}

// Edit updates the member's nickname or roles.
func (m *CommunityMember) Edit(ctx context.Context, options MemberEditOptions) error {
	if m == nil {
		return ErrObjectClientUnavailable
	}
	if err := requireObjectID(m.CommunityID, "community"); err != nil {
		return err
	}
	if err := requireObjectID(m.ID, "member"); err != nil {
		return err
	}
	request := &protoCommunities.EditMember{CommunityId: uint64(m.CommunityID), MemberId: uint64(m.ID), Nickname: options.Nickname}
	if options.RoleIDs != nil {
		ids, err := objectIDs(options.RoleIDs, "role")
		if err != nil {
			return err
		}
		request.RoleIds = &protoCommunities.CommunityMemberRoleIds{RoleIds: ids}
	}
	result, err := callObject(m.client, ctx, request)
	if err != nil {
		return err
	}
	if err := rpc.EnsureVoid(result, "communities.editMember"); err != nil {
		return err
	}
	if options.Nickname != nil {
		m.Nickname = cloneString(options.Nickname)
	}
	if options.RoleIDs != nil {
		m.RoleIDs = append([]ID(nil), options.RoleIDs...)
	}
	return nil
}

// SetRoles replaces all roles on the member.
func (m *CommunityMember) SetRoles(ctx context.Context, roleIDs ...ID) error {
	return m.Edit(ctx, MemberEditOptions{RoleIDs: append([]ID{}, roleIDs...)})
}

// AddRole adds a role if it is not already assigned.
func (m *CommunityMember) AddRole(ctx context.Context, roleID ID) error {
	if m == nil {
		return ErrObjectClientUnavailable
	}
	if err := requireObjectID(roleID, "role"); err != nil {
		return err
	}
	if m.Partial {
		if err := m.Fetch(ctx); err != nil {
			return err
		}
	}
	for _, current := range m.RoleIDs {
		if current == roleID {
			return nil
		}
	}
	roles := append(append([]ID(nil), m.RoleIDs...), roleID)
	return m.SetRoles(ctx, roles...)
}

// RemoveRole removes a role if it is assigned.
func (m *CommunityMember) RemoveRole(ctx context.Context, roleID ID) error {
	if m == nil {
		return ErrObjectClientUnavailable
	}
	if err := requireObjectID(roleID, "role"); err != nil {
		return err
	}
	if m.Partial {
		if err := m.Fetch(ctx); err != nil {
			return err
		}
	}
	roles := make([]ID, 0, len(m.RoleIDs))
	found := false
	for _, current := range m.RoleIDs {
		if current == roleID {
			found = true
			continue
		}
		roles = append(roles, current)
	}
	if !found {
		return nil
	}
	return m.SetRoles(ctx, roles...)
}

// Ban permanently or temporarily bans the member.
func (m *CommunityMember) Ban(ctx context.Context, options BanOptions) error {
	return m.remove(ctx, options, true)
}

// Kick removes the member without creating a ban.
func (m *CommunityMember) Kick(ctx context.Context, reason string) error {
	return m.remove(ctx, BanOptions{Reason: reason}, false)
}

// Send opens a direct chat with the member and sends a message.
func (m *CommunityMember) Send(ctx context.Context, params MessageSendParams) (*Message, error) {
	if m == nil {
		return nil, ErrObjectClientUnavailable
	}
	if err := requireObjectID(m.ID, "member"); err != nil {
		return nil, err
	}
	if err := requireObjectClient(m.client); err != nil {
		return nil, err
	}
	return sendMessage(ctx, m.client, UserChat(m.ID), params)
}

func (m *CommunityMember) remove(ctx context.Context, options BanOptions, ban bool) error {
	if m == nil {
		return ErrObjectClientUnavailable
	}
	if err := requireObjectID(m.CommunityID, "community"); err != nil {
		return err
	}
	if err := requireObjectID(m.ID, "member"); err != nil {
		return err
	}
	request := &protoCommunities.RemoveMembers{CommunityId: uint64(m.CommunityID), MemberIds: []uint64{uint64(m.ID)}}
	if ban {
		if options.Until == nil {
			permanent := uint64(0)
			request.Until = &permanent
		} else {
			request.Until = options.Until
		}
	}
	request.DeleteMessagesSince = options.DeleteMessagesSince
	if options.Reason != "" {
		request.Reason = &options.Reason
	}
	result, err := callObject(m.client, ctx, request)
	if err != nil {
		return err
	}
	if removed := result.GetRemovedMembers(); removed != nil {
		for _, member := range removed.GetMembers() {
			if ID(member.GetUserId()) == m.ID {
				return nil
			}
		}
		return ErrNotFound
	}
	return rpc.EnsureVoid(result, "communities.removeMembers")
}

// Edit updates the role while retaining fields omitted from options.
func (r *CommunityRole) Edit(ctx context.Context, options RoleEditOptions) error {
	if r == nil {
		return ErrObjectClientUnavailable
	}
	if err := requireObjectID(r.CommunityID, "community"); err != nil {
		return err
	}
	if err := requireObjectID(r.ID, "role"); err != nil {
		return err
	}
	if r.Partial {
		if err := r.Fetch(ctx); err != nil {
			return err
		}
	}
	name := r.Name
	permissions := r.Permissions
	priority := r.Priority
	color := r.Color
	separated := r.Separated
	public := r.Public
	if options.Name != nil {
		name = *options.Name
	}
	if options.Permissions != nil {
		permissions = *options.Permissions
	}
	if options.Priority != nil {
		priority = *options.Priority
	}
	if options.Color != nil {
		color = *options.Color
	}
	if options.Separated != nil {
		separated = *options.Separated
	}
	if options.Public != nil {
		public = *options.Public
	}
	if name == "" {
		return fmt.Errorf("role name is required")
	}
	result, err := callObject(r.client, ctx, &protoCommunities.EditRole{
		Id:          uint64(r.ID),
		CommunityId: uint64(r.CommunityID),
		Name:        name,
		Permissions: permissions,
		Priority:    priority,
		Color:       color,
		Separated:   separated,
		Public:      public,
	})
	if err != nil {
		return err
	}
	if err := rpc.EnsureVoid(result, "communities.editRole"); err != nil {
		return err
	}
	r.Name, r.Permissions, r.Priority, r.Color = name, permissions, priority, color
	r.Separated, r.Public = separated, public
	return nil
}

// SetPermissions replaces all role permissions.
func (r *CommunityRole) SetPermissions(ctx context.Context, permissions uint64) error {
	return r.Edit(ctx, RoleEditOptions{Permissions: &permissions})
}

// AddPermissions grants permission bits to the role.
func (r *CommunityRole) AddPermissions(ctx context.Context, permissions uint64) error {
	if r == nil {
		return ErrObjectClientUnavailable
	}
	if r.Partial {
		if err := r.Fetch(ctx); err != nil {
			return err
		}
	}
	value := r.Permissions | permissions
	return r.SetPermissions(ctx, value)
}

// RemovePermissions removes permission bits from the role.
func (r *CommunityRole) RemovePermissions(ctx context.Context, permissions uint64) error {
	if r == nil {
		return ErrObjectClientUnavailable
	}
	if r.Partial {
		if err := r.Fetch(ctx); err != nil {
			return err
		}
	}
	value := r.Permissions &^ permissions
	return r.SetPermissions(ctx, value)
}

// Delete deletes the role.
func (r *CommunityRole) Delete(ctx context.Context) error {
	if r == nil {
		return ErrObjectClientUnavailable
	}
	if err := requireObjectID(r.CommunityID, "community"); err != nil {
		return err
	}
	if err := requireObjectID(r.ID, "role"); err != nil {
		return err
	}
	result, err := callObject(r.client, ctx, &protoCommunities.DeleteRole{Id: uint64(r.ID), CommunityId: uint64(r.CommunityID)})
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "communities.deleteRole")
}

// ToProto converts a reaction emoji into the protocol representation.
type Emoji struct {
	Unicode string
	Custom  ID
}

func (e Emoji) ToProto() (*protoReactions.ReactionEmoji, error) {
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

func sendMessage(ctx context.Context, client *ObjectClient, chat ChatRef, params MessageSendParams) (*Message, error) {
	if params.Content == "" && len(params.Media) == 0 && params.BotInfo == nil {
		return nil, fmt.Errorf("message content is required")
	}
	chatProto, err := chat.ToProto()
	if err != nil {
		return nil, err
	}
	media, err := mediaRefsToProto(params.Media)
	if err != nil {
		return nil, err
	}
	request := &protoMessages.SendMessage{
		ChatRef:        chatProto,
		Message:        params.Content,
		Media:          media,
		Entities:       params.Entities,
		SuppressEmbeds: params.SuppressEmbeds,
	}
	if params.ReplyQuote != nil && params.ReplyTo == 0 {
		return nil, fmt.Errorf("reply quote requires a reply message ID")
	}
	if params.ReplyTo != 0 {
		request.ReplyTo = &protoMessages.SendMessage_ReplyTo{MessageId: uint64(params.ReplyTo)}
		if params.ReplyQuote != nil {
			request.ReplyTo.Quote = &protoMessages.SendMessage_ReplyTo_Quote{
				Message:  params.ReplyQuote.Content,
				Entities: params.ReplyQuote.Entities,
				Offset:   params.ReplyQuote.Offset,
			}
		}
	}
	if params.BotInfo != nil {
		request.BotInfo, err = botInfoToProto(params.BotInfo)
		if err != nil {
			return nil, err
		}
	}
	result, err := callObject(client, ctx, request)
	if err != nil {
		return nil, err
	}
	sent := result.GetSentMessage()
	if sent == nil {
		return nil, &rpc.UnexpectedResultError{Method: "messages.sendMessage"}
	}
	return &Message{ID: ID(sent.GetMessageId()), Chat: chat, AuthorID: 0, Content: params.Content, ReplyTo: params.ReplyTo, Partial: true, client: client}, nil
}

func editMessage(ctx context.Context, client *ObjectClient, params MessageEditParams) error {
	if err := requireObjectID(params.MessageID, "message"); err != nil {
		return err
	}
	chat, err := params.Chat.ToProto()
	if err != nil {
		return err
	}
	media, err := mediaRefsToProto(params.Media)
	if err != nil {
		return err
	}
	request := &protoMessages.EditMessage{
		ChatRef:        chat,
		MessageId:      uint64(params.MessageID),
		Message:        params.Content,
		RemoveMedia:    params.RemoveMedia,
		Media:          media,
		Entities:       params.Entities,
		SuppressEmbeds: params.SuppressEmbeds,
	}
	if params.Buttons != nil {
		request.Buttons, err = buttonsToProto(*params.Buttons)
		if err != nil {
			return err
		}
	}
	result, err := callObject(client, ctx, request)
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "messages.editMessage")
}

func deleteMessages(ctx context.Context, client *ObjectClient, params MessageDeleteParams) error {
	if len(params.MessageIDs) == 0 {
		return fmt.Errorf("message IDs are required")
	}
	ids, err := objectIDs(params.MessageIDs, "message")
	if err != nil {
		return err
	}
	chat, err := params.Chat.ToProto()
	if err != nil {
		return err
	}
	result, err := callObject(client, ctx, &protoMessages.DeleteMessage{ChatRef: chat, MessageIds: ids})
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "messages.deleteMessage")
}

func getMessageHistory(ctx context.Context, client *ObjectClient, params MessageHistoryParams) (*MessageHistory, error) {
	chat, err := params.Chat.ToProto()
	if err != nil {
		return nil, err
	}
	request := &protoMessages.GetHistory{ChatRef: chat}
	if params.Limit != 0 {
		request.Limit = &params.Limit
	}
	if params.Before != 0 {
		request.Offset = &protoMessages.GetHistory_Before{Before: uint64(params.Before)}
	}
	result, err := callObject(client, ctx, request)
	if err != nil {
		return nil, err
	}
	value := result.GetMessages()
	if value == nil {
		return nil, &rpc.UnexpectedResultError{Method: "messages.getHistory"}
	}
	return messageHistoryFromProto(value, client), nil
}

func searchMessages(ctx context.Context, client *ObjectClient, params MessageSearchParams) (*MessageHistory, error) {
	if params.Query == "" {
		return nil, fmt.Errorf("search query is required")
	}
	if params.Since != 0 && params.Before != 0 {
		return nil, fmt.Errorf("message search offset must be since or before")
	}
	request := &protoMessages.Search{Query: params.Query, Scoped: params.Scoped}
	if params.Chat != (ChatRef{}) {
		chat, err := params.Chat.ToProto()
		if err != nil {
			return nil, err
		}
		request.ChatRef = chat
	} else if params.Scoped {
		return nil, fmt.Errorf("scoped search requires a chat reference")
	}
	if params.Since != 0 {
		request.Offset = &protoMessages.Search_Since{Since: uint64(params.Since)}
	}
	if params.Before != 0 {
		request.Offset = &protoMessages.Search_Before{Before: uint64(params.Before)}
	}
	result, err := callObject(client, ctx, request)
	if err != nil {
		return nil, err
	}
	value := result.GetMessages()
	if value == nil {
		return nil, &rpc.UnexpectedResultError{Method: "messages.search"}
	}
	return messageHistoryFromProto(value, client), nil
}

func messageHistoryFromProto(value *protoMessages.Messages, client *ObjectClient) *MessageHistory {
	history := &MessageHistory{Raw: value}
	users := make(map[ID]*User, len(value.GetUsers()))
	for _, user := range value.GetUsers() {
		model := UserFromProto(user, client)
		history.Users = append(history.Users, model)
		if model != nil {
			users[model.ID] = model
		}
	}
	for _, message := range value.GetMessages() {
		model := MessageFromProto(message, client)
		if model != nil && users[model.AuthorID] != nil {
			model.Author = users[model.AuthorID]
		}
		history.Messages = append(history.Messages, model)
	}
	return history
}

func refChat(ref ChannelRef) (*protoRefs.ChatRef, error) {
	return ChannelChat(ref.CommunityID, ref.ChannelID).ToProto()
}

func objectIDs(values []ID, name string) ([]uint64, error) {
	if len(values) == 0 {
		return nil, nil
	}
	ids := make([]uint64, len(values))
	for i, value := range values {
		if err := requireObjectID(value, name); err != nil {
			return nil, err
		}
		ids[i] = uint64(value)
	}
	return ids, nil
}

func idPointer(value ID) *ID {
	if value == 0 {
		return nil
	}
	cloned := value
	return &cloned
}

func uint64Pointer(value ID) *uint64 {
	converted := uint64(value)
	return &converted
}

func mediaRefsToProto(values []*MediaRef) ([]*protoMedia.MediaRef, error) {
	if len(values) == 0 {
		return nil, nil
	}
	result := make([]*protoMedia.MediaRef, len(values))
	for i, value := range values {
		if value == nil {
			return nil, fmt.Errorf("message media %d is nil", i)
		}
		converted, err := value.ToProto()
		if err != nil {
			return nil, fmt.Errorf("message media %d: %w", i, err)
		}
		result[i] = converted
	}
	return result, nil
}

func botInfoToProto(value *MessageBotInfo) (*protoMessages.SendMessage_BotInfo, error) {
	if value == nil {
		return nil, nil
	}
	info := &protoMessages.SendMessage_BotInfo{}
	if value.Cloak != nil {
		info.Cloak = &protoMessages.SendMessage_BotInfo_MessageCloak{Name: value.Cloak.Name}
		if value.Cloak.PhotoID != 0 {
			photoID := uint64(value.Cloak.PhotoID)
			info.Cloak.PhotoId = &photoID
		}
	}
	if value.Buttons != nil {
		buttons, err := buttonsToProto(value.Buttons)
		if err != nil {
			return nil, err
		}
		info.Buttons = buttons
	}
	return info, nil
}

func buttonsToProto(value MessageButtons) (*protoMessages.SendMessage_BotInfo_Buttons, error) {
	if len(value) > 5 {
		return nil, fmt.Errorf("message buttons cannot have more than 5 rows")
	}
	buttons := &protoMessages.SendMessage_BotInfo_Buttons{Rows: make([]*protoMessages.SendMessage_BotInfo_Buttons_ButtonRow, len(value))}
	for i, row := range value {
		if len(row) > 5 {
			return nil, fmt.Errorf("message button row %d cannot have more than 5 buttons", i)
		}
		buttons.Rows[i] = &protoMessages.SendMessage_BotInfo_Buttons_ButtonRow{Buttons: make([]*protoMessages.MessageButton, len(row))}
		for j, button := range row {
			converted, err := buttonToProto(button)
			if err != nil {
				return nil, fmt.Errorf("button %d in row %d: %w", j, i, err)
			}
			buttons.Rows[i].Buttons[j] = converted
		}
	}
	return buttons, nil
}

func buttonToProto(value MessageButton) (*protoMessages.MessageButton, error) {
	if value.Label == "" {
		return nil, fmt.Errorf("button label is required")
	}
	actions := 0
	button := &protoMessages.MessageButton{Label: value.Label}
	if value.URL != "" {
		actions++
		button.Action = &protoMessages.MessageButton_Url{Url: &protoMessages.MessageButton_MessageButtonUrl{Url: value.URL}}
	}
	if value.Interaction != "" {
		actions++
		button.Action = &protoMessages.MessageButton_Interaction{Interaction: &protoMessages.MessageButton_MessageButtonInteraction{Data: value.Interaction}}
	}
	if value.Clipboard != "" {
		actions++
		button.Action = &protoMessages.MessageButton_Clipboard{Clipboard: &protoMessages.MessageButton_MessageButtonClipboard{Text: value.Clipboard}}
	}
	if actions != 1 {
		return nil, fmt.Errorf("button must define exactly one action")
	}
	return button, nil
}
