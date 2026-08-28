package rpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/ofabiodev/osmose/proto/core"
	"google.golang.org/protobuf/proto"
)

var ErrUnsupportedRequest = errors.New("unsupported Osmium request")

type CallFunc func(context.Context, proto.Message) (*core.RPCResult, error)

type UnexpectedResultError struct{ Method string }

func (e *UnexpectedResultError) Error() string {
	return fmt.Sprintf("unexpected result for %s", e.Method)
}

// EnsureVoid validates an RPC response whose protocol result is intentionally
// empty. A nil RPCResult means the broker/service boundary returned an invalid
// response and must not be treated as success.
func EnsureVoid(result *core.RPCResult, method string) error {
	if result == nil {
		return &UnexpectedResultError{Method: method}
	}
	if result.GetResult() != nil {
		return &UnexpectedResultError{Method: method}
	}
	return nil
}
