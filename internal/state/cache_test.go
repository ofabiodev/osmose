package state

import (
	"testing"
	"time"

	protoTypes "github.com/ofabiodev/osmose/proto/types"
)

func TestCacheBoundsExpiryIsolationAndRevision(t *testing.T) {
	c := New(Config{Enabled: true, Users: 2, Members: -1, TTL: time.Minute})
	now := time.Unix(100, 0)
	c.now = func() time.Time { return now }
	put := func(id uint64) {
		c.Change(func(w *Writer) { w.Put(Key{Kind: User, ID: id}, &protoTypes.User{Id: id, Name: "original"}, false) })
	}
	put(1)
	put(2)
	entry, ok := c.Get(Key{Kind: User, ID: 1})
	if !ok {
		t.Fatal("cache miss")
	}
	entry.Value.(*protoTypes.User).Name = "caller mutation"
	put(3)
	if _, ok := c.Get(Key{Kind: User, ID: 2}); ok {
		t.Fatal("LRU did not evict user 2")
	}
	entry, _ = c.Get(Key{Kind: User, ID: 1})
	if entry.Value.(*protoTypes.User).Name != "original" {
		t.Fatal("caller changed cache")
	}
	revision := c.Revision()
	c.Clear()
	c.Accept(revision, func(w *Writer) { w.Put(Key{Kind: User, ID: 1}, &protoTypes.User{Id: 1}, false) })
	if _, ok := c.Get(Key{Kind: User, ID: 1}); ok {
		t.Fatal("stale response resurrected an invalidated entry")
	}
	put(1)
	now = now.Add(time.Minute)
	if _, ok := c.Get(Key{Kind: User, ID: 1}); ok {
		t.Fatal("entry survived TTL")
	}
	c.Change(func(w *Writer) {
		w.Put(Key{Kind: Member, Scope: 1, ID: 2}, &protoTypes.CommunityMember{Id: 2, CommunityId: 1}, false)
	})
	if len(c.List(func(Key) bool { return true })) != 0 {
		t.Fatal("disabled entity cache stored data")
	}
	for _, config := range []Config{{Users: -2}, {Messages: 1_000_001}, {TTL: -1}} {
		if config.Validate() == nil {
			t.Fatalf("invalid config accepted: %+v", config)
		}
	}
}

func TestPartialDoesNotReplaceCompleteSnapshot(t *testing.T) {
	c := New(Config{Enabled: true})
	key := Key{Kind: Member, Scope: 10, ID: 20}
	c.Change(func(w *Writer) {
		w.Put(key, &protoTypes.CommunityMember{Id: 20, CommunityId: 10, RoleIds: []uint64{30}}, false)
	})
	c.Change(func(w *Writer) { w.Put(key, &protoTypes.CommunityMember{Id: 20, CommunityId: 10}, true) })
	entry, ok := c.Get(key)
	if !ok || entry.Partial || len(entry.Value.(*protoTypes.CommunityMember).GetRoleIds()) != 1 {
		t.Fatal("partial snapshot discarded known roles")
	}
}
