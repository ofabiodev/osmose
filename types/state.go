package types

import (
	"context"
	"fmt"

	"github.com/ofabiodev/osmose/internal/state"
	protoChats "github.com/ofabiodev/osmose/proto/chats"
	protoCommunities "github.com/ofabiodev/osmose/proto/communities"
	"github.com/ofabiodev/osmose/proto/core"
	protoMessages "github.com/ofabiodev/osmose/proto/messages"
	protoTypes "github.com/ofabiodev/osmose/proto/types"
	"github.com/ofabiodev/osmose/proto/updates"
	protoUsers "github.com/ofabiodev/osmose/proto/users"
	"google.golang.org/protobuf/proto"
)

// CacheConfig configures bounded, opt-in client state. See Config.Cache.
type CacheConfig = state.Config

type objectFlight struct {
	done   chan struct{}
	result *core.RPCResult
	err    error
}

// Call is the shared service/object call boundary. Read calls in flight are
// coalesced, but completed Fetch calls never reuse cached results.
func (c *ObjectClient) Call(ctx context.Context, request proto.Message) (*core.RPCResult, error) {
	if err := requireObjectClient(c); err != nil {
		return nil, err
	}
	if request == nil {
		return nil, fmt.Errorf("request is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var key string
	revision := c.cache.Revision()
	if isStateRead(request) {
		data, err := proto.Marshal(request)
		if err != nil {
			return nil, err
		}
		key = fmt.Sprintf("%T:%d:%s", request, revision, data)
	}
	var flight *objectFlight
	if key != "" {
		c.flightMu.Lock()
		if pending := c.flights[key]; pending != nil {
			c.flightMu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-pending.done:
				if pending.result == nil {
					return nil, pending.err
				}
				return proto.Clone(pending.result).(*core.RPCResult), pending.err
			}
		}
		// Bound bookkeeping without spawning goroutines or queuing new work.
		if len(c.flights) < 256 {
			flight = &objectFlight{done: make(chan struct{})}
			c.flights[key] = flight
		}
		c.flightMu.Unlock()
	}
	result, err := c.call(ctx, request)
	if err == nil && result != nil && result.GetError() == nil {
		if key != "" {
			c.cache.Accept(revision, func(w *state.Writer) { observeResult(w, request, result) })
		} else {
			c.cache.Mutate(revision,
				func(w *state.Writer) { observeMutation(w, request, result) },
				func(w *state.Writer) { invalidateMutation(w, request, result) })
		}
	}
	if flight != nil {
		c.flightMu.Lock()
		flight.err = err
		if result != nil {
			flight.result = proto.Clone(result).(*core.RPCResult)
		}
		delete(c.flights, key)
		close(flight.done)
		c.flightMu.Unlock()
	}
	return result, err
}

func isStateRead(request proto.Message) bool {
	switch request.(type) {
	case *protoCommunities.GetCommunities, *protoCommunities.GetChannels,
		*protoCommunities.GetMembers, *protoCommunities.GetRoles, *protoCommunities.GetChannelMembers,
		*protoChats.GetChat, *protoChats.GetChats, *protoChats.GetChatMembers,
		*protoMessages.GetHistory, *protoMessages.Search, *protoMessages.GetPinnedMessages,
		*protoUsers.LookupUsername, *protoUsers.GetProfile:
		return true
	default:
		return false
	}
}

func entityKey(kind state.Kind, scope, id ID) state.Key {
	return state.Key{Kind: kind, Scope: uint64(scope), ID: uint64(id)}
}

func messageKey(chat ChatRef, id ID) state.Key {
	key := state.Key{Kind: state.Message, ID: uint64(id)}
	switch {
	case chat.CommunityID != 0:
		key.Chat, key.Scope, key.Parent = 1, uint64(chat.CommunityID), uint64(chat.ChannelID)
	case chat.UserID != 0:
		key.Chat, key.Parent = 2, uint64(chat.UserID)
	case chat.GroupID != 0:
		key.Chat, key.Parent = 3, uint64(chat.GroupID)
	case chat.Self:
		key.Chat = 4
	}
	return key
}

func putUser(w *state.Writer, v *protoTypes.User) {
	if v != nil {
		w.Put(entityKey(state.User, 0, ID(v.GetId())), v, false)
	}
}
func putCommunity(w *state.Writer, v *protoTypes.Community) {
	if v != nil {
		w.Put(entityKey(state.Community, 0, ID(v.GetId())), v, false)
	}
}
func putChannel(w *state.Writer, v *protoTypes.Channel) {
	if v != nil && v.GetCommunityId() != 0 {
		w.Put(entityKey(state.Channel, ID(v.GetCommunityId()), ID(v.GetId())), v, false)
	}
}
func putMember(w *state.Writer, v *protoTypes.CommunityMember, scope ID) {
	if v == nil {
		return
	}
	if v.GetCommunityId() == 0 {
		v = proto.Clone(v).(*protoTypes.CommunityMember)
		v.CommunityId = uint64(scope)
	}
	if v.GetCommunityId() != 0 {
		w.Put(entityKey(state.Member, ID(v.GetCommunityId()), ID(v.GetId())), v, false)
	}
}
func putRole(w *state.Writer, v *protoTypes.CommunityRole, scope ID) {
	if v == nil {
		return
	}
	if v.GetCommunityId() == 0 {
		v = proto.Clone(v).(*protoTypes.CommunityRole)
		v.CommunityId = uint64(scope)
	}
	if v.GetCommunityId() != 0 {
		w.Put(entityKey(state.Role, ID(v.GetCommunityId()), ID(v.GetId())), v, false)
	}
}
func putMessage(w *state.Writer, v *protoTypes.Message) {
	if v == nil {
		return
	}
	chat := ChatRefFromProto(v.GetChatRef())
	if chat.Valid() {
		w.Put(messageKey(chat, ID(v.GetMessageId())), v, false)
	}
}

func observeChat(w *state.Writer, value *protoChats.Chat) {
	if value == nil {
		return
	}
	for _, u := range value.GetUsers() {
		putUser(w, u)
	}
	putChannel(w, value.GetChannel())
	putMessage(w, value.GetMessage())
}

func observeResult(w *state.Writer, request proto.Message, result *core.RPCResult) {
	switch {
	case result.GetCommunities() != nil:
		// The response is a full list, so removed communities must not remain cached.
		visible := make(map[uint64]bool, len(result.GetCommunities().GetCommunities()))
		for _, v := range result.GetCommunities().GetCommunities() {
			visible[v.GetId()] = true
		}
		w.DeleteWhere(func(k state.Key) bool {
			return (k.Kind == state.Community && !visible[k.ID]) || (k.Scope != 0 && !visible[k.Scope])
		})
		for _, v := range result.GetCommunities().GetCommunities() {
			putCommunity(w, v)
		}
		for _, id := range result.GetCommunities().GetUnavailable() {
			invalidateCommunity(w, id)
		}
	case result.GetChannels() != nil:
		if r, ok := request.(*protoCommunities.GetChannels); ok {
			visible := make(map[uint64]bool, len(result.GetChannels().GetChannels()))
			for _, v := range result.GetChannels().GetChannels() {
				visible[v.GetId()] = true
			}
			w.DeleteWhere(func(k state.Key) bool {
				return k.Scope == r.GetCommunityId() && ((k.Kind == state.Channel && !visible[k.ID]) || (k.Kind == state.Message && !visible[k.Parent]))
			})
		}
		for _, v := range result.GetChannels().GetChannels() {
			putChannel(w, v)
		}
		for _, v := range result.GetChannels().GetMessages() {
			putMessage(w, v)
		}
	case result.GetMembers() != nil:
		var scope ID
		if r, ok := request.(*protoCommunities.GetMembers); ok {
			scope = ID(r.GetCommunityId())
			// Missing targeted IDs are authoritative absence, not negative cache entries.
			if len(r.GetMemberIds()) != 0 {
				for _, id := range r.GetMemberIds() {
					w.Delete(entityKey(state.Member, scope, ID(id)))
				}
			} else if len(result.GetMembers().GetMembers()) != 0 || len(result.GetMembers().GetUsers()) == 0 {
				w.DeleteWhere(func(k state.Key) bool { return k.Kind == state.Member && k.Scope == uint64(scope) })
			}
		}
		for _, v := range result.GetMembers().GetUsers() {
			putUser(w, v)
		}
		for _, v := range result.GetMembers().GetMembers() {
			putMember(w, v, scope)
		}
	case result.GetCommunityRoles() != nil:
		var scope ID
		if r, ok := request.(*protoCommunities.GetRoles); ok {
			scope = ID(r.GetCommunityId())
		}
		w.DeleteWhere(func(k state.Key) bool { return k.Kind == state.Role && k.Scope == uint64(scope) })
		for _, v := range result.GetCommunityRoles().GetRoles() {
			putRole(w, v, scope)
		}
	case result.GetMessages() != nil:
		for _, v := range result.GetMessages().GetUsers() {
			putUser(w, v)
		}
		for _, v := range result.GetMessages().GetMembers() {
			putMember(w, v, 0)
		}
		for _, v := range result.GetMessages().GetMessages() {
			putMessage(w, v)
		}
	case result.GetChat() != nil:
		observeChat(w, result.GetChat())
	case result.GetChats() != nil:
		for _, v := range result.GetChats().GetUsers() {
			putUser(w, v)
		}
		for _, v := range result.GetChats().GetChannels() {
			putChannel(w, v)
		}
		for _, v := range result.GetChats().GetMessages() {
			putMessage(w, v)
		}
	case result.GetChatMembers() != nil:
		for _, v := range result.GetChatMembers().GetUsers() {
			putUser(w, v)
		}
	case result.GetUserDetails() != nil:
		putUser(w, result.GetUserDetails().GetUser())
	case result.GetMemberList() != nil:
		for _, v := range result.GetMemberList().GetEntries() {
			putUser(w, v.GetUser().GetUser())
		}
	}
}

func invalidateCommunity(w *state.Writer, id uint64) {
	w.DeleteWhere(func(k state.Key) bool {
		return (k.Kind == state.Community && k.ID == id) || (k.Scope == id && k.Kind != state.User)
	})
}

func invalidateChannel(w *state.Writer, communityID, channelID uint64) {
	w.Delete(state.Key{Kind: state.Channel, Scope: communityID, ID: channelID})
	w.DeleteWhere(func(k state.Key) bool {
		return k.Kind == state.Message && k.Chat == 1 && k.Scope == communityID && k.Parent == channelID
	})
}

// ApplyUpdate synchronizes state before the update reaches the handler queue.
// It does no network I/O and runs even if the bounded handler queue is full.
func (c *ObjectClient) ApplyUpdate(update *updates.Update) {
	if c == nil || update == nil {
		return
	}
	c.cache.Change(func(w *state.Writer) {
		switch v := update.GetUpdate().(type) {
		case *updates.Update_MessageCreated:
			putUser(w, v.MessageCreated.GetAuthor())
			putMessage(w, v.MessageCreated.GetMessage())
		case *updates.Update_Message:
			putMessage(w, v.Message.GetMessage())
		case *updates.Update_MessageDeleted:
			for _, id := range v.MessageDeleted.GetMessageIds() {
				w.Delete(messageKey(ChatRefFromProto(v.MessageDeleted.GetChatRef()), ID(id)))
			}
		case *updates.Update_Channel:
			putChannel(w, v.Channel.GetChannel())
		case *updates.Update_ChannelDeleted:
			r := v.ChannelDeleted.GetChannel()
			invalidateChannel(w, r.GetCommunityId(), r.GetChannelId())
		case *updates.Update_User:
			if v.User.GetUser() == nil {
				w.Delete(entityKey(state.User, 0, ID(v.User.GetUserId())))
			} else {
				putUser(w, v.User.GetUser())
			}
		case *updates.Update_UserStatus:
			updateStatus(w, v.UserStatus)
		case *updates.Update_UserStatusBatch:
			for _, u := range v.UserStatusBatch.GetUpdates() {
				updateStatus(w, u)
			}
		case *updates.Update_Community:
			id := v.Community.GetCommunityId()
			w.DeleteWhere(func(k state.Key) bool { return k.Kind == state.Role && k.Scope == id })
			if v.Community.GetCommunity() == nil {
				w.Delete(entityKey(state.Community, 0, ID(id)))
			} else {
				putCommunity(w, v.Community.GetCommunity())
			}
		case *updates.Update_CommunityDeleted:
			invalidateCommunity(w, v.CommunityDeleted.GetCommunityId())
		case *updates.Update_CommunityUnavailable:
			invalidateCommunity(w, v.CommunityUnavailable.GetCommunityId())
		case *updates.Update_CommunityMemberCreated:
			r := v.CommunityMemberCreated
			putUser(w, r.GetUser())
			if r.GetMember() != nil {
				putMember(w, r.GetMember(), ID(r.GetCommunityId()))
			} else {
				w.Put(entityKey(state.Member, ID(r.GetCommunityId()), ID(r.GetMemberId())), &protoTypes.CommunityMember{Id: r.GetMemberId(), CommunityId: r.GetCommunityId()}, true)
			}
		case *updates.Update_CommunityMember:
			r := v.CommunityMember
			if r.GetMember() == nil {
				w.Delete(entityKey(state.Member, ID(r.GetCommunityId()), ID(r.GetMemberId())))
			} else {
				putMember(w, r.GetMember(), ID(r.GetCommunityId()))
			}
		case *updates.Update_CommunityMemberDeleted:
			r := v.CommunityMemberDeleted
			w.Delete(entityKey(state.Member, ID(r.GetCommunityId()), ID(r.GetMemberId())))
		case *updates.Update_MemberList:
			// A list contains users/nicknames, not authoritative membership roles.
			for _, entry := range v.MemberList.GetEntries() {
				putUser(w, entry.GetUser().GetUser())
			}
		case *updates.Update_Chat:
			observeChat(w, v.Chat.GetChat())
		case *updates.Update_Group:
			for _, u := range v.Group.GetUsers() {
				putUser(w, u)
			}
		}
	})
}

func updateStatus(w *state.Writer, status *updates.UpdateUserStatus) {
	key := entityKey(state.User, 0, ID(status.GetUserId()))
	if entry, ok := w.Read(key); ok {
		user := proto.Clone(entry.Value).(*protoTypes.User)
		user.Status = status.GetStatus()
		w.Put(key, user, entry.Partial)
	}
}

// ClearCache also fences off in-flight reads; reconnect cannot resurrect stale
// permissions or entities that disappeared while the gateway was disconnected.
func (c *ObjectClient) ClearCache() {
	if c != nil {
		c.cache.Clear()
	}
}

func observeMutation(w *state.Writer, request proto.Message, result *core.RPCResult) {
	// Most mutations return void. Reject unexpected non-void responses before
	// changing local state, just as the public object methods do.
	if result.GetResult() != nil {
		if r, ok := request.(*protoMessages.SendMessage); ok && result.GetSentMessage() != nil {
			id := ID(result.GetSentMessage().GetMessageId())
			w.Put(messageKey(ChatRefFromProto(r.GetChatRef()), id), &protoTypes.Message{MessageId: uint64(id), ChatRef: r.GetChatRef(), Message: r.GetMessage(), Entities: r.GetEntities()}, true)
		}
		if r, ok := request.(*protoCommunities.RemoveMembers); ok && result.GetRemovedMembers() != nil {
			for _, m := range result.GetRemovedMembers().GetMembers() {
				w.Delete(entityKey(state.Member, ID(r.GetCommunityId()), ID(m.GetUserId())))
			}
		}
		return
	}
	switch r := request.(type) {
	case *protoCommunities.DeleteCommunity:
		invalidateCommunity(w, r.GetCommunityId())
	case *protoCommunities.LeaveCommunity:
		invalidateCommunity(w, r.GetCommunityId())
	case *protoCommunities.DeleteChannel:
		invalidateChannel(w, r.GetChannel().GetCommunityId(), r.GetChannel().GetChannelId())
	case *protoCommunities.EditCommunity:
		key := entityKey(state.Community, 0, ID(r.GetCommunityId()))
		if entry, ok := w.Read(key); ok {
			v := proto.Clone(entry.Value).(*protoTypes.Community)
			if r.Name != nil {
				v.Name = *r.Name
			}
			if r.Username != nil {
				v.Username = r.Username
			}
			w.Put(key, v, entry.Partial)
		}
	case *protoCommunities.EditChannel:
		// Some edit fields map to server-maintained flags. Re-fetch rather than
		// pretend that a local projection contains the authoritative channel.
		w.Delete(entityKey(state.Channel, ID(r.GetChannel().GetCommunityId()), ID(r.GetChannel().GetChannelId())))
	case *protoCommunities.EditMember:
		key := entityKey(state.Member, ID(r.GetCommunityId()), ID(r.GetMemberId()))
		if entry, ok := w.Read(key); ok {
			v := proto.Clone(entry.Value).(*protoTypes.CommunityMember)
			if r.Nickname != nil {
				v.Nickname = r.Nickname
			}
			if r.RoleIds != nil {
				v.RoleIds = r.RoleIds.GetRoleIds()
			}
			w.Put(key, v, entry.Partial)
		}
	case *protoCommunities.RemoveMembers:
		for _, id := range r.GetMemberIds() {
			w.Delete(entityKey(state.Member, ID(r.GetCommunityId()), ID(id)))
		}
	case *protoCommunities.EditRole:
		putRole(w, &protoTypes.CommunityRole{Id: r.GetId(), CommunityId: r.GetCommunityId(), Name: r.GetName(), Permissions: r.GetPermissions(), Priority: r.GetPriority(), Color: r.GetColor(), Separated: r.GetSeparated(), Public: r.GetPublic()}, ID(r.GetCommunityId()))
	case *protoCommunities.DeleteRole:
		w.Delete(entityKey(state.Role, ID(r.GetCommunityId()), ID(r.GetId())))
		w.DeleteWhere(func(k state.Key) bool { return k.Kind == state.Member && k.Scope == r.GetCommunityId() })
	case *protoCommunities.EditDefaultPermissions:
		invalidateCommunity(w, r.GetCommunityId())
	case *protoMessages.EditMessage:
		w.Delete(messageKey(ChatRefFromProto(r.GetChatRef()), ID(r.GetMessageId())))
	case *protoMessages.DeleteMessage:
		for _, id := range r.GetMessageIds() {
			w.Delete(messageKey(ChatRefFromProto(r.GetChatRef()), ID(id)))
		}
	case *protoMessages.SetMessagePin:
		key := messageKey(ChatRefFromProto(r.GetChatRef()), ID(r.GetMessageId()))
		if entry, ok := w.Read(key); ok {
			v := proto.Clone(entry.Value).(*protoTypes.Message)
			pin := r.GetPin()
			v.Pinned = &pin
			w.Put(key, v, entry.Partial)
		}
	}
}

func invalidateMutation(w *state.Writer, request proto.Message, result *core.RPCResult) {
	if result.GetResult() != nil {
		if _, ok := request.(*protoCommunities.RemoveMembers); ok && result.GetRemovedMembers() != nil {
			observeMutation(w, request, result)
		}
		return
	}
	switch r := request.(type) {
	case *protoCommunities.EditCommunity:
		w.Delete(entityKey(state.Community, 0, ID(r.GetCommunityId())))
	case *protoCommunities.EditChannel:
		w.Delete(entityKey(state.Channel, ID(r.GetChannel().GetCommunityId()), ID(r.GetChannel().GetChannelId())))
	case *protoCommunities.EditMember:
		w.Delete(entityKey(state.Member, ID(r.GetCommunityId()), ID(r.GetMemberId())))
	case *protoCommunities.EditRole:
		w.Delete(entityKey(state.Role, ID(r.GetCommunityId()), ID(r.GetId())))
	case *protoMessages.SetMessagePin:
		w.Delete(messageKey(ChatRefFromProto(r.GetChatRef()), ID(r.GetMessageId())))
	default:
		observeMutation(w, request, result)
	}
}
