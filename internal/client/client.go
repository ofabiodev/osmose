package client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ofabiodev/osmose/chats"
	"github.com/ofabiodev/osmose/communities"
	"github.com/ofabiodev/osmose/internal/gateway"
	"github.com/ofabiodev/osmose/internal/rpc"
	"github.com/ofabiodev/osmose/internal/scheduler"
	"github.com/ofabiodev/osmose/messages"
	"github.com/ofabiodev/osmose/proto/auth"
	"github.com/ofabiodev/osmose/proto/core"
	protoMessages "github.com/ofabiodev/osmose/proto/messages"
	"github.com/ofabiodev/osmose/reactions"
	"github.com/ofabiodev/osmose/types"
	"github.com/ofabiodev/osmose/users"
	"github.com/ofabiodev/osmose/voice"
	"google.golang.org/protobuf/proto"
)

type activeConnection struct {
	conn   *gateway.Connection
	broker *rpc.Broker
	ctx    context.Context
	cancel context.CancelFunc
}

// Client is the central Osmose client. Services are initialized and ready to
// use immediately; network operations wait until Run has completed auth.
// A client has one lifecycle: reconnects happen inside Run, and a completed
// Run cannot be started again.
type Client struct {
	config Config
	logger *slog.Logger

	Messages    *messages.Service
	Chats       *chats.Service
	Communities *communities.Service
	Users       *users.Service
	Reactions   *reactions.Service
	Voice       *voice.Service

	events       *eventDispatcher
	raw          *RawClient
	objectClient *types.ObjectClient

	stateMu sync.RWMutex
	state   State
	ready   chan struct{}

	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc
	done            chan struct{}
	doneOnce        sync.Once

	activeMu sync.RWMutex
	active   *activeConnection

	userMu sync.RWMutex
	user   *types.User

	scheduler scheduler.Scheduler

	sessionID atomic.Uint64

	runMu       sync.Mutex
	running     bool
	closed      bool
	runStarted  bool
	runFinished bool
	runCancel   context.CancelFunc
}

// RawClient is the escape hatch for generated protobuf requests not wrapped by
// a high-level service yet.
type RawClient struct{ client *Client }

func (r *RawClient) Call(ctx context.Context, request proto.Message) (*core.RPCResult, error) {
	if r == nil || r.client == nil {
		return nil, ErrClosed
	}
	return r.client.call(ctx, request)
}

func New(config Config) (*Client, error) {
	config = config.withDefaults()
	if err := config.validate(); err != nil {
		return nil, err
	}
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
	c := &Client{
		config:          config,
		logger:          config.Logger,
		state:           Disconnected,
		ready:           make(chan struct{}),
		events:          newEventDispatcher(config.EventQueue, config.EventWorkers, config.Logger),
		lifecycleCtx:    lifecycleCtx,
		lifecycleCancel: lifecycleCancel,
		done:            make(chan struct{}),
		scheduler:       scheduler.Scheduler{Interval: config.RequestInterval},
	}
	c.events.onHandlerError = config.OnHandlerError
	c.events.onEventOverflow = config.OnEventOverflow
	c.events.setClient(c)
	c.raw = &RawClient{client: c}
	c.objectClient = types.NewObjectClient(c.call)
	c.Messages = messages.New(c.call)
	c.Chats = chats.New(c.call)
	c.Communities = communities.New(c.call)
	c.Users = users.New(c.call)
	c.Reactions = reactions.New(c.call)
	c.Voice = voice.New(c.call)
	return c, nil
}

func (c *Client) State() State {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.state
}

func (c *Client) User() *types.User {
	c.userMu.RLock()
	defer c.userMu.RUnlock()
	return c.user
}

func (c *Client) SessionID() types.ID { return types.ID(c.sessionID.Load()) }

func (c *Client) Raw() *RawClient { return c.raw }

// Done closes after the client's run and shutdown cleanup have completed.
func (c *Client) Done() <-chan struct{} {
	if c == nil {
		return nil
	}
	return c.done
}

func (c *Client) WaitReady(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		c.stateMu.RLock()
		state, ready := c.state, c.ready
		c.stateMu.RUnlock()
		if state == Ready {
			return nil
		}
		if c.isClosed() || state == Closing {
			return ErrClosed
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ready:
		}
	}
}

