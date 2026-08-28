package client

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ofabiodev/osmose/internal/gateway"
	"github.com/ofabiodev/osmose/proto/auth"
	"github.com/ofabiodev/osmose/proto/communities"
	"github.com/ofabiodev/osmose/proto/core"
	protoMessages "github.com/ofabiodev/osmose/proto/messages"
	protoTypes "github.com/ofabiodev/osmose/proto/types"
	"github.com/ofabiodev/osmose/proto/updates"
	modelTypes "github.com/ofabiodev/osmose/types"
	"google.golang.org/protobuf/proto"
)

type scriptedSocket struct {
	reads  chan []byte
	closed chan struct{}
	once   sync.Once

	mu            sync.Mutex
	writes        []*core.ClientMessage
	drop          bool
	rpcErr        bool
	authorizeErr  bool
	authorizeCode uint32
}

func newScriptedSocket() *scriptedSocket {
	return &scriptedSocket{reads: make(chan []byte, 128), closed: make(chan struct{})}
}

func (s *scriptedSocket) ReadMessage() (int, []byte, error) {
	select {
	case data := <-s.reads:
		return websocket.BinaryMessage, data, nil
	case <-s.closed:
		return 0, nil, io.EOF
	}
}

func (s *scriptedSocket) WriteMessage(messageType int, data []byte) error {
	if messageType != websocket.BinaryMessage {
		return errors.New("expected binary frame")
	}
	select {
	case <-s.closed:
		return io.EOF
	default:
	}
	request := &core.ClientMessage{}
	if err := proto.Unmarshal(data, request); err != nil {
		return err
	}
	s.mu.Lock()
	s.writes = append(s.writes, proto.Clone(request).(*core.ClientMessage))
	drop, rpcErr, authorizeErr := s.drop, s.rpcErr, s.authorizeErr
	s.mu.Unlock()
	if drop || request.GetMessage() == nil {
		return nil
	}
	var result *core.RPCResult
	if rpcErr || (authorizeErr && request.GetAuthAuthorize() != nil) {
		code := uint32(403)
		if s.authorizeCode != 0 {
			code = s.authorizeCode
		}
		result = &core.RPCResult{ReqId: request.GetId(), Result: &core.RPCResult_Error{Error: &core.RPCError{ErrorCode: code, ErrorMessage: "forbidden"}}}
	} else {
		switch request.GetMessage().(type) {
		case *core.ClientMessage_CoreInitialize:
			result = &core.RPCResult{ReqId: request.GetId(), Result: &core.RPCResult_Initialized{Initialized: &core.Initialized{}}}
		case *core.ClientMessage_AuthAuthorize:
			username := "test-bot"
			result = &core.RPCResult{ReqId: request.GetId(), Result: &core.RPCResult_Authorization{Authorization: &auth.Authorization{User: &protoTypes.User{Id: 77, Name: "Test Bot", Username: &username}, SessionId: 88}}}
		case *core.ClientMessage_MessagesSendMessage:
			result = &core.RPCResult{ReqId: request.GetId(), Result: &core.RPCResult_SentMessage{SentMessage: &protoMessages.SentMessage{MessageId: 2}}}
		case *core.ClientMessage_MessagesSetInteractionResponse:
			result = &core.RPCResult{ReqId: request.GetId()}
		case *core.ClientMessage_CommunitiesGetCommunities:
			result = &core.RPCResult{ReqId: request.GetId(), Result: &core.RPCResult_Communities{Communities: &communities.Communities{}}}
		}
	}
	if result == nil {
		return nil
	}
	frame, err := proto.Marshal(&core.ServerMessage{Message: &core.ServerMessage_Result{Result: result}})
	if err != nil {
		return err
	}
	select {
	case s.reads <- frame:
		return nil
	case <-s.closed:
		return io.EOF
	}
}

func (s *scriptedSocket) SetReadLimit(int64)               {}
func (s *scriptedSocket) SetWriteDeadline(time.Time) error { return nil }
func (s *scriptedSocket) Close() error                     { s.once.Do(func() { close(s.closed) }); return nil }

