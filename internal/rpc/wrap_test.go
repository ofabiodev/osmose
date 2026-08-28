package rpc

import (
	"errors"
	"testing"

	"github.com/ofabiodev/osmose/proto/core"
	protoMessages "github.com/ofabiodev/osmose/proto/messages"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestWrapUsesTheGeneratedClientMessageOneof(t *testing.T) {
	message, err := Wrap(42, &protoMessages.SendMessage{Message: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if message.GetId() != 42 {
		t.Fatalf("id=%d", message.GetId())
	}
	if _, ok := message.GetMessage().(*core.ClientMessage_MessagesSendMessage); !ok {
		t.Fatalf("unexpected message type %T", message.GetMessage())
	}
}

func TestWrapRejectsUnknownMessages(t *testing.T) {
	_, err := Wrap(1, &emptypb.Empty{})
	if !errors.Is(err, ErrUnsupportedRequest) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func BenchmarkWrap(b *testing.B) {
	request := &protoMessages.SendMessage{Message: "hello"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Wrap(uint32(i+1), request); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProtoRoundTrip(b *testing.B) {
	message, err := Wrap(1, &protoMessages.SendMessage{Message: "hello"})
	if err != nil {
		b.Fatal(err)
	}
	data, err := proto.Marshal(message)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		decoded := &core.ClientMessage{}
		if err := proto.Unmarshal(data, decoded); err != nil {
			b.Fatal(err)
		}
	}
}
