package types

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/ofabiodev/osmose/internal/state"
	protoTypes "github.com/ofabiodev/osmose/proto/types"
)

var (
	// ErrNotFound means a valid server response did not contain the requested object.
	ErrNotFound = errors.New("object not found")
	// ErrIncompleteObject prevents partial data from being used for destructive
	// read-modify-write operations, especially role and permission replacements.
	ErrIncompleteObject = errors.New("object is incomplete")
)

// Managers share one client-owned cache. Scoped managers are inexpensive views.
type Managers struct {
	Users       *UserManager
	Communities *CommunityManager
	Channels    *ChannelManager
	Members     *MemberManager
	Roles       *RoleManager
	Messages    *MessageManager
}

func newManagers(c *ObjectClient) *Managers {
	return &Managers{
		Users:       &UserManager{managerCore{client: c, kind: state.User}},
		Communities: &CommunityManager{managerCore{client: c, kind: state.Community}},
		Channels:    &ChannelManager{managerCore{client: c, kind: state.Channel}},
		Members:     &MemberManager{managerCore{client: c, kind: state.Member}},
		Roles:       &RoleManager{managerCore{client: c, kind: state.Role}},
		Messages:    &MessageManager{managerCore{client: c, kind: state.Message}},
	}
}

func (c *ObjectClient) Managers() *Managers { return c.managers }

// Clear discards every cached entity and fences off pending cache fills.
func (m *Managers) Clear() {
	if m != nil && m.Users != nil {
		m.Users.client.ClearCache()
	}
}

type managerCore struct {
	client *ObjectClient
	kind   state.Kind
	scope  ID
	chat   ChatRef
}

func (m managerCore) key(id ID) state.Key {
	if m.kind == state.Message {
		return messageKey(m.chat, id)
	}
	return entityKey(m.kind, m.scope, id)
}

func (m managerCore) validate(id ID) error {
	if err := requireObjectClient(m.client); err != nil {
		return err
	}
	if id == 0 {
		return fmt.Errorf("object ID is required")
	}
	switch m.kind {
	case state.Channel, state.Member, state.Role:
		if m.scope == 0 {
			return fmt.Errorf("community ID is required; use In(communityID)")
		}
	case state.Message:
		if !m.chat.Valid() {
			return fmt.Errorf("chat reference is required; use In(chat)")
		}
	}
	return nil
}

func (m managerCore) get(id ID) (state.Entry, bool) {
	if m.validate(id) != nil {
		return state.Entry{}, false
	}
	return m.client.cache.Get(m.key(id))
}

func (m managerCore) matches(k state.Key) bool {
	if k.Kind != m.kind {
		return false
	}
	if m.kind == state.Message && m.chat.Valid() {
		expected := m.key(ID(k.ID))
		return expected == k
	}
	return m.scope == 0 || k.Scope == uint64(m.scope)
}

// Invalidate evicts one object. It performs no RPC and does not delete it remotely.
func (m managerCore) Invalidate(id ID) {
	if m.client == nil || id == 0 {
		return
	}
	m.client.cache.Invalidate(func(k state.Key) bool {
		return m.matches(k) && k.ID == uint64(id)
	})
}

// Clear evicts the entries in this manager's scope.
func (m managerCore) Clear() {
	if m.client != nil {
		m.client.cache.Invalidate(m.matches)
	}
}

func (m managerCore) cached() []state.Entry {
	if m.client == nil {
		return nil
	}
	entries := m.client.cache.List(m.matches)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Key.ID < entries[j].Key.ID })
	return entries
}

// CommunityCollections preserves v0.2 methods such as Community.Members(ctx).
type CommunityCollections struct {
	Channels *ChannelManager
	Members  *MemberManager
	Roles    *RoleManager
}

func (c *Community) Collections() CommunityCollections {
	var client *ObjectClient
	var id ID
	if c != nil {
		client, id = c.client, c.ID
	}
	return CommunityCollections{
		Channels: &ChannelManager{managerCore{client: client, kind: state.Channel, scope: id}},
		Members:  &MemberManager{managerCore{client: client, kind: state.Member, scope: id}},
		Roles:    &RoleManager{managerCore{client: client, kind: state.Role, scope: id}},
	}
}

type ChannelCollections struct{ Messages *MessageManager }

func (c *Channel) Collections() ChannelCollections {
	var client *ObjectClient
	var chat ChatRef
	if c != nil {
		client, chat = c.client, ChannelChat(c.CommunityID, c.ID)
	}
	return ChannelCollections{Messages: &MessageManager{managerCore{client: client, kind: state.Message, chat: chat}}}
}

// UserManager provides local lookup and explicit network operations.
type UserManager struct{ managerCore }

// Ref creates a client-bound partial object without a cache lookup or RPC.
func (m *UserManager) Ref(id ID) *User {
	if m == nil {
		return nil
	}
	return &User{ID: id, Partial: true, client: m.client}
}

