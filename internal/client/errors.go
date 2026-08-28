package client

import (
	"errors"
	"fmt"

	"github.com/ofabiodev/osmose/events"
	"github.com/ofabiodev/osmose/internal/rpc"
)

var (
	ErrClosed              = events.ErrClosed
	ErrNotConnected        = errors.New("osmose client is not connected")
	ErrNotReady            = errors.New("osmose client is not ready")
	ErrAlreadyRunning      = errors.New("osmose client is already running")
	ErrRunCompleted        = errors.New("osmose client run has completed")
	ErrPermanent           = errors.New("osmose permanent error")
	ErrAuthorizationFailed = errors.New("osmose authorization failed")
	ErrProtocolMismatch    = errors.New("osmose protocol mismatch")
	ErrDisconnected        = rpc.ErrDisconnected
	ErrUnsupportedRequest  = rpc.ErrUnsupportedRequest
)

// PermanentError marks an error that should not be retried by Client.Run.
// Its cause remains available through errors.Is and errors.As.
type PermanentError struct{ Err error }

func (e *PermanentError) Error() string {
	if e == nil || e.Err == nil {
		return ErrPermanent.Error()
	}
	return e.Err.Error()
}

func (e *PermanentError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *PermanentError) Is(target error) bool { return target == ErrPermanent }

// IsPermanent reports whether err contains a permanent Osmose error.
func IsPermanent(err error) bool {
	var permanent *PermanentError
	return errors.As(err, &permanent)
}

func permanentError(reason, cause error) error {
	if cause == nil {
		cause = reason
	}
	return &PermanentError{Err: fmt.Errorf("%w: %w", reason, cause)}
}

// RPCError is an error returned by the Osmium RPC endpoint.
type RPCError struct {
	Code    uint32
	Message string
	TraceID string
}

func (e *RPCError) Error() string {
	if e.TraceID == "" {
		return fmt.Sprintf("osmium RPC error %d: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("osmium RPC error %d: %s (trace %s)", e.Code, e.Message, e.TraceID)
}

// UnexpectedResultError means the server answered with a result different
// from the one defined for the requested method.
type UnexpectedResultError = rpc.UnexpectedResultError

// Osmium currently uses code 403 when the Authorize request is rejected.
// Keep this classification narrow: other RPC errors may be transient.
const authorizationRejectedCode uint32 = 403

func isPermanentAuthorizationError(err error) bool {
	var rpcErr *RPCError
	return errors.As(err, &rpcErr) && rpcErr.Code == authorizationRejectedCode
}
