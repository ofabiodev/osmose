package types

import (
	"context"
	"fmt"
	"strings"

	"github.com/ofabiodev/osmose/internal/rpc"
	protoChats "github.com/ofabiodev/osmose/proto/chats"
	protoCommunities "github.com/ofabiodev/osmose/proto/communities"
	protoMessages "github.com/ofabiodev/osmose/proto/messages"
	protoUsers "github.com/ofabiodev/osmose/proto/users"
)

// Fetch fetches a user by ID through chats.getChat. The protocol's getProfile
// returns profile metadata, not a User; no username lookup or member scan is used.
func (m *UserManager) Fetch(ctx context.Context, id ID) (*User, error) {
	if m == nil {
		return nil, ErrObjectClientUnavailable
	}
	if err := m.validate(id); err != nil {
		return nil, err
	}
	ref, _ := UserChat(id).ToProto()
	result, err := callObject(m.client, ctx, &protoChats.GetChat{ChatRef: ref})
	if err != nil {
		return nil, err
	}
	if result.GetChat() == nil {
		return nil, &rpc.UnexpectedResultError{Method: "chats.getChat"}
	}
	for _, u := range result.GetChat().GetUsers() {
		if ID(u.GetId()) == id {
			return UserFromProto(u, m.client), nil
		}
	}
	return nil, ErrNotFound
}

func (m *UserManager) Lookup(ctx context.Context, username string) (*User, error) {
	if m == nil {
		return nil, ErrObjectClientUnavailable
	}
	if strings.TrimSpace(username) == "" {
		return nil, fmt.Errorf("username is required")
	}
	result, err := callObject(m.client, ctx, &protoUsers.LookupUsername{Username: username})
	if err != nil {
		return nil, err
	}
	if result.GetUserDetails().GetUser() == nil {
		return nil, &rpc.UnexpectedResultError{Method: "users.lookupUsername"}
	}
	return UserFromProto(result.GetUserDetails().GetUser(), m.client), nil
}

// List returns known cached users. The protocol has no global user list or
// bot API for creating, editing or deleting arbitrary user accounts.
func (m *UserManager) List() []*User { return m.ListCached() }

// List fetches communities visible to the bot in one RPC.
func (m *CommunityManager) List(ctx context.Context) ([]*Community, error) {
	if m == nil {
		return nil, ErrObjectClientUnavailable
	}
	result, err := callObject(m.client, ctx, &protoCommunities.GetCommunities{})
	if err != nil {
		return nil, err
	}
	if result.GetCommunities() == nil {
		return nil, &rpc.UnexpectedResultError{Method: "communities.getCommunities"}
	}
	var values []*Community
	for _, c := range result.GetCommunities().GetCommunities() {
		if c != nil {
			values = append(values, CommunityFromProto(c, m.client))
		}
	}
	return values, nil
}

// Fetch must use getCommunities: the pinned protocol has no getCommunity by ID.
func (m *CommunityManager) Fetch(ctx context.Context, id ID) (*Community, error) {
	if m == nil {
		return nil, ErrObjectClientUnavailable
	}
	if err := m.validate(id); err != nil {
		return nil, err
	}
	values, err := m.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range values {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, ErrNotFound
}

// Create returns only confirmation: Osmium supplies the new ID via updates.
func (m *CommunityManager) Create(ctx context.Context, name string) error {
	if m == nil {
		return ErrObjectClientUnavailable
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("community name is required")
	}
	result, err := callObject(m.client, ctx, &protoCommunities.CreateCommunity{Name: name})
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "communities.createCommunity")
}
func (m *CommunityManager) Edit(ctx context.Context, id ID, options CommunityEditOptions) error {
	return m.Ref(id).Edit(ctx, options)
}
func (m *CommunityManager) Delete(ctx context.Context, id ID) error { return m.Ref(id).Delete(ctx) }

func (m *ChannelManager) Fetch(ctx context.Context, id ID) (*Channel, error) {
	if m == nil {
		return nil, ErrObjectClientUnavailable
	}
	if err := m.validate(id); err != nil {
		return nil, err
	}
	ref, _ := ChannelChat(m.scope, id).ToProto()
	result, err := callObject(m.client, ctx, &protoChats.GetChat{ChatRef: ref})
	if err != nil {
		return nil, err
	}
	if result.GetChat() == nil {
		return nil, &rpc.UnexpectedResultError{Method: "chats.getChat"}
	}
	channel := ChannelFromProto(result.GetChat().GetChannel(), m.client)
	if channel == nil || channel.ID != id || channel.CommunityID != m.scope {
		return nil, ErrNotFound
	}
	return channel, nil
}
func (m *ChannelManager) List(ctx context.Context) ([]*Channel, error) {
	if m == nil {
		return nil, ErrObjectClientUnavailable
	}
	return (&Community{ID: m.scope, client: m.client}).Channels(ctx)
}
func (m *ChannelManager) Create(ctx context.Context, options ChannelCreateOptions) error {
	if m == nil {
		return ErrObjectClientUnavailable
	}
	return (&Community{ID: m.scope, client: m.client}).CreateChannel(ctx, options)
}
func (m *ChannelManager) Edit(ctx context.Context, id ID, options ChannelEditOptions) error {
	return m.Ref(id).Edit(ctx, options)
}
func (m *ChannelManager) Delete(ctx context.Context, id ID) error { return m.Ref(id).Delete(ctx) }