func (s *scriptedSocket) snapshot() []*core.ClientMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*core.ClientMessage(nil), s.writes...)
}

func (s *scriptedSocket) push(update *updates.Update) error {
	data, err := proto.Marshal(&core.ServerMessage{Message: &core.ServerMessage_Update{Update: update}})
	if err != nil {
		return err
	}
	s.reads <- data
	return nil
}

func testClient(t *testing.T, socket *scriptedSocket) *Client {
	t.Helper()
	client, err := New(Config{
		Token:             "token",
		ClientID:          120715,
		ServerURL:         "ws://localhost",
		RequestTimeout:    100 * time.Millisecond,
		HeartbeatInterval: time.Hour,
		StableConnection:  time.Hour,
		BackoffMin:        time.Millisecond,
		BackoffMax:        2 * time.Millisecond,
		dial: func(context.Context, string) (gateway.Socket, error) {
			return socket, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func startReadyClient(t *testing.T, client *Client) (context.CancelFunc, <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- client.Run(ctx) }()
	readyCtx, readyCancel := context.WithTimeout(ctx, time.Second)
	defer readyCancel()
	if err := client.WaitReady(readyCtx); err != nil {
		cancel()
		t.Fatalf("wait ready: %v", err)
	}
	return cancel, runErr
}

func stopTestClient(t *testing.T, client *Client, cancel context.CancelFunc, runErr <-chan error) {
	t.Helper()
	_ = client.Close()
	cancel()
	select {
	case err := <-runErr:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("client did not stop")
	}
}

func TestClientInitializesAuthorizesAndReplies(t *testing.T) {
	socket := newScriptedSocket()
	client := testClient(t, socket)
	cancel, runErr := startReadyClient(t, client)
	message := &modelTypes.Message{ID: 1, Chat: modelTypes.SelfChat()}
	chat, err := message.Chat.ToProto()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Messages.Reply(context.Background(), message, "pong"); err != nil {
		t.Fatal(err)
	}
	requests := socket.snapshot()
	if len(requests) < 3 {
		t.Fatalf("got %d requests", len(requests))
	}
	if requests[0].GetCoreInitialize() == nil || requests[1].GetAuthAuthorize() == nil || client.State() != Ready || client.User().ID != 77 || client.SessionID() != 88 {
		t.Fatal("handshake did not complete")
	}
	send := requests[len(requests)-1].GetMessagesSendMessage()
	if send == nil || !proto.Equal(send.GetChatRef(), chat) || send.GetReplyTo() == nil || send.GetReplyTo().GetMessageId() != 1 {
		t.Fatalf("unexpected raw request: %#v", send)
	}
	stopTestClient(t, client, cancel, runErr)
}

func TestClientUsesProtocolKeepalive(t *testing.T) {
	socket := newScriptedSocket()
	client, err := New(Config{
		Token:             "token",
		ClientID:          120715,
		ServerURL:         "ws://localhost",
		RequestTimeout:    100 * time.Millisecond,
		HeartbeatInterval: time.Millisecond,
		StableConnection:  time.Hour,
		BackoffMin:        time.Millisecond,
		BackoffMax:        2 * time.Millisecond,
		dial: func(context.Context, string) (gateway.Socket, error) {
			return socket, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	cancel, runErr := startReadyClient(t, client)
	deadline := time.Now().Add(time.Second)
	found := false
	for !found && time.Now().Before(deadline) {
		for _, request := range socket.snapshot() {
			if request.GetId() == 1 && request.GetMessage() == nil {
				found = true
				break
			}
		}
		if !found {
			time.Sleep(time.Millisecond)
		}
	}
	stopTestClient(t, client, cancel, runErr)
	if !found {
		t.Fatal("client did not send the protocol keepalive message")
	}
}

func TestClientDispatchesTypedMessageEvent(t *testing.T) {
	socket := newScriptedSocket()
	client := testClient(t, socket)
	received := make(chan *MessageCreateEvent, 1)
	client.OnMessageCreate(func(_ context.Context, event *MessageCreateEvent) error {
		received <- event
		return nil
	})
	cancel, runErr := startReadyClient(t, client)
	content := "hello"
	if err := socket.push(&updates.Update{Update: &updates.Update_MessageCreated{MessageCreated: &updates.UpdateMessageCreated{
		Message: &protoTypes.Message{MessageId: 9, Message: content},
		Author:  &protoTypes.User{Id: 10, Name: "Author"},
	}}}); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-received:
		if event.Message == nil || event.Message.Content != content || event.Message.Author == nil || event.Message.Author.ID != 10 || event.Author == nil || event.Author.ID != 10 || event.Client() != client {
			t.Fatalf("unexpected event: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("message event was not dispatched")
	}
	stopTestClient(t, client, cancel, runErr)
}

func TestClientRequestTimeoutRemovesPending(t *testing.T) {
	socket := newScriptedSocket()
	client := testClient(t, socket)
	cancel, runErr := startReadyClient(t, client)
	socket.mu.Lock()
	socket.drop = true
	socket.mu.Unlock()
	ctx, timeout := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer timeout()
	_, err := client.Communities.List(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected timeout error: %v", err)
	}
	client.activeMu.RLock()
	pending := client.active.broker.Pending()
	client.activeMu.RUnlock()
	if pending != 0 {
		t.Fatalf("pending request remains: %d", pending)
	}
	stopTestClient(t, client, cancel, runErr)
}

func TestClientConvertsRPCError(t *testing.T) {
	socket := newScriptedSocket()
	client := testClient(t, socket)
	cancel, runErr := startReadyClient(t, client)
	socket.mu.Lock()
	socket.rpcErr = true
	socket.mu.Unlock()
	_, err := client.Communities.List(context.Background())
	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) || rpcErr.Code != 403 || rpcErr.Message != "forbidden" {
		t.Fatalf("unexpected RPC error: %v", err)
	}
	stopTestClient(t, client, cancel, runErr)
}

func TestClientReturnsPermanentAuthorizationError(t *testing.T) {
	socket := newScriptedSocket()
	socket.mu.Lock()
	socket.authorizeErr = true
	socket.mu.Unlock()
	attempts := 0
	client, err := New(Config{
		Token:             "token",
		ClientID:          120715,
		ServerURL:         "ws://localhost",
		RequestTimeout:    50 * time.Millisecond,
		HeartbeatInterval: time.Hour,
		StableConnection:  time.Hour,
		BackoffMin:        time.Millisecond,
		BackoffMax:        2 * time.Millisecond,
		dial: func(context.Context, string) (gateway.Socket, error) {
			attempts++
			return socket, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = client.Run(context.Background())
	if !errors.Is(err, ErrPermanent) || !errors.Is(err, ErrAuthorizationFailed) || !IsPermanent(err) {
		t.Fatalf("unexpected permanent error: %v", err)
	}
	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) || rpcErr.Code != 403 {
		t.Fatalf("authorization RPC error was not preserved: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("permanent error triggered reconnects: %d attempts", attempts)
	}
}

func TestClientReconnectsAfterTransientAuthorizationError(t *testing.T) {
	first := newScriptedSocket()
	first.authorizeErr = true
	first.authorizeCode = 500
	second := newScriptedSocket()
	attempts := 0
	client, err := New(Config{
		Token:             "token",
		ClientID:          120715,
		ServerURL:         "ws://localhost",
		RequestTimeout:    50 * time.Millisecond,
		HeartbeatInterval: time.Hour,
		StableConnection:  time.Hour,
		BackoffMin:        time.Millisecond,
		BackoffMax:        2 * time.Millisecond,
		dial: func(context.Context, string) (gateway.Socket, error) {
			attempts++
			if attempts == 1 {
				return first, nil
			}
			return second, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	cancel, runErr := startReadyClient(t, client)
	if attempts < 2 {
		t.Fatalf("transient authorization error stopped reconnecting: attempts=%d", attempts)
	}
	stopTestClient(t, client, cancel, runErr)
}

func TestClientReconnectsAfterConnectionFailure(t *testing.T) {
	first := newScriptedSocket()
	first.drop = true
	second := newScriptedSocket()
	attempts := 0
	client, err := New(Config{
		Token:             "token",
		ClientID:          120715,
		ServerURL:         "ws://localhost",
		RequestTimeout:    50 * time.Millisecond,
		HeartbeatInterval: time.Hour,
		StableConnection:  time.Hour,
		BackoffMin:        time.Millisecond,
		BackoffMax:        2 * time.Millisecond,
		dial: func(context.Context, string) (gateway.Socket, error) {
			attempts++
			if attempts == 1 {
				return first, nil
			}
			return second, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	cancel, runErr := startReadyClient(t, client)
	if attempts < 2 {
		t.Fatalf("client did not reconnect, attempts=%d", attempts)
	}
	stopTestClient(t, client, cancel, runErr)
}

func TestClientRunIsSingleUse(t *testing.T) {
	client := testClient(t, newScriptedSocket())
	cancel, runErr := startReadyClient(t, client)
	cancel()
	if err := <-runErr; err != nil {
		t.Fatal(err)
	}
	if err := client.Run(context.Background()); !errors.Is(err, ErrRunCompleted) {
		t.Fatalf("unexpected second run error: %v", err)
	}
}

func TestClientShutdownWaitsForCleanup(t *testing.T) {
	client := testClient(t, newScriptedSocket())
	_, runErr := startReadyClient(t, client)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-runErr; err != nil {
		t.Fatal(err)
	}
	select {
	case <-client.Done():
	default:
		t.Fatal("client done channel is still open")
	}
}

func TestClientRejectsEmptyInteractionID(t *testing.T) {
	client := testClient(t, newScriptedSocket())
	if err := client.replyInteraction(context.Background(), 0, "reply"); err == nil {
		t.Fatal("zero interaction ID was accepted")
	}
}

func TestInteractionResponseCanAcknowledgeWithoutMessage(t *testing.T) {
	socket := newScriptedSocket()
	client := testClient(t, socket)
	cancel, runErr := startReadyClient(t, client)
	if err := client.acknowledgeInteraction(context.Background(), 9); err != nil {
		t.Fatal(err)
	}
	requests := socket.snapshot()
	response := requests[len(requests)-1].GetMessagesSetInteractionResponse()
	if response == nil || response.GetInteractionId() != 9 || response.Message != nil {
		t.Fatalf("unexpected interaction acknowledgement: %#v", response)
	}
	stopTestClient(t, client, cancel, runErr)
}

func TestClientEmitsConnectionLifecycleEvents(t *testing.T) {
	client := testClient(t, newScriptedSocket())
	connecting := make(chan *ConnectionEvent, 1)
	connected := make(chan *ConnectionEvent, 1)
	disconnected := make(chan *ConnectionEvent, 1)
	client.OnConnecting(func(_ context.Context, event *ConnectionEvent) error {
		connecting <- event
		return nil
	})
	client.OnConnected(func(_ context.Context, event *ConnectionEvent) error {
		connected <- event
		return nil
	})
	client.OnDisconnected(func(_ context.Context, event *ConnectionEvent) error {
		disconnected <- event
		return nil
	})
	cancel, runErr := startReadyClient(t, client)
	select {
	case event := <-connecting:
		if event.Attempt != 1 || event.State != Connecting || event.Client() != client {
			t.Fatalf("unexpected connecting event: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("connecting event was not emitted")
	}
	select {
	case event := <-connected:
		if event.Attempt != 1 || event.State != Initializing || event.Client() != client {
			t.Fatalf("unexpected connected event: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("connected event was not emitted")
	}
	stopTestClient(t, client, cancel, runErr)
	select {
	case event := <-disconnected:
		if event.Attempt != 1 || event.Err != nil || event.Client() != client {
			t.Fatalf("unexpected disconnected event: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("disconnected event was not emitted")
	}
}
