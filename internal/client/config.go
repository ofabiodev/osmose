package client

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/ofabiodev/osmose/internal/gateway"
)

// Config controls a Client. Zero values use sensible defaults.
type Config struct {
	Token    string
	ClientID uint32

	ServerURL         string
	Logger            *slog.Logger
	OnHandlerError    HandlerErrorHandler
	OnEventOverflow   EventOverflowHandler
	RequestTimeout    time.Duration
	RequestInterval   time.Duration
	HeartbeatInterval time.Duration
	EventQueue        int
	EventWorkers      int
	WriteQueue        int
	WriteTimeout      time.Duration
	ReadLimit         int64
	DialTimeout       time.Duration
	StableConnection  time.Duration
	BackoffMin        time.Duration
	BackoffMax        time.Duration

	// dial replaces the default WebSocket dialer in package tests. It remains
	// private so the gateway stays an implementation detail of the SDK.
	dial func(context.Context, string) (gateway.Socket, error)

	DeviceType    string
	DeviceVersion string
	AppVersion    string
}

func (c Config) withDefaults() Config {
	if c.ServerURL == "" {
		c.ServerURL = "wss://ws-0.osmium.chat"
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	if c.RequestTimeout == 0 {
		c.RequestTimeout = 10 * time.Second
	}
	if c.HeartbeatInterval == 0 {
		c.HeartbeatInterval = 30 * time.Second
	}
	if c.EventQueue == 0 {
		c.EventQueue = 1024
	}
	if c.EventWorkers == 0 {
		c.EventWorkers = 1
	}
	if c.WriteQueue == 0 {
		c.WriteQueue = 1024
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 10 * time.Second
	}
	if c.ReadLimit == 0 {
		c.ReadLimit = 16 << 20
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = 10 * time.Second
	}
	if c.StableConnection == 0 {
		c.StableConnection = 30 * time.Second
	}
	if c.BackoffMin == 0 {
		c.BackoffMin = 500 * time.Millisecond
	}
	if c.BackoffMax == 0 {
		c.BackoffMax = 30 * time.Second
	}
	if c.DeviceType == "" {
		c.DeviceType = "osmose-go-bot"
	}
	if c.DeviceVersion == "" {
		c.DeviceVersion = fmt.Sprintf("go/%s %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	}
	if c.AppVersion == "" {
		c.AppVersion = "dev"
	}
	if c.dial == nil {
		dialer := gateway.Dialer{HandshakeTimeout: c.DialTimeout}
		c.dial = dialer.Dial
	}
	return c
}

func (c Config) validate() error {
	if strings.TrimSpace(c.Token) == "" {
		return fmt.Errorf("token is required")
	}
	if c.ClientID == 0 {
		return fmt.Errorf("client ID must be non-zero")
	}
	parsed, err := url.Parse(c.ServerURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "ws" && parsed.Scheme != "wss") {
		return fmt.Errorf("server URL must use ws:// or wss://")
	}
	if parsed.User != nil {
		return fmt.Errorf("server URL must not contain user info")
	}
	if c.RequestTimeout <= 0 || c.RequestInterval < 0 || c.HeartbeatInterval <= 0 || c.WriteTimeout <= 0 || c.DialTimeout <= 0 || c.StableConnection <= 0 {
		return fmt.Errorf("durations must be positive")
	}
	if c.EventQueue <= 0 || c.EventQueue > 1_000_000 || c.EventWorkers <= 0 || c.EventWorkers > 1024 || c.WriteQueue <= 0 || c.WriteQueue > 1_000_000 {
		return fmt.Errorf("queue and worker values are out of bounds")
	}
	if c.ReadLimit <= 0 || c.BackoffMin <= 0 || c.BackoffMax < c.BackoffMin {
		return fmt.Errorf("read limit and backoff values are invalid")
	}
	return nil
}
