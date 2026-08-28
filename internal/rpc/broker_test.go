package rpc

import (
	"errors"
	"sync"
	"testing"

	"github.com/ofabiodev/osmose/proto/core"
)

func TestBrokerCorrelatesConcurrentResults(t *testing.T) {
	b := NewBroker()
	const count = 128
	calls := make(map[uint32]*Call, count)
	for i := 0; i < count; i++ {
		id, call, err := b.Register()
		if err != nil {
			t.Fatal(err)
		}
		calls[id] = call
	}

	var wg sync.WaitGroup
	for id := range calls {
		id := id
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !b.Resolve(&core.RPCResult{ReqId: id}) {
				t.Errorf("result %d was not resolved", id)
			}
		}()
	}
	wg.Wait()
	for id, call := range calls {
		outcome := <-call.Done()
		if outcome.Err != nil || outcome.Result.GetReqId() != id {
			t.Fatalf("call %d got %#v", id, outcome)
		}
	}
	if b.Pending() != 0 {
		t.Fatalf("pending calls remain: %d", b.Pending())
	}
}

func TestBrokerFailAllAndIgnoreLateResults(t *testing.T) {
	b := NewBroker()
	id, call, err := b.Register()
	if err != nil {
		t.Fatal(err)
	}
	b.FailAll(ErrDisconnected)
	if outcome := <-call.Done(); !errors.Is(outcome.Err, ErrDisconnected) {
		t.Fatalf("unexpected error: %v", outcome.Err)
	}
	if b.Resolve(&core.RPCResult{ReqId: id}) {
		t.Fatal("late result was resolved")
	}
	if _, _, err := b.Register(); !errors.Is(err, ErrBrokerClosed) {
		t.Fatalf("unexpected register error: %v", err)
	}
}

func TestBrokerCancelChecksCallIdentity(t *testing.T) {
	b := NewBroker()
	id, call, err := b.Register()
	if err != nil {
		t.Fatal(err)
	}
	_, other, err := b.Register()
	if err != nil {
		t.Fatal(err)
	}
	if b.Cancel(id, other) {
		t.Fatal("cancelled a different call")
	}
	if !b.Cancel(id, call) || b.Pending() != 1 {
		t.Fatal("matching call was not cancelled")
	}
}

func BenchmarkBrokerResolve(b *testing.B) {
	broker := NewBroker()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id, call, err := broker.Register()
		if err != nil {
			b.Fatal(err)
		}
		if !broker.Resolve(&core.RPCResult{ReqId: id}) {
			b.Fatal("result was not resolved")
		}
		<-call.Done()
	}
}