// Get returns an independent snapshot from cache only.
func (m *UserManager) Get(id ID) (*User, bool) {
	if m == nil {
		return nil, false
	}
	entry, ok := m.get(id)
	if !ok {
		return nil, false
	}
	value := UserFromProto(entry.Value.(*protoTypes.User), m.client)
	value.Partial = entry.Partial
	return value, true
}

// Resolve uses a complete cache entry or fetches the object directly.
func (m *UserManager) Resolve(ctx context.Context, id ID) (*User, error) {
	if m == nil {
		return nil, ErrObjectClientUnavailable
	}
	if err := ctxError(ctx); err != nil {
		return nil, err
	}
	if value, ok := m.Get(id); ok && !value.Partial {
		return value, nil
	}
	return m.Fetch(ctx, id)
}

// ListCached returns only cached snapshots, sorted by ID; it is not a remote list.
func (m *UserManager) ListCached() []*User {
	if m == nil {
		return nil
	}
	var values []*User
	for _, entry := range m.cached() {
		value := UserFromProto(entry.Value.(*protoTypes.User), m.client)
		value.Partial = entry.Partial
		values = append(values, value)
	}
	return values
}

// CommunityManager provides local lookup and explicit network operations.
type CommunityManager struct{ managerCore }

// Ref creates a client-bound partial object without a cache lookup or RPC.
func (m *CommunityManager) Ref(id ID) *Community {
	if m == nil {
		return nil
	}
	return &Community{ID: id, Partial: true, client: m.client}
}

// Get returns an independent snapshot from cache only.
func (m *CommunityManager) Get(id ID) (*Community, bool) {
	if m == nil {
		return nil, false
	}
	entry, ok := m.get(id)
	if !ok {
		return nil, false
	}
	value := CommunityFromProto(entry.Value.(*protoTypes.Community), m.client)
	value.Partial = entry.Partial
	return value, true
}

// Resolve uses a complete cache entry or fetches the object directly.
func (m *CommunityManager) Resolve(ctx context.Context, id ID) (*Community, error) {
	if m == nil {
		return nil, ErrObjectClientUnavailable
	}
	if err := ctxError(ctx); err != nil {
		return nil, err
	}
	if value, ok := m.Get(id); ok && !value.Partial {
		return value, nil
	}
	return m.Fetch(ctx, id)
}

// ListCached returns only cached snapshots, sorted by ID; it is not a remote list.
func (m *CommunityManager) ListCached() []*Community {
	if m == nil {
		return nil
	}
	var values []*Community
	for _, entry := range m.cached() {
		value := CommunityFromProto(entry.Value.(*protoTypes.Community), m.client)
		value.Partial = entry.Partial
		values = append(values, value)
	}
	return values
}

// ChannelManager provides local lookup and explicit network operations.
type ChannelManager struct{ managerCore }

// Ref creates a client-bound partial object without a cache lookup or RPC.
func (m *ChannelManager) Ref(id ID) *Channel {
	if m == nil {
		return nil
	}
	return &Channel{ID: id, CommunityID: m.scope, Partial: true, client: m.client}
}

// Get returns an independent snapshot from cache only.
func (m *ChannelManager) Get(id ID) (*Channel, bool) {
	if m == nil {
		return nil, false
	}
	entry, ok := m.get(id)
	if !ok {
		return nil, false
	}
	value := ChannelFromProto(entry.Value.(*protoTypes.Channel), m.client)
	value.Partial = entry.Partial
	return value, true
}

// Resolve uses a complete cache entry or fetches the object directly.
func (m *ChannelManager) Resolve(ctx context.Context, id ID) (*Channel, error) {
	if m == nil {
		return nil, ErrObjectClientUnavailable
	}
	if err := ctxError(ctx); err != nil {
		return nil, err
	}
	if value, ok := m.Get(id); ok && !value.Partial {
		return value, nil
	}
	return m.Fetch(ctx, id)
}

// ListCached returns only cached snapshots, sorted by ID; it is not a remote list.
func (m *ChannelManager) ListCached() []*Channel {
	if m == nil {
		return nil
	}
	var values []*Channel
	for _, entry := range m.cached() {
		value := ChannelFromProto(entry.Value.(*protoTypes.Channel), m.client)
		value.Partial = entry.Partial
		values = append(values, value)
	}
	return values
}

func (m *ChannelManager) In(communityID ID) *ChannelManager {
	if m == nil {
		return &ChannelManager{}
	}
	return &ChannelManager{managerCore{client: m.client, kind: state.Channel, scope: communityID}}
}

// MemberManager provides local lookup and explicit network operations.
type MemberManager struct{ managerCore }

// Ref creates a client-bound partial object without a cache lookup or RPC.
func (m *MemberManager) Ref(id ID) *Member {
	if m == nil {
		return nil
	}
	return &Member{ID: id, CommunityID: m.scope, Partial: true, client: m.client}
}

