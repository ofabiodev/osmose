package types

import "context"

// Objects are caller-owned snapshots. Fetch replaces this snapshot after a
// successful RPC. Gateway events replace manager state, never public struct fields;
// synchronize access if an application mutates the same object from multiple goroutines.

func (o *User) Fetch(ctx context.Context) error {
	if o == nil {
		return ErrObjectClientUnavailable
	}
	if err := requireObjectClient(o.client); err != nil {
		return err
	}
	value, err := o.client.managers.Users.Fetch(ctx, o.ID)
	if err != nil {
		return err
	}
	if value.Partial {
		return ErrIncompleteObject
	}
	*o = *value
	return nil
}

// Profile fetches profile metadata for this user.
func (o *User) Profile(ctx context.Context) (*UserProfile, error) {
	if o == nil {
		return nil, ErrObjectClientUnavailable
	}
	if err := requireObjectClient(o.client); err != nil {
		return nil, err
	}
	return o.client.managers.Users.Profile(ctx, o.ID)
}

func (o *Community) Fetch(ctx context.Context) error {
	if o == nil {
		return ErrObjectClientUnavailable
	}
	if err := requireObjectClient(o.client); err != nil {
		return err
	}
	value, err := o.client.managers.Communities.Fetch(ctx, o.ID)
	if err != nil {
		return err
	}
	if value.Partial {
		return ErrIncompleteObject
	}
	*o = *value
	return nil
}

func (o *Channel) Fetch(ctx context.Context) error {
	if o == nil {
		return ErrObjectClientUnavailable
	}
	if err := requireObjectClient(o.client); err != nil {
		return err
	}
	value, err := o.client.managers.Channels.In(o.CommunityID).Fetch(ctx, o.ID)
	if err != nil {
		return err
	}
	if value.Partial {
		return ErrIncompleteObject
	}
	*o = *value
	return nil
}

func (o *CommunityMember) Fetch(ctx context.Context) error {
	if o == nil {
		return ErrObjectClientUnavailable
	}
	if err := requireObjectClient(o.client); err != nil {
		return err
	}
	value, err := o.client.managers.Members.In(o.CommunityID).Fetch(ctx, o.ID)
	if err != nil {
		return err
	}
	if value.Partial {
		return ErrIncompleteObject
	}
	*o = *value
	return nil
}

func (o *CommunityRole) Fetch(ctx context.Context) error {
	if o == nil {
		return ErrObjectClientUnavailable
	}
	if err := requireObjectClient(o.client); err != nil {
		return err
	}
	value, err := o.client.managers.Roles.In(o.CommunityID).Fetch(ctx, o.ID)
	if err != nil {
		return err
	}
	if value.Partial {
		return ErrIncompleteObject
	}
	*o = *value
	return nil
}

func (o *Message) Fetch(ctx context.Context) error {
	if o == nil {
		return ErrObjectClientUnavailable
	}
	if err := requireObjectClient(o.client); err != nil {
		return err
	}
	value, err := o.client.managers.Messages.In(o.Chat).Fetch(ctx, o.ID)
	if err != nil {
		return err
	}
	if value.Partial {
		return ErrIncompleteObject
	}
	*o = *value
	return nil
}

// Delete removes the membership without banning the user.
func (m *CommunityMember) Delete(ctx context.Context) error { return m.Kick(ctx, "") }

// SendText is the string convenience form of Send.
func (c *Channel) SendText(ctx context.Context, content string) (*Message, error) {
	return c.Send(ctx, MessageSendParams{Content: content})
}
func (m *CommunityMember) SendText(ctx context.Context, content string) (*Message, error) {
	return m.Send(ctx, MessageSendParams{Content: content})
}

// Community returns a partial reference without looking up the community list.
func (m *Message) Community() *Community {
	if m == nil || m.client == nil || m.Chat.CommunityID == 0 {
		return nil
	}
	if c, ok := m.client.managers.Communities.Get(m.Chat.CommunityID); ok {
		return c
	}
	return m.client.managers.Communities.Ref(m.Chat.CommunityID)
}

// Channel returns a cached channel or a partial reference without a network call.
func (m *Message) Channel() *Channel {
	if m == nil || m.client == nil || m.Chat.ChannelID == 0 {
		return nil
	}
	manager := m.client.managers.Channels.In(m.Chat.CommunityID)
	if c, ok := manager.Get(m.Chat.ChannelID); ok {
		return c
	}
	return manager.Ref(m.Chat.ChannelID)
}

// Member returns the author's cached membership or a partial reference.
// A message proves neither role assignments nor administrative permission.
func (m *Message) Member() *Member {
	if m == nil || m.client == nil || m.Chat.CommunityID == 0 || m.AuthorID == 0 {
		return nil
	}
	manager := m.client.managers.Members.In(m.Chat.CommunityID)
	if member, ok := manager.Get(m.AuthorID); ok {
		return member
	}
	member := manager.Ref(m.AuthorID)
	member.User = m.Author
	return member
}
