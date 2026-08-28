package events

// State describes the public lifecycle state of an Osmose client.
type State uint8

const (
	Disconnected State = iota
	Connecting
	Initializing
	Authenticating
	Ready
	Closing
)

func (s State) String() string {
	switch s {
	case Disconnected:
		return "disconnected"
	case Connecting:
		return "connecting"
	case Initializing:
		return "initializing"
	case Authenticating:
		return "authenticating"
	case Ready:
		return "ready"
	case Closing:
		return "closing"
	default:
		return "unknown"
	}
}
