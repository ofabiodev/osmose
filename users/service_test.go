package users

import (
	"context"
	"testing"

	"github.com/ofabiodev/osmose/proto/core"
	protoTypes "github.com/ofabiodev/osmose/proto/types"
	protoUsers "github.com/ofabiodev/osmose/proto/users"
	"github.com/ofabiodev/osmose/types"
	"google.golang.org/protobuf/proto"
)

func TestGetBuildsLookupAndMapsUser(t *testing.T) {
	var got proto.Message
	service := New(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
		got = request
		return &core.RPCResult{Result: &core.RPCResult_UserDetails{UserDetails: &protoUsers.UserDetails{
			User: &protoTypes.User{Id: 8, Name: "Author"},
		}}}, nil
	})

	user, err := service.Get(context.Background(), "author")
	if err != nil {
		t.Fatal(err)
	}
	request, ok := got.(*protoUsers.LookupUsername)
	if !ok || request.GetUsername() != "author" {
		t.Fatalf("unexpected request: %#v", got)
	}
	if user.ID != 8 || user.Name != "Author" {
		t.Fatalf("unexpected user: %#v", user)
	}
}

func TestProfileBuildsReferenceAndMapsBio(t *testing.T) {
	var got proto.Message
	service := New(func(_ context.Context, request proto.Message) (*core.RPCResult, error) {
		got = request
		return &core.RPCResult{Result: &core.RPCResult_Profile{Profile: &protoUsers.Profile{Bio: "hello"}}}, nil
	})

	profile, err := service.Profile(context.Background(), types.UserRef{ID: 12})
	if err != nil {
		t.Fatal(err)
	}
	request, ok := got.(*protoUsers.GetProfile)
	if !ok || request.GetRef().GetUserId() != 12 {
		t.Fatalf("unexpected request: %#v", got)
	}
	if profile.Ref.ID != 12 || profile.Bio != "hello" {
		t.Fatalf("unexpected profile: %#v", profile)
	}
}
