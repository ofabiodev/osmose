package client

import "github.com/ofabiodev/osmose/events"

// State is kept as an alias for compatibility with the root client API.
type State = events.State

const (
	Disconnected   = events.Disconnected
	Connecting     = events.Connecting
	Initializing   = events.Initializing
	Authenticating = events.Authenticating
	Ready          = events.Ready
	Closing        = events.Closing
)
