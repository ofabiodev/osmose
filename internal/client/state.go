package client

import "github.com/ofabiodev/osmose/events"

// State describes the client's connection lifecycle.
type State = events.State

const (
	Disconnected   = events.Disconnected
	Connecting     = events.Connecting
	Initializing   = events.Initializing
	Authenticating = events.Authenticating
	Ready          = events.Ready
	Closing        = events.Closing
)
