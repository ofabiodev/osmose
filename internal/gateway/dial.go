package gateway

import (
	"context"
	"time"

	"github.com/gorilla/websocket"
)

// Dialer uses gorilla/websocket because its one-reader/one-writer ownership
// model matches Osmium's controlled gateway loops and it is mature and small.
type Dialer struct {
	HandshakeTimeout time.Duration
}

func (d Dialer) Dial(ctx context.Context, endpoint string) (Socket, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if d.HandshakeTimeout <= 0 {
		d.HandshakeTimeout = 10 * time.Second
	}
	dialer := websocket.Dialer{HandshakeTimeout: d.HandshakeTimeout, EnableCompression: false}
	conn, _, err := dialer.DialContext(ctx, endpoint, nil)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