// Fetch sends exactly one getMembers request containing only the requested ID.
// The users sidecar is not proof of membership; a missing member fails closed.
func (m *MemberManager) Fetch(ctx context.Context, id ID) (*Member, error) {
	if m == nil {
		return nil, ErrObjectClientUnavailable
	}
	if err := m.validate(id); err != nil {
		return nil, err
	}
	values, err := m.FetchMany(ctx, id)
	if err != nil {
		return nil, err
	}
	for _, member := range values {
		if member.ID == id && member.CommunityID == m.scope {
			return member, nil
		}
	}
	return nil, ErrNotFound
}

// FetchMany fetches specific IDs in one RPC; use List explicitly for all members.
func (m *MemberManager) FetchMany(ctx context.Context, ids ...ID) ([]*Member, error) {
	if m == nil {
		return nil, ErrObjectClientUnavailable
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("member IDs are required; use List to fetch all members")
	}
	unique := make([]ID, 0, len(ids))
	seen := make(map[ID]bool, len(ids))
	for _, id := range ids {
		if err := m.validate(id); err != nil {
			return nil, err
		}
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	return (&Community{ID: m.scope, client: m.client}).Members(ctx, unique...)
}
func (m *MemberManager) List(ctx context.Context) ([]*Member, error) {
	if m == nil {
		return nil, ErrObjectClientUnavailable
	}
	return (&Community{ID: m.scope, client: m.client}).Members(ctx)
}

// Create adds an existing user; no membership is fabricated from a void result.
func (m *MemberManager) Create(ctx context.Context, userID ID) error {
	if m == nil {
		return ErrObjectClientUnavailable
	}
	return (&Community{ID: m.scope, client: m.client}).AddMember(ctx, userID)
}
func (m *MemberManager) Edit(ctx context.Context, id ID, options MemberEditOptions) error {
	return m.Ref(id).Edit(ctx, options)
}

// Delete removes a membership (kick), not the user's account.
func (m *MemberManager) Delete(ctx context.Context, id ID) error { return m.Ref(id).Kick(ctx, "") }

func (m *RoleManager) Fetch(ctx context.Context, id ID) (*Role, error) {
	if m == nil {
		return nil, ErrObjectClientUnavailable
	}
	if err := m.validate(id); err != nil {
		return nil, err
	}
	values, err := m.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, role := range values {
		if role.ID == id && role.CommunityID == m.scope {
			return role, nil
		}
	}
	return nil, ErrNotFound
}
func (m *RoleManager) List(ctx context.Context) ([]*Role, error) {
	if m == nil {
		return nil, ErrObjectClientUnavailable
	}
	return (&Community{ID: m.scope, client: m.client}).Roles(ctx)
}
func (m *RoleManager) Create(ctx context.Context, options RoleCreateOptions) error {
	if m == nil {
		return ErrObjectClientUnavailable
	}
	return (&Community{ID: m.scope, client: m.client}).CreateRole(ctx, options)
}
func (m *RoleManager) Edit(ctx context.Context, id ID, options RoleEditOptions) error {
	return m.Ref(id).Edit(ctx, options)
}
func (m *RoleManager) Delete(ctx context.Context, id ID) error { return m.Ref(id).Delete(ctx) }

// Fetch asks for a bounded history window around the ID. There is no getMessage
// endpoint in the pinned schema; absence from the response returns ErrNotFound.
func (m *MessageManager) Fetch(ctx context.Context, id ID) (*Message, error) {
	if m == nil {
		return nil, ErrObjectClientUnavailable
	}
	if err := m.validate(id); err != nil {
		return nil, err
	}
	ref, _ := m.chat.ToProto()
	limit := uint32(3)
	result, err := callObject(m.client, ctx, &protoMessages.GetHistory{ChatRef: ref, Limit: &limit, Offset: &protoMessages.GetHistory_Around{Around: uint64(id)}})
	if err != nil {
		return nil, err
	}
	if result.GetMessages() == nil {
		return nil, &rpc.UnexpectedResultError{Method: "messages.getHistory"}
	}
	for _, message := range messageHistoryFromProto(result.GetMessages(), m.client).Messages {
		if message != nil && message.ID == id && message.Chat == m.chat {
			return message, nil
		}
	}
	return nil, ErrNotFound
}

// List fetches one history page. Limit and Before retain the existing semantics.
func (m *MessageManager) List(ctx context.Context, params ...MessageHistoryParams) (*MessageHistory, error) {
	if m == nil {
		return nil, ErrObjectClientUnavailable
	}
	if len(params) > 1 {
		return nil, fmt.Errorf("at most one history parameter set is allowed")
	}
	var p MessageHistoryParams
	if len(params) == 1 {
		p = params[0]
	}
	p.Chat = m.chat
	return getMessageHistory(ctx, m.client, p)
}
func (m *MessageManager) Create(ctx context.Context, content string) (*Message, error) {
	return m.CreateWith(ctx, MessageSendParams{Content: content})
}
func (m *MessageManager) CreateWith(ctx context.Context, params MessageSendParams) (*Message, error) {
	if m == nil {
		return nil, ErrObjectClientUnavailable
	}
	return sendMessage(ctx, m.client, m.chat, params)
}
func (m *MessageManager) Edit(ctx context.Context, id ID, content string) error {
	return m.Ref(id).Edit(ctx, content)
}
func (m *MessageManager) Delete(ctx context.Context, id ID) error { return m.Ref(id).Delete(ctx) }
