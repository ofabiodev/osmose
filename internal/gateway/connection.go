// Package gateway owns the single WebSocket reader and controlled writer used
// by an Osmium client.
package gateway

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

var ErrClosed = errors.New("gateway connection closed")

// Socket is the small part of a WebSocket connection needed by Connection.
// It also makes the gateway straightforward to test without a network.
type Socket interface {
	ReadMessage() (messageType int, data []byte, err error)
	WriteMessage(messageType int, data []byte) error
	SetReadLimit(limit int64)
	SetWriteDeadline(deadline time.Time) error
	Close() error
}

type Frame struct {
	Type int
	Data []byte
}

type Options struct {
	QueueSize    int
	WriteTimeout time.Duration
	ReadLimit    int64
	Logger       *slog.Logger
	OnMessage    func(messageType int, data []byte)
	OnError      func(error)
}

// Connection guarantees one reader goroutine and one writer goroutine.
type Connection struct {
	socket Socket
	queue  chan Frame
	ctx    context.Context
	cancel context.CancelFunc
	opts   Options
	errs   chan error

	closeOnce sync.Once
	errorOnce sync.Once
	startOnce sync.Once
	wg        sync.WaitGroup
}

func NewConnection(parent context.Context, socket Socket, opts Options) *Connection {
	if parent == nil {
		parent = context.Background()
	}
	if opts.QueueSize <= 0 {
		opts.QueueSize = 1024
	}
	if opts.WriteTimeout <= 0 {
		opts.WriteTimeout = 10 * time.Second
	}
	if opts.ReadLimit <= 0 {
		opts.ReadLimit = 16 << 20
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(parent)
	c := &Connection{socket: socket, queue: make(chan Frame, opts.QueueSize), ctx: ctx, cancel: cancel, opts: opts, errs: make(chan error, 1)}
	if socket != nil {
		socket.SetReadLimit(opts.ReadLimit)
	}
	return c
}

func (c *Connection) Start() {
	c.startOnce.Do(func() {
		if c.socket == nil {
			c.report(ErrClosed)
			return
		}
		c.wg.Add(2)
		go c.readerLoop()
		go c.writerLoop()
	})
}

func (c *Connection) Enqueue(ctx context.Context, frame Frame) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.ctx.Done():
		return ErrClosed
	case c.queue <- frame:
		return nil
	}
}

func (c *Connection) Errors() <-chan error { return c.errs }

func (c *Connection) Close() {
	c.closeOnce.Do(func() {
		c.cancel()
		if c.socket != nil {
			_ = c.socket.Close()
		}
	})
}

func (c *Connection) Wait() { c.wg.Wait() }

func (c *Connection) report(err error) {
	if err == nil {
		err = ErrClosed
	}
	c.errorOnce.Do(func() {
		select {
		case c.errs <- err:
		default:
		}
		if c.opts.OnError != nil {
			c.opts.OnError(err)
		}
		c.cancel()
		if c.socket != nil {
			_ = c.socket.Close()
		}
	})
}
