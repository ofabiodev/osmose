---
title: Protocol and raw API
description: Understand the Osmium transport boundary and use generated requests.
group: Guides
order: 3
layout: doc
---

Osmose uses Osmium's current RPC-over-WebSocket protocol without adding a REST
layer.

## Connection flow

```text
Connect
  → Initialize
  → Initialized
  → Authorize
  → Authorization
  → Ready
```

After connection, binary protobuf frames contain either an `RPCResult` or an
`Update`:

```text
WebSocket
  → binary protobuf ServerMessage
  → RPCResult.req_id → broker → waiting call
  → Update → typed event dispatcher
```

Osmose manages the WebSocket reader, writes, keepalive, and request lifecycle
for you. Pending calls are completed with an error when a connection dies, and
reconnect starts the handshake again.

## Error classification

Transport failures and connection drops are retried with bounded backoff.
Only the currently known rejected-authorization code is permanent; other
authorization RPC errors remain retryable. Responses that violate the
expected handshake shape are returned as permanent errors. Check them with
`errors.Is(err, osmose.ErrPermanent)` or the more specific
`ErrAuthorizationFailed` and `ErrProtocolMismatch` values.

## Raw requests

High-level services are preferred for normal bot code. Advanced users can use
the generated protocol packages directly:

```go
import protoCommunities "github.com/ofabiodev/osmose/proto/communities"

result, err := client.Raw().Call(ctx, &protoCommunities.GetCommunities{})
if err != nil {
	return err
}

communities := result.GetCommunities()
```

The request wrapper is generated from the `ClientMessage` protobuf oneof. No
reflection, filesystem scan, or dynamic loading occurs during dispatch.
