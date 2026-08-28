package gateway

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"
)

type fakeSocket struct {
	closed    chan struct{}
	closeOnce sync.Once

	mu        sync.Mutex
	writes    int
	active    int
	maxActive int
	delay     time.Duration
}

func newFakeSocket() *fakeSocket { return &fakeSocket{closed: make(chan struct{})} }

func (s *fakeSocket) ReadMessage() (int, []byte, error) {
	<-s.closed
	return 0, nil, io.EOF
}

func (s *fakeSocket) WriteMessage(messageType int, data []byte) error {
	select {
	case <-s.closed:
		return io.EOF
	default:
	}
	s.mu.Lock()
	s.writes++
	s.active++
	if s.active > s.maxActive {
		s.maxActive = s.active
	}
	s.mu.Unlock()
	time.Sleep(s.delay)
	s.mu.Lock()
	s.active--
	s.mu.Unlock()
	return nil
}

func (s *fakeSocket) SetReadLimit(int64)               {}
func (s *fakeSocket) SetWriteDeadline(time.Time) error { return nil }
func (s *fakeSocket) Close() error                     { s.closeOnce.Do(func() { close(s.closed) }); return nil }
func (s *fakeSocket) snapshot() (writes, maxActive int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writes, s.maxActive
}

func TestConnectionSerializesWrites(t *testing.T) {
	socket := newFakeSocket()
	socket.delay = time.Millisecond
	connection := NewConnection(context.Background(), socket, Options{QueueSize: 64, WriteTimeout: time.Second})
	connection.Start()

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := connection.Enqueue(context.Background(), Frame{Data: []byte("x")}); err != nil {
				t.Errorf("enqueue failed: %v", err)
			}
		}()
	}
	wg.Wait()
	deadline := time.Now().Add(time.Second)
	for {
		writes, _ := socket.snapshot()
		if writes == 32 || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	connection.Close()
	connection.Wait()
	writes, maxActive := socket.snapshot()
	if writes != 32 || maxActive != 1 {
		t.Fatalf("writes=%d maxActive=%d", writes, maxActive)
	}
}

func TestEnqueueHonorsCancellation(t *testing.T) {
	socket := newFakeSocket()
	connection := NewConnection(context.Background(), socket, Options{QueueSize: 1})
	if err := connection.Enqueue(context.Background(), Frame{}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := connection.Enqueue(ctx, Frame{}); err != context.Canceled {
		t.Fatalf("unexpected error: %v", err)
	}
	connection.Close()
}

func TestWaitBackoffCanBeCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := WaitBackoff(ctx, time.Hour); err != context.Canceled {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKeepaliveSendsProtocolMessage(t *testing.T) {
	socket := newFakeSocket()
	connection := NewConnection(context.Background(), socket, Options{QueueSize: 4, WriteTimeout: time.Second})
	connection.Start()
	ctx, cancel := context.WithCancel(context.Background())
	keepalive := NewKeepalive(ctx, time.Millisecond, connection.Enqueue, Frame{Data: []byte{8, 1}}, nil)
	keepalive.Start()

	deadline := time.Now().Add(time.Second)
	for {
		writes, _ := socket.snapshot()
		if writes != 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	keepalive.Stop()
	connection.Close()
	connection.Wait()
	writes, _ := socket.snapshot()
	if writes == 0 {
		t.Fatal("keepalive did not enqueue a protocol message")
	}
}

func TestBackoffIsBounded(t *testing.T) {
	backoff := NewBackoff(time.Millisecond, 4*time.Millisecond)
	for attempt := 0; attempt < 10; attempt++ {
		if delay := backoff.Delay(attempt); delay <= 0 || delay > 4*time.Millisecond {
			t.Fatalf("attempt %d returned %s", attempt, delay)
		}
	}
}