// Run owns the connection lifecycle until ctx is canceled, Close is called,
// or a permanent protocol error is returned. It may be called only once per
// Client; use a new Client for a new lifecycle.
func (c *Client) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	c.runMu.Lock()
	if c.closed {
		c.runMu.Unlock()
		return ErrClosed
	}
	if c.running {
		c.runMu.Unlock()
		return ErrAlreadyRunning
	}
	if c.runStarted {
		c.runMu.Unlock()
		return ErrRunCompleted
	}
	runCtx, cancel := context.WithCancel(ctx)
	c.runStarted = true
	c.running = true
	c.runCancel = cancel
	c.runMu.Unlock()

	c.events.start(runCtx)
	defer func() {
		cancel()
		c.closeActive()
		c.lifecycleCancel()
		c.events.close()
		c.setState(Disconnected)
		c.runMu.Lock()
		c.running = false
		c.runFinished = true
		c.runCancel = nil
		c.runMu.Unlock()
		c.doneOnce.Do(func() { close(c.done) })
	}()

	backoff := gateway.NewBackoff(c.config.BackoffMin, c.config.BackoffMax)
	attempt := 0
	for runCtx.Err() == nil {
		attemptNumber := attempt + 1
		c.emitConnecting(runCtx, &ConnectionEvent{Base: newEventBase(c), Attempt: attemptNumber, State: Connecting})
		stable, err := c.runConnection(runCtx, attemptNumber)
		eventErr := err
		if runCtx.Err() != nil {
			eventErr = nil
		}
		c.emitDisconnected(runCtx, &ConnectionEvent{Base: newEventBase(c), Attempt: attemptNumber, State: c.State(), Err: eventErr})
		if runCtx.Err() != nil {
			break
		}
		if err != nil {
			c.emitConnectionError(runCtx, &ConnectionEvent{Base: newEventBase(c), Attempt: attemptNumber, State: c.State(), Err: err})
			c.logger.Warn("Osmium connection stopped", "error", err)
			if IsPermanent(err) {
				return err
			}
		}
		if stable {
			attempt = 0
		}
		delay := backoff.Delay(attempt)
		attempt++
		c.emitReconnecting(runCtx, &ConnectionEvent{Base: newEventBase(c), Attempt: attempt + 1, RetryIn: delay, State: Disconnected, Err: err})
		if err := gateway.WaitBackoff(runCtx, delay); err != nil {
			break
		}
	}
	return nil
}

// Close stops the run loop and active connection. Run returns after its
// current handler and connection cleanup finish.
func (c *Client) Close() error {
	c.runMu.Lock()
	if c.closed {
		noRun := !c.runStarted
		c.runMu.Unlock()
		if noRun {
			c.doneOnce.Do(func() { close(c.done) })
		}
		return nil
	}
	if c.runFinished {
		c.runMu.Unlock()
		return nil
	}
	cancel := c.runCancel
	noRun := !c.runStarted
	c.closed = true
	c.runMu.Unlock()

	c.lifecycleCancel()
	c.setState(Closing)
	if cancel != nil {
		cancel()
	}
	c.activeMu.RLock()
	active := c.active
	c.activeMu.RUnlock()
	if active != nil {
		active.broker.FailAll(ErrDisconnected)
		active.conn.Close()
	}
	if noRun {
		c.doneOnce.Do(func() { close(c.done) })
	}
	return nil
}

