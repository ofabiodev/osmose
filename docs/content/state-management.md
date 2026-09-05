---
title: Managers and state
description: Bounded client caches, direct member lookup, partial objects, and gateway synchronization.
group: Guides
order: 4
layout: doc
---

Osmose v0.3 adds six managers backed by one optional client-owned cache. Existing
services and rich objects use the same state. A lookup never needs a separate
community fetch just to construct a member or channel reference.

## Enable the cache

```go
client, err := osmose.New(osmose.Config{
    Token: token,
    ClientID: clientID,
    Cache: osmose.CacheConfig{
        Enabled: true,
        TTL: 5 * time.Minute,
        Members: 4096,
        Messages: 2048,
    },
})
```

Caching is disabled by default. With caching disabled, `Get` always misses and
`Resolve` fetches remotely. Managers, rich objects, and concurrent read coalescing
still work. There is no startup preload, background sweep, or automatic RPC retry.

Limits are per client, across all communities, with least-recently-used eviction:

| Entity | Default maximum entries |
| --- | ---: |
| Users | 2048 |
| Communities | 256 |
| Channels | 1024 |
| Members | 4096 |
| Roles | 1024 |
| Messages | 2048 |

Zero uses the default limit; `-1` disables that entity cache. TTL defaults to five
minutes and is measured from insertion/update, not from the last lookup. Expired
entries cannot be returned. Entry counts bound retention, not an exact byte size.

## Get, Resolve, and Fetch

```go
members := client.Managers.Members.In(communityID)

member, found := members.Get(userID)       // Cache only; no context or RPC.
member, err := members.Resolve(ctx, userID) // Complete cache hit, otherwise one RPC.
member, err = members.Fetch(ctx, userID)    // Always ask the server.
```

`Get` returns `(*Object, bool)`. `Fetch` and `Resolve` return `(*Object, error)`.
Check errors before using the returned object. An absent object returns
`types.ErrNotFound`. A user in a response's users sidecar is not proof of community
membership and never becomes a fabricated member with empty roles.

Concurrent identical reads in the same state revision share one RPC. Each waiter
can cancel its own wait; the first caller's context controls the underlying RPC,
so its cancellation/error is also returned to callers sharing that request. At
most 256 distinct reads are tracked; excess distinct reads proceed normally.
Completed calls are removed immediately, so later `Fetch` calls are fresh.

## Managers and collections

```go
community := client.Managers.Communities.Ref(communityID)
channel := community.Collections().Channels.Ref(channelID)

member, err := community.Collections().Members.Resolve(ctx, userID)
role, err := community.Collections().Roles.Fetch(ctx, roleID)
message, err := channel.Collections().Messages.Fetch(ctx, messageID)
```

`Ref(id)` constructs a client-bound partial object without any I/O. Scoping with
`In` or `Collections` also performs no I/O. Members and roles are keyed by community
and ID; messages are keyed by chat and ID. Entries cannot leak between scopes.

| Manager | Access | Operations |
| --- | --- | --- |
| `UserManager` | `client.Managers.Users` | `Get`, `Resolve`, `Fetch`, `Lookup(ctx, username)`, `List()` (cached users only) |
| `CommunityManager` | `client.Managers.Communities` | `Get`, `Resolve`, `Fetch`, `List(ctx)`, `Create(ctx, name)`, `Edit(ctx, id, options)`, `Delete(ctx, id)` |
| `ChannelManager` | `client.Managers.Channels.In(communityID)` | `Get`, `Resolve`, `Fetch`, `List(ctx)`, `Create(ctx, options)`, `Edit(ctx, id, options)`, `Delete(ctx, id)` |
| `MemberManager` | `client.Managers.Members.In(communityID)` | `Get`, `Resolve`, `Fetch`, `FetchMany(ctx, ids...)`, `List(ctx)`, `Create(ctx, userID)`, `Edit(ctx, id, options)`, `Delete(ctx, id)` |
| `RoleManager` | `client.Managers.Roles.In(communityID)` | `Get`, `Resolve`, `Fetch`, `List(ctx)`, `Create(ctx, options)`, `Edit(ctx, id, options)`, `Delete(ctx, id)` |
| `MessageManager` | `client.Managers.Messages.In(chatRef)` | `Get`, `Resolve`, `Fetch`, `List(ctx, historyParams...)`, `Create(ctx, content)`, `CreateWith(ctx, params)`, `Edit(ctx, id, content)`, `Delete(ctx, id)` |

Every manager also has `Ref`, `ListCached`, `Invalidate(id)`, and `Clear()`.
`ListCached` returns only retained entries, sorted by ID. It never means "all
members on the server." Root channel/member/role/message managers can inspect
their entire cached collection; network operations require an explicit scope.

`MemberManager.Create` adds an existing user; `Delete` kicks them. Community,
channel, role, and membership creation return `error`, because the protocol returns
confirmation without the created object. Obtain authoritative data from updates
or a subsequent `List`. Message creation returns a partial `*Message` with its ID.

## Protocol boundaries

