package users

import (
	"context"
	"fmt"
	"strings"

	"github.com/ofabiodev/osmose/internal/rpc"
	"github.com/ofabiodev/osmose/proto/core"
	protoUsers "github.com/ofabiodev/osmose/proto/users"
	"github.com/ofabiodev/osmose/types"
	"google.golang.org/protobuf/proto"
)

// Service provides user lookup operations.
type Service struct {
	call         func(context.Context, proto.Message) (*core.RPCResult, error)
	objectClient *types.ObjectClient
}

func New(call func(context.Context, proto.Message) (*core.RPCResult, error), clients ...*types.ObjectClient) *Service {
	var objectClient *types.ObjectClient
	if len(clients) != 0 && clients[0] != nil {
		objectClient = clients[0]
	} else {
		objectClient = types.NewObjectClient(call)
	}
	return &Service{call: objectClient.Call, objectClient: objectClient}
}

type Profile struct {
	Ref types.UserRef
	Bio string
	Raw *protoUsers.Profile
}

// Get looks up a user by their public username.
func (s *Service) Get(ctx context.Context, username string) (*types.User, error) {
	if strings.TrimSpace(username) == "" {
		return nil, fmt.Errorf("username is required")
	}
	result, err := s.do(ctx, &protoUsers.LookupUsername{Username: username})
	if err != nil {
		return nil, err
	}
	details := result.GetUserDetails()
	if details == nil || details.GetUser() == nil {
		return nil, &rpc.UnexpectedResultError{Method: "users.lookupUsername"}
	}
	return types.UserFromProto(details.GetUser(), s.objectClient), nil
}

func (s *Service) Profile(ctx context.Context, ref types.UserRef) (*Profile, error) {
	if ref.ID == 0 {
		return nil, fmt.Errorf("user ID is required")
	}
	result, err := s.do(ctx, &protoUsers.GetProfile{Ref: ref.ToProto()})
	if err != nil {
		return nil, err
	}
	value := result.GetProfile()
	if value == nil {
		return nil, &rpc.UnexpectedResultError{Method: "users.getProfile"}
	}
	return &Profile{Ref: ref, Bio: value.GetBio(), Raw: value}, nil
}

func (s *Service) do(ctx context.Context, request proto.Message) (*core.RPCResult, error) {
	if s == nil || s.call == nil {
		return nil, fmt.Errorf("users service is not initialized")
	}
	return s.call(ctx, request)
}