// Get returns an independent snapshot from cache only.
func (m *MemberManager) Get(id ID) (*Member, bool) {
	if m == nil {
		return nil, false
	}
	entry, ok := m.get(id)
	if !ok {
		return nil, false
	}
	value := CommunityMemberFromProto(entry.Value.(*protoTypes.CommunityMember), m.client)
	value.Partial = entry.Partial
	return value, true
}

// Resolve uses a complete cache entry or fetches the object directly.
func (m *MemberManager) Resolve(ctx context.Context, id ID) (*Member, error) {
	if m == nil {
		return nil, ErrObjectClientUnavailable
	}
	if err := ctxError(ctx); err != nil {
		return nil, err
	}
	if value, ok := m.Get(id); ok && !value.Partial {
		return value, nil
	}
	return m.Fetch(ctx, id)
}

// ListCached returns only cached snapshots, sorted by ID; it is not a remote list.
func (m *MemberManager) ListCached() []*Member {
	if m == nil {
		return nil
	}
	var values []*Member
	for _, entry := range m.cached() {
		value := CommunityMemberFromProto(entry.Value.(*protoTypes.CommunityMember), m.client)
		value.Partial = entry.Partial
		values = append(values, value)
	}
	return values
}

func (m *MemberManager) In(communityID ID) *MemberManager {
	if m == nil {
		return &MemberManager{}
	}
	return &MemberManager{managerCore{client: m.client, kind: state.Member, scope: communityID}}
}

// RoleManager provides local lookup and explicit network operations.
type RoleManager struct{ managerCore }

// Ref creates a client-bound partial object without a cache lookup or RPC.
func (m *RoleManager) Ref(id ID) *Role {
	if m == nil {
		return nil
	}
	return &Role{ID: id, CommunityID: m.scope, Partial: true, client: m.client}
}

// Get returns an independent snapshot from cache only.
func (m *RoleManager) Get(id ID) (*Role, bool) {
	if m == nil {
		return nil, false
	}
	entry, ok := m.get(id)
	if !ok {
		return nil, false
	}
	value := CommunityRoleFromProto(entry.Value.(*protoTypes.CommunityRole), m.client)
	value.Partial = entry.Partial
	return value, true
}

// Resolve uses a complete cache entry or fetches the object directly.
func (m *RoleManager) Resolve(ctx context.Context, id ID) (*Role, error) {
	if m == nil {
		return nil, ErrObjectClientUnavailable
	}
	if err := ctxError(ctx); err != nil {
		return nil, err
	}
	if value, ok := m.Get(id); ok && !value.Partial {
		return value, nil
	}
	return m.Fetch(ctx, id)
}

// ListCached returns only cached snapshots, sorted by ID; it is not a remote list.
func (m *RoleManager) ListCached() []*Role {
	if m == nil {
		return nil
	}
	var values []*Role
	for _, entry := range m.cached() {
		value := CommunityRoleFromProto(entry.Value.(*protoTypes.CommunityRole), m.client)
		value.Partial = entry.Partial
		values = append(values, value)
	}
	return values
}

func (m *RoleManager) In(communityID ID) *RoleManager {
	if m == nil {
		return &RoleManager{}
	}
	return &RoleManager{managerCore{client: m.client, kind: state.Role, scope: communityID}}
}

// MessageManager provides local lookup and explicit network operations.
type MessageManager struct{ managerCore }

// Ref creates a client-bound partial object without a cache lookup or RPC.
func (m *MessageManager) Ref(id ID) *Message {
	if m == nil {
		return nil
	}
	return &Message{ID: id, Chat: m.chat, Partial: true, client: m.client}
}

// Get returns an independent snapshot from cache only.
func (m *MessageManager) Get(id ID) (*Message, bool) {
	if m == nil {
		return nil, false
	}
	entry, ok := m.get(id)
	if !ok {
		return nil, false
	}
	value := MessageFromProto(entry.Value.(*protoTypes.Message), m.client)
	value.Partial = entry.Partial
	return value, true
}

// Resolve uses a complete cache entry or fetches the object directly.
func (m *MessageManager) Resolve(ctx context.Context, id ID) (*Message, error) {
	if m == nil {
		return nil, ErrObjectClientUnavailable
	}
	if err := ctxError(ctx); err != nil {
		return nil, err
	}
	if value, ok := m.Get(id); ok && !value.Partial {
		return value, nil
	}
	return m.Fetch(ctx, id)
}

// ListCached returns only cached snapshots, sorted by ID; it is not a remote list.
func (m *MessageManager) ListCached() []*Message {
	if m == nil {
		return nil
	}
	var values []*Message
	for _, entry := range m.cached() {
		value := MessageFromProto(entry.Value.(*protoTypes.Message), m.client)
		value.Partial = entry.Partial
		values = append(values, value)
	}
	return values
}

func (m *MessageManager) In(chat ChatRef) *MessageManager {
	if m == nil {
		return &MessageManager{}
	}
	return &MessageManager{managerCore{client: m.client, kind: state.Message, chat: chat}}
}

func ctxError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