The pinned [Osmium protocol](https://github.com/osmiumchat/proto/tree/93e452f64269aabb96283892d3ec759b2755afa9)
defines which operations can be made efficiently:

| Operation | Actual request |
| --- | --- |
| `Members.Fetch(ctx, id)` | One `communities.getMembers` with only that ID. No list scan. |
| `Members.FetchMany(ctx, ids...)` | One request for the supplied IDs, with duplicates removed. Empty input is rejected. |
| `Members.List(ctx)` | Explicit bulk `communities.getMembers`. Users-only responses do not establish complete membership. |
| `Users.Fetch(ctx, id)` | `chats.getChat` for the user's chat; the response must contain that user. `getProfile` only supplies profile metadata. |
| `Channels.Fetch(ctx, id)` | `chats.getChat` for the scoped channel. |
| `Communities.Fetch(ctx, id)` | One `communities.getCommunities`, then select the ID; no single-community endpoint exists. |
| `Roles.Fetch(ctx, id)` | One `communities.getRoles`, then select the ID. |
| `Messages.Fetch(ctx, id)` | One bounded history request around the ID, selecting an exact chat/ID match. No dedicated get-message endpoint exists. |

The bot protocol does not expose creation, editing, deletion, or global listing of
arbitrary user accounts. These operations are intentionally absent from UserManager.

## Object lifecycle and partial data

```go
channel := client.Managers.Channels.In(communityID).Ref(channelID)
sent, err := channel.SendText(ctx, "Hello") // IDs suffice; no channel fetch needed.
if err != nil {
    return err
}
if err := sent.Fetch(ctx); err != nil {    // Refreshes this snapshot in place.
    return err
}
return sent.Pin(ctx)
```

`User`, `Community`, `Channel`, `Message`, `Member`, and `Role` expose `Partial` and
`Fetch(ctx) error`. Fetch replaces the receiver only on success. `Edit` and `Delete`
retain their existing signatures; `Member.Delete(ctx)` is an alias for kicking.
Deletion evicts the cache entry; an already-held snapshot is not silently erased.

`message.Community()`, `message.Channel()`, and `message.Member()` return a cached
snapshot or a partial reference without I/O. `Member()` refers to the author. A
message does not prove the author's roles or administrator permissions.

Partial member role additions/removals and partial role edits/permission changes
fetch the missing state first. These read-modify-write operations must not erase
fields merely because an event omitted them. `SetRoles()` with no IDs removes all
roles. Fetch failures leave the receiver unchanged.

Managers are safe for concurrent use and return isolated snapshots, including
their `Raw` data. Gateway updates replace internal snapshots; they do not write
into public struct fields while application code is reading them. Call `Get` for
the latest cached snapshot, or `object.Fetch(ctx)` to refresh an existing one.
If your application edits the same object in multiple goroutines, it must
synchronize those accesses. The protocol has no atomic role add/remove endpoint;
concurrent remote role replacements can still conflict.

## Gateway synchronization

Updates enter state before entering the bounded handler queue. This happens even
when no handler is registered, or when handler delivery is dropped due to overflow.
No network calls or user handlers run while updating state. With multiple event
workers, a manager may already reflect an event newer than the one being handled.

| Update | State change |
| --- | --- |
| Message create/update/delete | Store a full snapshot, ingest an included author, or evict matching chat/IDs. |
| Channel update/delete | Replace the channel, or evict it and its cached messages. |
| Community update | Replace the community and invalidate its cached roles. |
| Community delete/unavailable | Evict the community, channels, members, roles, and messages in its scope. |
| Member create/update/delete | Replace membership, retain ID-only joins as partials, or evict the member. |
| User update/status/batch | Replace user data or update presence without discarding identity. |
| Member list/chat/group | Ingest the users and other entities actually supplied; never invent membership roles. |

ID-only user/community/member updates invalidate previously cached full data when
the payload supplies no replacement. Typed handlers receive partial references
where the IDs are available. Full data is not overwritten by a partial join or
send confirmation. Related `User`/`Author` values are resolved from shared state
when reading a member or message.

There is **no role update/delete gateway event** in this protocol revision. Local
role operations update/evict the cache, role fetches replace that community's role
list, community updates invalidate roles, and TTL bounds retention. Role changes
made by another client are not guaranteed to appear immediately. Use fresh member
and role fetches for authorization decisions that require current permissions.

Successful service/object mutations update known state or invalidate entries that
need an authoritative response (such as edited messages/channels). Failed RPCs
leave cached data intact. Requests made through `Raw().Call` participate for the
wrapped entity operations; after unsupported raw mutations, invalidate affected
state explicitly.

Slow reads cannot repopulate the cache after a newer event, mutation, or manual
invalidation. Such calls still return their response to the requesting caller;
only their cache write is discarded. A client-wide revision conservatively skips
unrelated cache fills during concurrent updates. Reconnect/shutdown clears cached
state because Osmium has no replay contract for missed updates. Old references
remain usable, but fetch new permission data after reconnect.

```go
community.Collections().Members.Invalidate(userID) // Cache eviction only.
community.Collections().Roles.Clear()             // This community's roles.
client.Managers.Clear()                            // All cached entities.
```

## Compatibility with v0.1 and v0.2

Existing service calls and `OnMessageCreate(func(ctx, *MessageCreateEvent) error)`
remain supported. `OnMessage` and `OnMessageEdit` are additive callbacks receiving
`*types.Message` directly. Context, error hooks, panic recovery, and unsubscribe
semantics are unchanged. `OnUpdate` remains the raw event escape hatch.

Go cannot have a `Members` field and a `Members` method on the same type. Therefore
the collection API is `community.Collections().Members`, preserving
`community.Members(ctx, ids...)`. Likewise, `channel.Collections().Messages`
preserves `channel.Messages(ctx, params)`. `Send(ctx, MessageSendParams)` stays
typed; `SendText(ctx, string)` is the new shorthand.

The v0.2 rename from `Message.Reply` metadata to `Message.ReplyInfo` still applies
when migrating directly from v0.1. v0.3 does not introduce another field rename.

See [member lookup performance](../member-lookup/) for a migration example and
the reproducible benchmark, and [events](../events/) for the rich callback API.
