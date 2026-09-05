package types

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	protoChats "github.com/ofabiodev/osmose/proto/chats"
	protoCommunities "github.com/ofabiodev/osmose/proto/communities"
	"github.com/ofabiodev/osmose/proto/core"
	protoMessages "github.com/ofabiodev/osmose/proto/messages"
	protoTypes "github.com/ofabiodev/osmose/proto/types"
	"github.com/ofabiodev/osmose/proto/updates"
	"google.golang.org/protobuf/proto"
)

func memberResult(scope, id ID) *core.RPCResult {
	username := "member"
	return &core.RPCResult{Result: &core.RPCResult_Members{Members: &protoCommunities.Members{
		Members: []*protoTypes.CommunityMember{{Id: uint64(id), CommunityId: uint64(scope), RoleIds: []uint64{40}}},
		Users:   []*protoTypes.User{{Id: uint64(id), Name: "Member", Username: &username}},
	}}}
}

func memberUpdate(scope, id ID, nickname string) *updates.Update {
	return &updates.Update{Update: &updates.Update_CommunityMember{CommunityMember: &updates.UpdateCommunityMember{
		CommunityId: uint64(scope), MemberId: uint64(id), Member: &protoTypes.CommunityMember{Id: uint64(id), CommunityId: uint64(scope), Nickname: &nickname, RoleIds: []uint64{40}},
	}}}
}

func TestMemberResolveUsesOneTargetedRPCAndFreshFetchBypassesCache(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			calls := 0
			c := NewObjectClient(func(ctx context.Context, request proto.Message) (*core.RPCResult, error) {
				calls++
				r, ok := request.(*protoCommunities.GetMembers)
				if !ok || r.GetCommunityId() != 10 || len(r.GetMemberIds()) != 1 || r.GetMemberIds()[0] != 20 {
					t.Fatalf("not a targeted lookup: %v", request)
				}
				return memberResult(10, 20), nil
			}, CacheConfig{Enabled: enabled})
			m := c.Managers().Communities.Ref(10).Collections().Members
			if _, ok := m.Get(20); ok || calls != 0 {
				t.Fatal("Get performed I/O")
			}
			for i := 0; i < 3; i++ {
				member, err := m.Resolve(context.Background(), 20)
				if err != nil || member.User == nil || member.Partial {
					t.Fatalf("member=%+v err=%v", member, err)
				}
				member.RoleIDs[0] = 999
				member.Raw.RoleIds[0] = 999
				member.User.Name = "changed"
			}
			want := 3
			if enabled {
				want = 1
			}
			if calls != want {
				t.Fatalf("calls=%d want=%d", calls, want)
			}
			fresh, err := m.Fetch(context.Background(), 20)
			if err != nil || fresh.RoleIDs[0] != 40 || fresh.User.Name != "Member" || calls != want+1 {
				t.Fatalf("fresh=%+v calls=%d err=%v", fresh, calls, err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := m.Resolve(ctx, 20); !errors.Is(err, context.Canceled) {
				t.Fatal("Resolve ignored cancellation")
			}
			m.Invalidate(20)
			if _, ok := m.Get(20); ok {
				t.Fatal("Invalidate did not evict")
			}
		})
	}
}

func TestConcurrentFetchCoalescesAndWaitersCanCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls atomic.Int32
		release := make(chan struct{})
		c := NewObjectClient(func(context.Context, proto.Message) (*core.RPCResult, error) {
			calls.Add(1)
			<-release
			return memberResult(10, 20), nil
		}, CacheConfig{Enabled: true})
		m := c.Managers().Members.In(10)
		var wg sync.WaitGroup
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				value, err := m.Fetch(context.Background(), 20)
				if err != nil || value.ID != 20 {
					t.Errorf("value=%v err=%v", value, err)
				}
			}()
		}
		synctest.Wait()
		ctx, cancel := context.WithCancel(context.Background())
		canceled := make(chan error, 1)
		go func() { _, err := m.Fetch(ctx, 20); canceled <- err }()
		synctest.Wait()
		cancel()
		synctest.Wait()
		if err := <-canceled; !errors.Is(err, context.Canceled) {
			t.Fatalf("waiter cancellation: %v", err)
		}
		if calls.Load() != 1 {
			t.Fatalf("issued %d simultaneous RPCs", calls.Load())
		}
		close(release)
		wg.Wait()
		if _, err := m.Fetch(context.Background(), 20); err != nil {
			t.Fatal(err)
		}
		if calls.Load() != 2 {
			t.Fatal("Fetch reused a completed call")
		}
	})
}

