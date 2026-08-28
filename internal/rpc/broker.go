package rpc

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/ofabiodev/osmose/proto/core"
)

var (
	ErrDisconnected   = errors.New("Osmium connection disconnected")
	ErrDuplicateReqID = errors.New("duplicate RPC request id")
	ErrBrokerClosed   = errors.New("RPC broker closed")
)

type Outcome struct {
	Result *core.RPCResult
	Err    error
}

type Call struct{ done chan Outcome }

func (c *Call) Done() <-chan Outcome { return c.done }

// Broker correlates RPCResult.req_id values with waiting calls. It is scoped
// to one gateway connection; a dead connection closes its broker.
type Broker struct {
	nextID atomic.Uint32

	mu      sync.Mutex
	pending map[uint32]*Call
	closed  bool
}

func NewBroker() *Broker { return &Broker{pending: make(map[uint32]*Call)} }

func (b *Broker) Register() (uint32, *Call, error) {
	for {
		id := b.nextID.Add(1)
		if id == 0 {
			continue
		}
		b.mu.Lock()
		if b.closed {
			b.mu.Unlock()
			return 0, nil, ErrBrokerClosed
		}
		if _, exists := b.pending[id]; exists {
			b.mu.Unlock()
			continue
		}
		call := &Call{done: make(chan Outcome, 1)}
		b.pending[id] = call
		b.mu.Unlock()
		return id, call, nil
	}
}

func (b *Broker) Resolve(result *core.RPCResult) bool {
	if result == nil {
		return false
	}
	b.mu.Lock()
	call, ok := b.pending[result.GetReqId()]
	if ok {
		delete(b.pending, result.GetReqId())
	}
	b.mu.Unlock()
	if !ok {
		return false
	}
	call.done <- Outcome{Result: result}
	return true
}

func (b *Broker) Cancel(id uint32, call *Call) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	current, ok := b.pending[id]
	if !ok || (call != nil && current != call) {
		return false
	}
	delete(b.pending, id)
	return true
}

func (b *Broker) FailAll(err error) {
	if err == nil {
		err = ErrDisconnected
	}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	calls := make([]*Call, 0, len(b.pending))
	for id, call := range b.pending {
		delete(b.pending, id)
		calls = append(calls, call)
	}
	b.mu.Unlock()
	for _, call := range calls {
		call.done <- Outcome{Err: err}
	}
}

func (b *Broker) Pending() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.pending)
}