// Shutdown closes the client and waits for all run and event-dispatch cleanup.
// It is safe to call when the client is not running.
func (c *Client) Shutdown(ctx context.Context) error {
	if c == nil {
		return ErrClosed
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := c.Close(); err != nil {
		return err
	}
	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// DroppedEvents returns the number of updates discarded because the bounded
// event queue was full.
func (c *Client) DroppedEvents() uint64 {
	if c == nil || c.events == nil {
		return 0
	}
	return c.events.dropped.Load()
}

func (c *Client) call(ctx context.Context, request proto.Message) (*core.RPCResult, error) {
	if request == nil {
		return nil, ErrUnsupportedRequest
	}
	state := c.State()
	if state == Closing || c.isClosed() {
		return nil, ErrClosed
	}
	if state == Disconnected || state == Connecting {
		return nil, ErrNotConnected
	}
	if state != Ready && !isHandshakeRequest(request) {
		return nil, ErrNotReady
	}
	c.activeMu.RLock()
	active := c.active
	c.activeMu.RUnlock()
	if active == nil {
		return nil, ErrNotConnected
	}
	return c.callOn(ctx, active, request)
}

func (c *Client) runConnection(root context.Context, attempt int) (bool, error) {
	c.setState(Connecting)
	socket, err := c.config.dial(root, c.config.ServerURL)
	if err != nil {
		c.setState(Disconnected)
		return false, fmt.Errorf("dial websocket: %w", err)
	}
	connCtx, cancel := context.WithCancel(root)
	active := &activeConnection{broker: rpc.NewBroker(), ctx: connCtx, cancel: cancel}
	connection := gateway.NewConnection(connCtx, socket, gateway.Options{
		QueueSize:    c.config.WriteQueue,
		WriteTimeout: c.config.WriteTimeout,
		ReadLimit:    c.config.ReadLimit,
		Logger:       c.logger,
		OnMessage:    func(_ int, data []byte) { c.handleFrame(active, data) },
		OnError:      func(error) { active.broker.FailAll(ErrDisconnected) },
	})
	active.conn = connection
	c.activeMu.Lock()
	c.active = active
	c.activeMu.Unlock()
	defer c.cleanupActive(active)

	connection.Start()
	c.setState(Initializing)
	c.emitConnected(connCtx, &ConnectionEvent{Base: newEventBase(c), Attempt: attempt, State: Initializing})
	initialized, err := c.callOn(connCtx, active, &core.Initialize{
		ClientId:      c.config.ClientID,
		DeviceType:    c.config.DeviceType,
		DeviceVersion: c.config.DeviceVersion,
		AppVersion:    c.config.AppVersion,
	})
	if err != nil {
		return false, fmt.Errorf("initialize: %w", err)
	}
	if initialized.GetInitialized() == nil {
		return false, permanentError(ErrProtocolMismatch, &UnexpectedResultError{Method: "core.initialize"})
	}

	c.setState(Authenticating)
	authorized, err := c.callOn(connCtx, active, &auth.Authorize{Token: c.config.Token})
	if err != nil {
		if isPermanentAuthorizationError(err) {
			return false, permanentError(ErrAuthorizationFailed, fmt.Errorf("authorize: %w", err))
		}
		return false, fmt.Errorf("authorize: %w", err)
	}
	authorization := authorized.GetAuthorization()
	if authorization == nil || authorization.GetUser() == nil {
		return false, permanentError(ErrProtocolMismatch, &UnexpectedResultError{Method: "auth.authorize"})
	}
	user := types.UserFromProto(authorization.GetUser())
	c.userMu.Lock()
	c.user = user
	c.userMu.Unlock()
	c.sessionID.Store(authorization.GetSessionId())
	c.setState(Ready)

	keepalivePayload, err := proto.Marshal(&core.ClientMessage{Id: 1})
	if err != nil {
		return false, fmt.Errorf("marshal keepalive: %w", err)
	}
	keepalive := gateway.NewKeepalive(connCtx, c.config.HeartbeatInterval, connection.Enqueue, gateway.Frame{
		Type: websocket.BinaryMessage,
		Data: keepalivePayload,
	}, c.logger)
	keepalive.Start()
	defer keepalive.Stop()
	c.emitReady(connCtx, &ReadyEvent{Base: newEventBase(c), User: user, SessionID: types.ID(authorization.GetSessionId())})

	stableTimer := time.NewTimer(c.config.StableConnection)
	defer stableTimer.Stop()
	stable := false
	stableC := stableTimer.C
	for {
		select {
		case <-root.Done():
			return stable, root.Err()
		case err := <-connection.Errors():
			if err == nil {
				continue
			}
			return stable, err
		case <-stableC:
			stable = true
			stableC = nil
		}
	}
}

func (c *Client) callOn(ctx context.Context, active *activeConnection, request proto.Message) (*core.RPCResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
	}
	if err := c.scheduler.Wait(ctx); err != nil {
		return nil, err
	}
	id, call, err := active.broker.Register()
	if err != nil {
		if errors.Is(err, rpc.ErrBrokerClosed) {
			return nil, ErrNotConnected
		}
		return nil, err
	}
	defer active.broker.Cancel(id, call)

	message, err := rpc.Wrap(id, request)
	if err != nil {
		return nil, err
	}
	data, err := proto.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	if err := active.conn.Enqueue(ctx, gateway.Frame{Type: websocket.BinaryMessage, Data: data}); err != nil {
		if errors.Is(err, gateway.ErrClosed) {
			return nil, ErrNotConnected
		}
		return nil, err
	}

	select {
	case outcome := <-call.Done():
		if outcome.Err != nil {
			return nil, outcome.Err
		}
		if result := outcome.Result; result != nil {
			if rpcError := result.GetError(); rpcError != nil {
				return nil, &RPCError{Code: rpcError.GetErrorCode(), Message: rpcError.GetErrorMessage(), TraceID: result.GetTraceId()}
			}
			return result, nil
		}
		return nil, &UnexpectedResultError{Method: "rpc"}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *Client) handleFrame(active *activeConnection, data []byte) {
	serverMessage := &core.ServerMessage{}
	if err := proto.Unmarshal(data, serverMessage); err != nil {
		c.logger.Warn("unmarshal server message failed", "error", err)
		return
	}
	if result := serverMessage.GetResult(); result != nil {
		if !active.broker.Resolve(result) {
			c.logger.Debug("ignored late or unknown RPC result", "request_id", result.GetReqId())
		}
		return
	}
	if update := serverMessage.GetUpdate(); update != nil {
		if err := c.events.enqueue(active.ctx, update); err != nil && active.ctx.Err() == nil {
			c.logger.Warn("event queue enqueue failed", "error", err)
		}
		return
	}
	c.logger.Warn("received empty server message")
}

func (c *Client) cleanupActive(active *activeConnection) {
	active.cancel()
	active.broker.FailAll(ErrDisconnected)
	active.conn.Close()
	active.conn.Wait()
	c.activeMu.Lock()
	if c.active == active {
		c.active = nil
	}
	c.activeMu.Unlock()
	if c.State() != Closing {
		c.setState(Disconnected)
	}
}

func (c *Client) closeActive() {
	c.activeMu.RLock()
	active := c.active
	c.activeMu.RUnlock()
	if active != nil {
		c.cleanupActive(active)
	}
}

func (c *Client) setState(next State) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	previous := c.state
	c.state = next
	if next == Ready && previous != Ready {
		closeReady(c.ready)
	}
	if (next == Disconnected || next == Closing) && previous != next {
		closeReady(c.ready)
	}
	if next == Disconnected && previous != Disconnected {
		c.ready = make(chan struct{})
	}
}

func closeReady(ready chan struct{}) {
	select {
	case <-ready:
	default:
		close(ready)
	}
}

func (c *Client) isClosed() bool {
	c.runMu.Lock()
	defer c.runMu.Unlock()
	return c.closed
}

func isHandshakeRequest(request proto.Message) bool {
	switch request.(type) {
	case *core.Initialize, *auth.Authorize:
		return true
	default:
		return false
	}
}

func (c *Client) replyInteraction(ctx context.Context, interactionID types.ID, content string) error {
	return c.respondInteraction(ctx, interactionID, &content)
}

func (c *Client) acknowledgeInteraction(ctx context.Context, interactionID types.ID) error {
	return c.respondInteraction(ctx, interactionID, nil)
}

func (c *Client) respondInteraction(ctx context.Context, interactionID types.ID, content *string) error {
	if interactionID == 0 {
		return fmt.Errorf("interaction ID is required")
	}
	result, err := c.call(ctx, &protoMessages.SetInteractionResponse{InteractionId: uint64(interactionID), Message: content})
	if err != nil {
		return err
	}
	if result.GetResult() != nil {
		return &UnexpectedResultError{Method: "messages.setInteractionResponse"}
	}
	return nil
}