func TestSlowFetchCannotResurrectDeletedMember(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		c := NewObjectClient(func(context.Context, proto.Message) (*core.RPCResult, error) {
			<-release
			return memberResult(10, 20), nil
		}, CacheConfig{Enabled: true})
		m := c.Managers().Members.In(10)
		done := make(chan struct{})
		go func() { defer close(done); _, _ = m.Fetch(context.Background(), 20) }()
		synctest.Wait()
		c.ApplyUpdate(&updates.Update{Update: &updates.Update_CommunityMemberDeleted{CommunityMemberDeleted: &updates.UpdateCommunityMemberDeleted{CommunityId: 10, MemberId: 20}}})
		close(release)
		<-done
		if _, ok := m.Get(20); ok {
			t.Fatal("old RPC resurrected a deleted member")
		}
	})
}

func TestStateScopesEventsPartialAndHydration(t *testing.T) {
	var calls int
	c := NewObjectClient(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
		calls++
		if _, ok := request.(*protoCommunities.GetMembers); ok {
			return memberResult(10, 20), nil
		}
		return voidResult(), nil
	}, CacheConfig{Enabled: true})
	a, b := c.Managers().Members.In(10), c.Managers().Members.In(11)
	c.ApplyUpdate(memberUpdate(10, 20, "first"))
	c.ApplyUpdate(memberUpdate(11, 20, "other"))
	snapshot, _ := a.Get(20)
	c.ApplyUpdate(memberUpdate(10, 20, "updated"))
	current, _ := a.Get(20)
	other, _ := b.Get(20)
	if *snapshot.Nickname != "first" || *current.Nickname != "updated" || *other.Nickname != "other" {
		t.Fatal("snapshot or community scope changed")
	}
	a.Clear()
	c.ApplyUpdate(&updates.Update{Update: &updates.Update_CommunityMemberCreated{CommunityMemberCreated: &updates.UpdateCommunityMemberCreated{CommunityId: 10, MemberId: 20}}})
	partial, ok := a.Get(20)
	if !ok || !partial.Partial {
		t.Fatal("missing partial member")
	}
	if err := partial.AddRole(context.Background(), 50); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || partial.Partial || len(partial.RoleIDs) != 2 || partial.RoleIDs[0] != 40 {
		t.Fatal("partial role mutation did not hydrate first")
	}
	if err := partial.SetRoles(context.Background()); err != nil {
		t.Fatal(err)
	}
	cleared, _ := a.Get(20)
	if len(cleared.RoleIDs) != 0 {
		t.Fatal("empty SetRoles did not remove all roles")
	}
	c.ApplyUpdate(&updates.Update{Update: &updates.Update_CommunityDeleted{CommunityDeleted: &updates.UpdateCommunityDeleted{CommunityId: 10}}})
	if _, ok := a.Get(20); ok {
		t.Fatal("community deletion left members")
	}
	if _, ok := b.Get(20); !ok {
		t.Fatal("deleted unrelated community member")
	}
}

