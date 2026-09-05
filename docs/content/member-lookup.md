---
title: Member lookup performance
description: Avoid bulk membership scans, understand cache costs, and measure real command latency.
group: Guides
order: 5
layout: doc
---

## What was found

The v0.2 SDK already supports `community.Members(ctx, userID)`, backed by the
protocol's targeted `GetMembers.member_ids`. That request does not need to scan
channels or fetch the community first. However, every completed call goes to the
server, and services created independent object clients with no shared cache.
Repeated commands cannot reuse earlier membership or gateway data through the SDK.

The reported custom-command path was also inspected in the local bot project. It
already has targeted lookup paths and a separate application member cache. Its
username fallback can perform a username lookup followed by a member request;
display-name fallback can load the whole community. Replacing only the SDK will
not remove those application fallbacks or automatically enable caching.

No fixed five-second wait was found in Osmose's member operations. The SDK's
outbound `RequestInterval` is opt-in and defaults to zero; the bot inspected did
not set it. The default single event worker can nevertheless queue commands behind
an earlier slow handler. Network latency, name resolution, server response time,
database work, and handler queueing need measurements from a real command run.

## Use the known ID

Enable `Config.Cache.Enabled`, then replace repeated membership RPCs with:

```go
community := client.Managers.Communities.Ref(communityID)
member, err := community.Collections().Members.Resolve(ctx, mentionedUserID)
if err != nil {
    return err
}
_, err = member.SendText(ctx, "Hello")
return err
```

The warm path performs no RPC. A cold miss sends one `getMembers` request containing
only the supplied ID. It never fetches channels, the community list, or all members.
Simultaneous identical lookups share an in-flight request. `FetchMany(ctx, ids...)`
can fetch multiple known IDs in one call. `Fetch(ctx, id)` always requests fresh data.

For the message author, `message.Member()` creates a bound membership reference
without I/O. Call its `Fetch(ctx)` before relying on roles if it is partial. For
critical permission checks, fetch fresh membership and roles even on a cache hit.

Do not treat a cache miss as "not a member." Do not treat a partial object or a
users-only response as complete membership. Use `errors.Is(err, types.ErrNotFound)`
after a fetch to handle absence. Avoid duplicating an SDK manager with an
application cache unless your application needs a separate retention policy.

## Reproduce the local comparison

```bash
go test -run '^$' -bench BenchmarkMemberLookup -benchmem ./types
```

The benchmark compares resolving one ID from cache, one targeted fresh lookup,
and bulk conversion/scanning of a 1,000-member response. It uses an in-process
fake RPC, so results measure SDK CPU/allocation overhead, **not network time**.

One Windows amd64 run on an Intel i5-10400 produced:

| Path | Approximate time/op | Bytes/op | RPCs/op |
| --- | ---: | ---: | ---: |
| Cached resolve | 1.0 µs | 416 | 0 |
| Fresh targeted fetch | 3.7 µs | 1,393 | 1 |
| List 1,000 members and scan | 471 µs | 202,508 | 1 |

These values vary by machine and do not predict the server's latency. No live
Osmium command was benchmarked with credentials, so a reduction from five seconds
to a specific real-world duration is not claimed.

## Measure the real command

Measure the time from event reception to handler entry separately from time spent
resolving parameters, fetching membership/roles, accessing the database, and sending
the reply. A warm `Resolve` should need zero RPCs; explicitly calling legacy
`Members`, `List`, or fresh `Fetch` still performs network I/O.

If queueing dominates, inspect slow handlers and only increase `EventWorkers` when
the bot's handlers and shared data are safe to run concurrently. If one targeted
RPC still takes seconds, the next investigation is server/network timing, not a
larger cache. Use [state management](../state-management/) for cache consistency,
limits, cancellation, and the protocol's role-update limitations.
