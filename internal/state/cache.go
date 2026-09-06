// Package state owns bounded, immutable protocol snapshots. It never performs I/O.
package state

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
)

type Kind uint8

const (
	User Kind = iota
	Community
	Channel
	Member
	Role
	Message
	Kinds
)

// Config enables caching explicitly. Limits apply across the entire client,
// not per community. Zero limits use defaults; -1 disables an entity cache.
type Config struct {
	Enabled                                                bool
	TTL                                                    time.Duration
	Users, Communities, Channels, Members, Roles, Messages int
}

func (c Config) Validate() error {
	if c.TTL < 0 {
		return fmt.Errorf("cache TTL must not be negative")
	}
	for _, n := range c.limits() {
		if n < -1 || n > 1_000_000 {
			return fmt.Errorf("cache limits must be between -1 and 1000000")
		}
	}
	return nil
}

func (c Config) limits() [Kinds]int {
	return [Kinds]int{c.Users, c.Communities, c.Channels, c.Members, c.Roles, c.Messages}
}

type Key struct {
	Kind              Kind
	Scope, Parent, ID uint64
	Chat              uint8
}

type Entry struct {
	Key     Key
	Value   proto.Message
	Partial bool
	expires time.Time
}

type Cache struct {
	mu     sync.Mutex
	items  map[Key]*list.Element
	lru    [Kinds]list.List
	limits [Kinds]int
	ttl    time.Duration
	// A revision prevents a slow RPC response from overwriting newer gateway
	// state (including deletions), without an unbounded map of tombstones.
	revision uint64
	now      func() time.Time
}

func New(config Config) *Cache {
	c := &Cache{items: make(map[Key]*list.Element), ttl: config.TTL, now: time.Now}
	if c.ttl == 0 {
		c.ttl = 5 * time.Minute
	}
	if config.Enabled {
		c.limits = config.limits()
		defaults := [Kinds]int{2048, 256, 1024, 4096, 1024, 2048}
		for i, n := range c.limits {
			if n == 0 {
				c.limits[i] = defaults[i]
			}
		}
	}
	return c
}

func (c *Cache) Revision() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.revision
}

// Change applies an event/mutation atomically. Edit functions must not do I/O.
func (c *Cache) Change(edit func(*Writer)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.revision++
	edit(&Writer{c})
}

// Accept applies a read only if no event, mutation or invalidation raced it.
// ponytail: one client-wide revision may skip unrelated cache fills during busy
// traffic; use scoped revisions only if benchmarks justify the extra bookkeeping.
func (c *Cache) Accept(revision uint64, edit func(*Writer)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.revision == revision {
		edit(&Writer{c})
	}
}

// Mutate fences earlier reads. If an event raced the request, evict the affected
// entities instead of overwriting newer server state with an older local edit.
func (c *Cache) Mutate(revision uint64, edit, invalidate func(*Writer)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	current := c.revision
	c.revision++
	if current == revision {
		edit(&Writer{c})
	} else {
		invalidate(&Writer{c})
	}
}

type Writer struct{ c *Cache }

func (w *Writer) Put(key Key, value proto.Message, partial bool) {
	if key.ID == 0 || key.Kind >= Kinds || w.c.limits[key.Kind] <= 0 || value == nil {
		return
	}
	if old := w.c.items[key]; old != nil {
		entry := old.Value.(Entry)
		if partial && !entry.Partial && w.c.now().Before(entry.expires) {
			return
		}
		w.remove(old)
	}
	entry := Entry{Key: key, Value: proto.Clone(value), Partial: partial, expires: w.c.now().Add(w.c.ttl)}
	w.c.items[key] = w.c.lru[key.Kind].PushFront(entry)
	if w.c.lru[key.Kind].Len() > w.c.limits[key.Kind] {
		w.remove(w.c.lru[key.Kind].Back())
	}
}

// Read is only for transforms inside Change; callers must clone before editing.
func (w *Writer) Read(key Key) (Entry, bool) {
	element := w.c.items[key]
	if element == nil {
		return Entry{}, false
	}
	entry := element.Value.(Entry)
	if !w.c.now().Before(entry.expires) {
		w.remove(element)
		return Entry{}, false
	}
	return entry, true
}

func (w *Writer) Delete(key Key) {
	if element := w.c.items[key]; element != nil {
		w.remove(element)
	}
}

func (w *Writer) DeleteWhere(match func(Key) bool) {
	for key, element := range w.c.items {
		if match(key) {
			w.remove(element)
		}
	}
}

func (w *Writer) remove(element *list.Element) {
	entry := element.Value.(Entry)
	delete(w.c.items, entry.Key)
	w.c.lru[entry.Key.Kind].Remove(element)
}

func (c *Cache) Get(key Key) (Entry, bool) {
	c.mu.Lock()
	entry, ok := (&Writer{c}).Read(key)
	if ok {
		c.lru[key.Kind].MoveToFront(c.items[key])
	}
	c.mu.Unlock()
	if ok {
		entry.Value = proto.Clone(entry.Value)
	}
	return entry, ok
}

func (c *Cache) List(match func(Key) bool) []Entry {
	c.mu.Lock()
	entries := make([]Entry, 0)
	w := &Writer{c}
	for key := range c.items {
		if match(key) {
			if entry, ok := w.Read(key); ok {
				entries = append(entries, entry)
			}
		}
	}
	c.mu.Unlock()
	for i := range entries {
		entries[i].Value = proto.Clone(entries[i].Value)
	}
	return entries
}

func (c *Cache) Invalidate(match func(Key) bool) {
	c.Change(func(w *Writer) { w.DeleteWhere(match) })
}

func (c *Cache) Clear() { c.Invalidate(func(Key) bool { return true }) }