func TestCoreManagersFetchAndObjectLifecycle(t *testing.T) {
	chat := ChannelChat(10, 30)
	ref, _ := chat.ToProto()
	c := NewObjectClient(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
		switch r := request.(type) {
		case *protoCommunities.GetCommunities:
			return &core.RPCResult{Result: &core.RPCResult_Communities{Communities: &protoCommunities.Communities{Communities: []*protoTypes.Community{{Id: 10, Name: "Community"}}}}}, nil
		case *protoChats.GetChat:
			return &core.RPCResult{Result: &core.RPCResult_Chat{Chat: &protoChats.Chat{Channel: &protoTypes.Channel{Id: 30, CommunityId: 10, Name: "general"}, Users: []*protoTypes.User{{Id: 20, Name: "User"}}}}}, nil
		case *protoCommunities.GetMembers:
			return memberResult(10, 20), nil
		case *protoCommunities.GetRoles:
			return &core.RPCResult{Result: &core.RPCResult_CommunityRoles{CommunityRoles: &protoCommunities.CommunityRoles{Roles: []*protoTypes.CommunityRole{{Id: 40, CommunityId: 10, Name: "Role", Permissions: 8}}}}}, nil
		case *protoMessages.GetHistory:
			if r.GetAround() != 50 || r.GetLimit() > 3 {
				t.Fatal("Fetch scanned message history")
			}
			return &core.RPCResult{Result: &core.RPCResult_Messages{Messages: &protoMessages.Messages{Messages: []*protoTypes.Message{{MessageId: 50, ChatRef: ref, AuthorId: 20, Message: "hello"}}}}}, nil
		case *protoCommunities.RemoveMembers:
			return &core.RPCResult{Result: &core.RPCResult_RemovedMembers{RemovedMembers: &protoCommunities.RemovedMembers{Members: []*protoCommunities.RemovedMembers_RemovedMember{{UserId: 20}}}}}, nil
		default:
			return voidResult(), nil
		}
	}, CacheConfig{Enabled: true})
	ctx := context.Background()
	u := c.Managers().Users.Ref(20)
	community := c.Managers().Communities.Ref(10)
	channel := community.Collections().Channels.Ref(30)
	member := community.Collections().Members.Ref(20)
	role := community.Collections().Roles.Ref(40)
	message := channel.Collections().Messages.Ref(50)
	for _, object := range []interface{ Fetch(context.Context) error }{community, u, channel, member, role, message} {
		if err := object.Fetch(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if community.Name != "Community" || u.Name != "User" || channel.Name != "general" || role.Name != "Role" || message.Content != "hello" || message.Author == nil {
		t.Fatal("objects not hydrated")
	}
	if message.Channel() == nil || message.Community() == nil || message.Member() == nil {
		t.Fatal("related objects missing")
	}
	if err := role.AddPermissions(ctx, 16); err != nil {
		t.Fatal(err)
	}
	got, _ := community.Collections().Roles.Get(40)
	if got.Permissions != 24 {
		t.Fatal("role cache missed local edit")
	}
	if err := member.Kick(ctx, ""); err != nil {
		t.Fatalf("valid RemovedMembers rejected: %v", err)
	}
	if _, ok := community.Collections().Members.Get(20); ok {
		t.Fatal("kick left stale membership")
	}
	if err := message.Delete(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok := channel.Collections().Messages.Get(50); ok {
		t.Fatal("delete left stale message")
	}
}

func TestMissingMemberFailsClosedAndFailedMutationPreservesCache(t *testing.T) {
	reject := errors.New("permission denied")
	c := NewObjectClient(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
		if _, ok := request.(*protoCommunities.GetMembers); ok {
			return &core.RPCResult{Result: &core.RPCResult_Members{Members: &protoCommunities.Members{Users: []*protoTypes.User{{Id: 20}}}}}, nil
		}
		return nil, reject
	}, CacheConfig{Enabled: true})
	m := c.Managers().Members.In(10)
	if _, err := m.Fetch(context.Background(), 20); !errors.Is(err, ErrNotFound) {
		t.Fatalf("users sidecar treated as membership: %v", err)
	}
	c.ApplyUpdate(memberUpdate(10, 20, "before"))
	member, _ := m.Get(20)
	if err := member.Kick(context.Background(), ""); !errors.Is(err, reject) {
		t.Fatal(err)
	}
	if _, ok := m.Get(20); !ok {
		t.Fatal("failed mutation evicted good state")
	}
	for _, run := range []func() error{
		func() error { _, e := m.Fetch(context.Background(), 0); return e },
		func() error { _, e := c.Managers().Members.Fetch(context.Background(), 20); return e },
		func() error { _, e := c.Managers().Messages.Fetch(context.Background(), 50); return e },
	} {
		if err := run(); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
}

func TestUserUpdatesAndRoleInvalidation(t *testing.T) {
	c := NewObjectClient(func(context.Context, proto.Message) (*core.RPCResult, error) {
		return &core.RPCResult{Result: &core.RPCResult_CommunityRoles{CommunityRoles: &protoCommunities.CommunityRoles{Roles: []*protoTypes.CommunityRole{{Id: 40, CommunityId: 10, Name: "admin", Permissions: 8}}}}}, nil
	}, CacheConfig{Enabled: true})
	ctx := context.Background()
	if _, err := c.Managers().Roles.In(10).Fetch(ctx, 40); err != nil {
		t.Fatal(err)
	}
	c.ApplyUpdate(&updates.Update{Update: &updates.Update_User{User: &updates.UpdateUser{UserId: 20, User: &protoTypes.User{Id: 20, Name: "before"}}}})
	before, _ := c.Managers().Users.Get(20)
	c.ApplyUpdate(&updates.Update{Update: &updates.Update_UserStatusBatch{UserStatusBatch: &updates.UpdateUserStatusBatch{Updates: []*updates.UpdateUserStatus{{UserId: 20, Status: &protoTypes.UserStatus{Status: protoTypes.UserStatus_ONLINE.Enum()}}}}}})
	after, _ := c.Managers().Users.Get(20)
	if after.Name != "before" || after.Status == nil || before.Status != nil {
		t.Fatal("presence update discarded identity or mutated a snapshot")
	}
	c.ApplyUpdate(&updates.Update{Update: &updates.Update_Community{Community: &updates.UpdateCommunity{CommunityId: 10, Community: &protoTypes.Community{Id: 10, Name: "changed"}}}})
	if _, ok := c.Managers().Roles.In(10).Get(40); ok {
		t.Fatal("community update retained potentially stale roles")
	}
	community, ok := c.Managers().Communities.Get(10)
	if !ok || community.Name != "changed" {
		t.Fatal("community update not applied")
	}
	c.ApplyUpdate(&updates.Update{Update: &updates.Update_User{User: &updates.UpdateUser{UserId: 20}}})
	if _, ok := c.Managers().Users.Get(20); ok {
		t.Fatal("ID-only update retained stale user")
	}
}

func TestConcurrentGatewayReadsAreIsolated(t *testing.T) {
	c := NewObjectClient(func(context.Context, proto.Message) (*core.RPCResult, error) {
		return nil, errors.New("no RPC expected")
	}, CacheConfig{Enabled: true})
	c.ApplyUpdate(memberUpdate(10, 20, "member"))
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 100; n++ {
				c.ApplyUpdate(memberUpdate(10, 20, "member"))
				m, ok := c.Managers().Members.In(10).Get(20)
				if !ok || m.RoleIDs[0] != 40 {
					t.Error("corrupted concurrent snapshot")
					return
				}
				m.RoleIDs[0] = 999
				m.Raw.RoleIds[0] = 999
			}
		}()
	}
	wg.Wait()
}

