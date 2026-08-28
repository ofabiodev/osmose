package types

import (
	"testing"

	protoMedia "github.com/ofabiodev/osmose/proto/media"
	protoRefs "github.com/ofabiodev/osmose/proto/refs"
	protoTypes "github.com/ofabiodev/osmose/proto/types"
	protoUpdates "github.com/ofabiodev/osmose/proto/updates"
)

func TestMessageFromProtoKeepsUsefulFields(t *testing.T) {
	replyTo := uint64(3)
	editedAt := uint64(44)
	pinned := true
	value := &protoTypes.Message{
		ChatRef:   &protoRefs.ChatRef{Ref: &protoRefs.ChatRef_Self{Self: &protoRefs.RefSelf{}}},
		MessageId: 4,
		AuthorId:  8,
		Message:   "hello",
		ReplyTo:   &replyTo,
		EditedAt:  &editedAt,
		Pinned:    &pinned,
		Type:      protoTypes.Message_CALL.Enum(),
	}

	message := MessageFromProto(value)
	if message == nil || !message.Chat.Self || message.ID != 4 || message.AuthorID != 8 || message.Content != "hello" || message.ReplyTo != 3 || message.EditedAt != 44 || message.Type != protoTypes.Message_CALL || !message.Pinned || message.Raw != value {
		t.Fatalf("unexpected model: %#v", message)
	}
}

func TestChannelRefRoundTrip(t *testing.T) {
	want := ChannelRef{CommunityID: 7, ChannelID: 9}
	value, err := want.ToProto()
	if err != nil {
		t.Fatal(err)
	}
	if got := ChannelRefFromProto(value); got != want {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	if _, err := (ChannelRef{}).ToProto(); err == nil {
		t.Fatal("invalid channel reference was accepted")
	}
}

func TestTopLevelModelsMapProtocolFields(t *testing.T) {
	draft := "draft"
	parentID := uint64(4)
	communityName := "community"
	conversation := ConversationFromProto(&protoTypes.Conversation{
		ChatRef:             &protoRefs.ChatRef{Ref: &protoRefs.ChatRef_Self{Self: &protoRefs.RefSelf{}}},
		LastMessageId:       10,
		UnreadCount:         2,
		Draft:               &draft,
		UnreadMentionsCount: 1,
	})
	channel := ChannelFromProto(&protoTypes.Channel{Id: 5, CommunityId: 6, Name: "general", ParentId: &parentID})
	community := CommunityFromProto(&protoTypes.Community{Id: 7, Name: communityName})
	member := CommunityMemberFromProto(&protoTypes.CommunityMember{Id: 8, CommunityId: 7, RoleIds: []uint64{9}})
	chatMember := ChatMemberFromProto(&protoTypes.ChatMember{UserId: 10})

	if conversation == nil || !conversation.Chat.Self || conversation.LastMessageID != 10 || conversation.UnreadCount != 2 || conversation.Draft == nil || *conversation.Draft != draft || conversation.Raw == nil {
		t.Fatalf("unexpected conversation model: %#v", conversation)
	}
	if channel == nil || channel.ID != 5 || channel.CommunityID != 6 || channel.Name != "general" || channel.ParentID == nil || *channel.ParentID != 4 || channel.Raw == nil {
		t.Fatalf("unexpected channel model: %#v", channel)
	}
	if community == nil || community.ID != 7 || community.Name != communityName || community.Raw == nil {
		t.Fatalf("unexpected community model: %#v", community)
	}
	if member == nil || member.ID != 8 || member.CommunityID != 7 || len(member.RoleIDs) != 1 || member.RoleIDs[0] != 9 || member.Raw == nil {
		t.Fatalf("unexpected community member model: %#v", member)
	}
	if chatMember == nil || chatMember.UserID != 10 || chatMember.Raw == nil {
		t.Fatalf("unexpected chat member model: %#v", chatMember)
	}
}

func TestMessageMapsReplyAndBotInfo(t *testing.T) {
	replyID := uint64(12)
	value := &protoTypes.Message{
		Reply: &protoTypes.Message_MessageReplyInfo{
			MessageId: &replyID,
			Quote:     &protoTypes.Message_MessageReplyInfo_Quote{Message: "quoted"},
		},
		BotInfo: &protoTypes.Message_MessageBotInfo{
			Cloak: &protoTypes.Message_MessageBotInfo_MessageCloak{Name: "Helper"},
			Buttons: &protoTypes.Message_MessageBotInfo_Buttons{
				Rows: []*protoTypes.Message_MessageBotInfo_Buttons_ButtonRow{{
					Buttons: []*protoTypes.MessageButton{{
						Label:  "Open",
						Action: &protoTypes.MessageButton_Url{Url: &protoTypes.MessageButton_MessageButtonUrl{Url: "https://example.com"}},
					}},
				}},
			},
		},
	}
	message := MessageFromProto(value)
	if message == nil || message.Reply == nil || message.Reply.MessageID != 12 || message.Reply.Quote == nil || message.Reply.Quote.Content != "quoted" {
		t.Fatalf("unexpected reply model: %#v", message)
	}
	if message.BotInfo == nil || message.BotInfo.Cloak == nil || message.BotInfo.Cloak.Name != "Helper" || len(message.BotInfo.Buttons) != 1 || message.BotInfo.Buttons[0][0].URL != "https://example.com" {
		t.Fatalf("unexpected bot info model: %#v", message)
	}
}

func TestInteractionFromProto(t *testing.T) {
	data := "confirm"
	value := &protoUpdates.UpdateInteraction{InteractionId: 1, UserId: 2, MessageId: 3, Data: &data}
	interaction := InteractionFromProto(value)
	if interaction == nil || interaction.ID != 1 || interaction.UserID != 2 || interaction.MessageID != 3 || interaction.Data != data || interaction.Raw != value {
		t.Fatalf("unexpected interaction model: %#v", interaction)
	}
}

func TestNestedModelsDoNotExposeMutableProtocolValuesDirectly(t *testing.T) {
	preview := []byte{1, 2, 3}
	photoValue := &protoTypes.ChatPhoto{FileId: 7, Preview: preview}
	photo := ChatPhotoFromProto(photoValue)
	photo.Preview[0] = 9
	if photoValue.GetPreview()[0] != 1 || photo.FileID != 7 || photo.Raw != photoValue {
		t.Fatalf("photo model was not detached: %#v", photo)
	}

	media := EmbedMedia("https://example.com/image.png")
	converted, err := media.ToProto()
	if err != nil || converted.GetEmbed().GetUrl() != "https://example.com/image.png" {
		t.Fatalf("media reference conversion failed: %#v, %v", converted, err)
	}
	if _, err := (&MediaRef{}).ToProto(); err == nil {
		t.Fatal("empty media reference was accepted")
	}

	permission := PermissionOverridesFromProto(&protoTypes.PermissionOverrides{Pos: 1, Neg: 2})
	if permission == nil || permission.Pos != 1 || permission.Neg != 2 {
		t.Fatalf("permission model was not mapped: %#v", permission)
	}

	mediaValue := &protoMedia.MediaRef{Ref: &protoMedia.MediaRef_Embed{Embed: &protoMedia.MediaRefEmbed{Url: "https://example.com"}}}
	fromProto := MediaRefFromProto(mediaValue)
	if fromProto == nil || fromProto.EmbedURL != "https://example.com" || fromProto.Raw != mediaValue {
		t.Fatalf("media model was not mapped: %#v", fromProto)
	}
}