func TestMemberListUsersRemainBound(t *testing.T) {
	c := NewObjectClient(func(context.Context, proto.Message) (*core.RPCResult, error) {
		return &core.RPCResult{Result: &core.RPCResult_Chat{Chat: &protoChats.Chat{Users: []*protoTypes.User{{Id: 20, Name: "refreshed"}}}}}, nil
	})
	entry := MemberListEntryFromProto(&updates.MemberListEntry{Entry: &updates.MemberListEntry_User{User: &updates.MemberListEntryUser{User: &protoTypes.User{Id: 20}}}}, c)
	if err := entry.User.Fetch(context.Background()); err != nil || entry.User.Name != "refreshed" {
		t.Fatalf("unbound member-list user: %v", err)
	}
}

func BenchmarkMemberLookup(b *testing.B) {
	for _, mode := range []string{"cached-resolve", "fresh-fetch", "list-and-scan"} {
		b.Run(mode, func(b *testing.B) {
			var calls int
			bulk := &protoCommunities.Members{}
			for i := uint64(1); i <= 1000; i++ {
				bulk.Members = append(bulk.Members, &protoTypes.CommunityMember{Id: i, CommunityId: 10})
			}
			c := NewObjectClient(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
				calls++
				if len(request.(*protoCommunities.GetMembers).GetMemberIds()) == 0 {
					return &core.RPCResult{Result: &core.RPCResult_Members{Members: bulk}}, nil
				}
				return memberResult(10, 20), nil
			}, CacheConfig{Enabled: mode == "cached-resolve"})
			m := c.Managers().Members.In(10)
			if mode == "cached-resolve" {
				_, _ = m.Fetch(context.Background(), 20)
				calls = 0
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				switch mode {
				case "cached-resolve":
					_, _ = m.Resolve(context.Background(), 20)
				case "fresh-fetch":
					_, _ = m.Fetch(context.Background(), 20)
				case "list-and-scan":
					members, _ := m.List(context.Background())
					for _, member := range members {
						if member.ID == 20 {
							break
						}
					}
				}
			}
			b.ReportMetric(float64(calls)/float64(b.N), "RPC/op")
		})
	}
}
