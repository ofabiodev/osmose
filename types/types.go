// Package types contains the small public models shared by Osmose services
// and events. Raw protobuf values remain available through their Raw fields.
package types

import (
	"fmt"

	protoChats "github.com/ofabiodev/osmose/proto/chats"
	protoMedia "github.com/ofabiodev/osmose/proto/media"
	protoReactions "github.com/ofabiodev/osmose/proto/reactions"
	protoRefs "github.com/ofabiodev/osmose/proto/refs"
	protoTypes "github.com/ofabiodev/osmose/proto/types"
	protoUpdates "github.com/ofabiodev/osmose/proto/updates"
	"google.golang.org/protobuf/proto"
)

// ID is an Osmium snowflake represented by its wire value.
type ID uint64

func (id ID) Uint64() uint64 { return uint64(id) }

// ChatPhoto is the stable SDK representation of a user's or chat's photo.
// Raw is available for protocol-specific fields and advanced callers.
type ChatPhoto struct {
	FileID  ID
	Preview []byte
	Raw     *protoTypes.ChatPhoto
}

func ChatPhotoFromProto(value *protoTypes.ChatPhoto) *ChatPhoto {
	if value == nil {
		return nil
	}
	return &ChatPhoto{
		FileID:  ID(value.GetFileId()),
		Preview: append([]byte(nil), value.GetPreview()...),
		Raw:     value,
	}
}

func (p ChatPhoto) ToProto() *protoTypes.ChatPhoto {
	return &protoTypes.ChatPhoto{FileId: uint64(p.FileID), Preview: append([]byte(nil), p.Preview...)}
}

// UserStatus is the stable SDK representation of a user's presence.
type UserStatus struct {
	Status     protoTypes.UserStatus_Status
	Activities []UserActivity
	Raw        *protoTypes.UserStatus
}

// UserActivity describes one activity from a user's presence.
type UserActivity struct {
	Title     string
	Type      protoTypes.UserStatus_Activity_ActivityType
	StartTime uint64
	EndTime   *uint64
	State     *string
	Raw       *protoTypes.UserStatus_Activity
}

func UserStatusFromProto(value *protoTypes.UserStatus) *UserStatus {
	if value == nil {
		return nil
	}
	status := &UserStatus{Status: value.GetStatus(), Raw: value}
	for _, activity := range value.GetActivities() {
		if activity == nil {
			continue
		}
		status.Activities = append(status.Activities, UserActivity{
			Title:     activity.GetTitle(),
			Type:      activity.GetType(),
			StartTime: activity.GetStartTime(),
			EndTime:   cloneUint64(activity.EndTime),
			State:     cloneString(activity.State),
			Raw:       activity,
		})
	}
	return status
}

// User is the useful, stable part of an Osmium user model.
type User struct {
	Partial  bool
	client   *ObjectClient
	ID       ID
	Name     string
	Username string
	Status   *UserStatus
	Photo    *ChatPhoto
	Icon     ID
	Color    uint32
	Bot      bool
	Raw      *protoTypes.User
}

// Interaction is the protocol's typed interaction update. The current
// Osmium schema intentionally leaves the payload as data; applications can
// use Data to distinguish their own buttons or interaction actions.
type Interaction struct {
	ID        ID
	UserID    ID
	MessageID ID
	Data      string
	Raw       *protoUpdates.UpdateInteraction
}

func InteractionFromProto(value *protoUpdates.UpdateInteraction) *Interaction {
	if value == nil {
		return nil
	}
	return &Interaction{
		ID:        ID(value.GetInteractionId()),
		UserID:    ID(value.GetUserId()),
		MessageID: ID(value.GetMessageId()),
		Data:      value.GetData(),
		Raw:       value,
	}
}

func UserFromProto(value *protoTypes.User, clients ...*ObjectClient) *User {
	if value == nil {
		return nil
	}
	return &User{
		client:   firstObjectClient(clients...),
		ID:       ID(value.GetId()),
		Name:     value.GetName(),
		Username: value.GetUsername(),
		Status:   UserStatusFromProto(value.GetStatus()),
		Photo:    ChatPhotoFromProto(value.GetPhoto()),
		Icon:     ID(value.GetIcon()),
		Color:    value.GetColor(),
		Bot:      value.GetBot(),
		Raw:      value,
	}
}

// Message exposes the fields most bot handlers need without making them
// understand protobuf's generated field names.
type Message struct {
	Partial   bool
	ID        ID
	Chat      ChatRef
	AuthorID  ID
	Author    *User
	Content   string
	ReplyTo   ID
	Media     []*MessageMedia
	Entities  []*MessageEntity
	ReplyInfo *MessageReply
	BotInfo   *MessageBotInfo
	EditedAt  uint64
	Type      MessageType
	Pinned    bool
	Raw       *protoTypes.Message
	client    *ObjectClient
}

func MessageFromProto(value *protoTypes.Message, clients ...*ObjectClient) *Message {
	if value == nil {
		return nil
	}
	model := &Message{
		ID:        ID(value.GetMessageId()),
		Chat:      ChatRefFromProto(value.GetChatRef()),
		AuthorID:  ID(value.GetAuthorId()),
		Content:   value.GetMessage(),
		ReplyTo:   ID(value.GetReplyTo()),
		Media:     cloneMessageMedia(value.GetMedia()),
		Entities:  cloneMessageEntities(value.GetEntities()),
		ReplyInfo: MessageReplyFromProto(value.GetReply()),
		BotInfo:   MessageBotInfoFromProto(value.GetBotInfo()),
		EditedAt:  value.GetEditedAt(),
		Type:      value.GetType(),
		Pinned:    value.GetPinned(),
		Raw:       value,
		client:    firstObjectClient(clients...),
	}
	if model.client != nil {
		model.Author, _ = model.client.Managers().Users.Get(model.AuthorID)
	}
	return model
}

// MessageQuote is the quoted part of a reply, when the protocol includes it.
type MessageQuote struct {
	Content  string
	Entities []*MessageEntity
	Offset   *uint64
}

// MessageReply contains the richer reply metadata. ReplyTo remains available
// on Message for the message ID used by the protocol.
type MessageReply struct {
	MessageID ID
	Quote     *MessageQuote
}

func MessageReplyFromProto(value *protoTypes.Message_MessageReplyInfo) *MessageReply {
	if value == nil {
		return nil
	}
	reply := &MessageReply{MessageID: ID(value.GetMessageId())}
	if quote := value.GetQuote(); quote != nil {
		reply.Quote = &MessageQuote{
			Content:  quote.GetMessage(),
			Entities: cloneMessageEntities(quote.GetEntities()),
			Offset:   cloneUint64(quote.Offset),
		}
	}
	return reply
}

// MessageCloak describes the optional bot identity shown on a message.
// PhotoID is used when sending; Photo is populated when reading a message.
type MessageCloak struct {
	Name    string
	PhotoID ID
	Photo   *ChatPhoto
}

// MessageButton is one of the button actions supported by the Osmium
// messages protocol. Exactly one action should be set.
type MessageButton struct {
	Label       string
	URL         string
	Interaction string
	Clipboard   string
}

// MessageButtons is organized as rows, matching the protocol's maximum of
// five rows with at most five buttons per row.
type MessageButtons [][]MessageButton

// UploadedFileRef identifies a file previously uploaded through the Osmium
// media protocol.
type UploadedFileRef struct {
	ID        ID
	Name      string
	PartCount uint32
	Raw       *protoMedia.UploadedFileRef
}

func UploadedFile(id ID, name string, partCount uint32) *UploadedFileRef {
	return &UploadedFileRef{ID: id, Name: name, PartCount: partCount}
}

func UploadedFileRefFromProto(value *protoMedia.UploadedFileRef) *UploadedFileRef {
	if value == nil {
		return nil
	}
	return &UploadedFileRef{ID: ID(value.GetId()), Name: value.GetName(), PartCount: value.GetPartCount(), Raw: value}
}

func (f UploadedFileRef) ToProto() *protoMedia.UploadedFileRef {
	return &protoMedia.UploadedFileRef{Id: uint64(f.ID), Name: f.Name, PartCount: f.PartCount}
}

// MediaRef is a stable SDK reference used when sending message media.
// Complex protocol-specific metadata remains available through Raw.
type MediaRef struct {
	EmbedURL string
	Uploaded *UploadedFileRef
	Filename string
	MIMEType string
	Raw      *protoMedia.MediaRef
}

// EmbedMedia creates a URL-backed message attachment reference.
func EmbedMedia(url string) *MediaRef { return &MediaRef{EmbedURL: url} }

// UploadedMedia creates a message attachment reference for an uploaded file.
func UploadedMedia(file *UploadedFileRef, filename, mimetype string) *MediaRef {
	return &MediaRef{Uploaded: file, Filename: filename, MIMEType: mimetype}
}

func MediaRefFromProto(value *protoMedia.MediaRef) *MediaRef {
	if value == nil {
		return nil
	}
	ref := &MediaRef{Raw: value}
	switch value := value.GetRef().(type) {
	case *protoMedia.MediaRef_Embed:
		if value.Embed != nil {
			ref.EmbedURL = value.Embed.GetUrl()
		}
	case *protoMedia.MediaRef_Uploaded:
		uploaded := value.Uploaded
		if uploaded != nil {
			ref.Uploaded = UploadedFileRefFromProto(uploaded.GetFile())
			ref.Filename = uploaded.GetFilename()
			ref.MIMEType = uploaded.GetMimetype()
		}
	}
	return ref
}

func (r MediaRef) ToProto() (*protoMedia.MediaRef, error) {
	if r.EmbedURL != "" && r.Uploaded != nil {
		return nil, fmt.Errorf("media reference cannot contain both embed and uploaded media")
	}
	if r.EmbedURL != "" {
		return &protoMedia.MediaRef{Ref: &protoMedia.MediaRef_Embed{Embed: &protoMedia.MediaRefEmbed{Url: r.EmbedURL}}}, nil
	}
	if r.Uploaded == nil {
		return nil, fmt.Errorf("media reference must contain an embed URL or uploaded file")
	}
	uploaded := &protoMedia.MediaRefUploadedFile{
		File:     r.Uploaded.ToProto(),
		Mimetype: r.MIMEType,
	}
	if r.Filename != "" {
		uploaded.Filename = &r.Filename
	}
	return &protoMedia.MediaRef{Ref: &protoMedia.MediaRef_Uploaded{Uploaded: uploaded}}, nil
}

// PermissionOverrides contains the positive and negative permission masks.
type PermissionOverrides struct {
	Pos uint64
	Neg uint64
	Raw *protoTypes.PermissionOverrides
}

func PermissionOverridesFromProto(value *protoTypes.PermissionOverrides) *PermissionOverrides {
	if value == nil {
		return nil
	}
	return &PermissionOverrides{Pos: value.GetPos(), Neg: value.GetNeg(), Raw: value}
}

func (p PermissionOverrides) ToProto() *protoTypes.PermissionOverrides {
	return &protoTypes.PermissionOverrides{Pos: p.Pos, Neg: p.Neg}
}

// MessageBotInfo contains optional bot-specific message presentation.
type MessageBotInfo struct {
	Cloak   *MessageCloak
	Buttons MessageButtons
	Raw     *protoTypes.Message_MessageBotInfo
}

func MessageBotInfoFromProto(value *protoTypes.Message_MessageBotInfo) *MessageBotInfo {
	if value == nil {
		return nil
	}
	info := &MessageBotInfo{Raw: value}
	if cloak := value.GetCloak(); cloak != nil {
		info.Cloak = &MessageCloak{Name: cloak.GetName(), Photo: ChatPhotoFromProto(cloak.GetPhoto())}
	}
	if buttons := value.GetButtons(); buttons != nil {
		info.Buttons = make(MessageButtons, len(buttons.GetRows()))
		for i, row := range buttons.GetRows() {
			if row == nil {
				continue
			}
			info.Buttons[i] = make([]MessageButton, len(row.GetButtons()))
			for j, button := range row.GetButtons() {
				info.Buttons[i][j] = MessageButtonFromProto(button)
			}
		}
	}
	return info
}

func MessageButtonFromProto(value *protoTypes.MessageButton) MessageButton {
	if value == nil {
		return MessageButton{}
	}
	button := MessageButton{Label: value.GetLabel()}
	if action := value.GetUrl(); action != nil {
		button.URL = action.GetUrl()
	}
	if action := value.GetInteraction(); action != nil {
		button.Interaction = action.GetData()
	}
	if action := value.GetClipboard(); action != nil {
		button.Clipboard = action.GetText()
	}
	return button
}

// ChatRef identifies one of Osmium's chat kinds. Exactly one kind should be
// set; use the constructors for the common cases.
type ChatRef struct {
	UserID      ID
	CommunityID ID
	ChannelID   ID
	GroupID     ID
	Self        bool
}

func UserChat(id ID) ChatRef { return ChatRef{UserID: id} }
func ChannelChat(communityID, channelID ID) ChatRef {
	return ChatRef{CommunityID: communityID, ChannelID: channelID}
}
func GroupChat(id ID) ChatRef { return ChatRef{GroupID: id} }
func SelfChat() ChatRef       { return ChatRef{Self: true} }

func (r ChatRef) Valid() bool {
	switch {
	case r.Self:
		return r.UserID == 0 && r.CommunityID == 0 && r.ChannelID == 0 && r.GroupID == 0
	case r.UserID != 0:
		return r.CommunityID == 0 && r.ChannelID == 0 && r.GroupID == 0
	case r.ChannelID != 0:
		return r.UserID == 0 && r.CommunityID != 0 && r.GroupID == 0
	case r.GroupID != 0:
		return r.UserID == 0 && r.CommunityID == 0 && r.ChannelID == 0
	default:
		return false
	}
}

// ChannelRef identifies a community channel for update events and raw calls.
type ChannelRef struct {
	CommunityID ID
	ChannelID   ID
}

func (r ChannelRef) Valid() bool { return r.CommunityID != 0 && r.ChannelID != 0 }

func (r ChannelRef) ToProto() (*protoRefs.ChannelRef, error) {
	if !r.Valid() {
		return nil, fmt.Errorf("invalid channel reference")
	}
	return &protoRefs.ChannelRef{CommunityId: uint64(r.CommunityID), ChannelId: uint64(r.ChannelID)}, nil
}

func ChannelRefFromProto(value *protoRefs.ChannelRef) ChannelRef {
	if value == nil {
		return ChannelRef{}
	}
	return ChannelRef{CommunityID: ID(value.GetCommunityId()), ChannelID: ID(value.GetChannelId())}
}

// ToProto is the explicit bridge for advanced callers and service internals.
func (r ChatRef) ToProto() (*protoRefs.ChatRef, error) {
	if !r.Valid() {
		return nil, fmt.Errorf("invalid chat reference")
	}
	switch {
	case r.Self:
		return &protoRefs.ChatRef{Ref: &protoRefs.ChatRef_Self{Self: &protoRefs.RefSelf{}}}, nil
	case r.UserID != 0:
		return &protoRefs.ChatRef{Ref: &protoRefs.ChatRef_User{User: &protoRefs.UserRef{UserId: uint64(r.UserID)}}}, nil
	case r.ChannelID != 0:
		return &protoRefs.ChatRef{Ref: &protoRefs.ChatRef_Channel{Channel: &protoRefs.ChannelRef{CommunityId: uint64(r.CommunityID), ChannelId: uint64(r.ChannelID)}}}, nil
	default:
		return &protoRefs.ChatRef{Ref: &protoRefs.ChatRef_Group{Group: &protoRefs.GroupRef{GroupId: uint64(r.GroupID)}}}, nil
	}
}

func ChatRefFromProto(value *protoRefs.ChatRef) ChatRef {
	if value == nil {
		return ChatRef{}
	}
	switch ref := value.GetRef().(type) {
	case *protoRefs.ChatRef_User:
		return UserChat(ID(ref.User.GetUserId()))
	case *protoRefs.ChatRef_Channel:
		return ChannelChat(ID(ref.Channel.GetCommunityId()), ID(ref.Channel.GetChannelId()))
	case *protoRefs.ChatRef_Group:
		return GroupChat(ID(ref.Group.GetGroupId()))
	case *protoRefs.ChatRef_Self:
		return SelfChat()
	default:
		return ChatRef{}
	}
}

// UserRef is used by user profile requests.
type UserRef struct{ ID ID }

func (r UserRef) Valid() bool { return r.ID != 0 }

func (r UserRef) ToProto() *protoRefs.UserRef { return &protoRefs.UserRef{UserId: uint64(r.ID)} }

// Conversation is the useful bot-facing state for a chat.
type Conversation struct {
	Chat                ChatRef
	LastMessageID       ID
	LastReadMessageID   ID
	UnreadCount         uint32
	Draft               *string
	Permissions         *PermissionOverrides
	NotifPrefs          NotifPrefs
	UnreadMentionsCount uint32
	Raw                 *protoTypes.Conversation
}

func ConversationFromProto(value *protoTypes.Conversation) *Conversation {
	if value == nil {
		return nil
	}
	return &Conversation{
		Chat:                ChatRefFromProto(value.GetChatRef()),
		LastMessageID:       ID(value.GetLastMessageId()),
		LastReadMessageID:   ID(value.GetLastReadMessageId()),
		UnreadCount:         value.GetUnreadCount(),
		Draft:               cloneString(value.Draft),
		Permissions:         PermissionOverridesFromProto(value.GetPermissions()),
		NotifPrefs:          value.GetNotifPrefs(),
		UnreadMentionsCount: value.GetUnreadMentionsCount(),
		Raw:                 value,
	}
}

// Group is a private/group chat model.
type Group struct {
	ID             ID
	Name           string
	Owner          bool
	ParticipantIDs []ID
	Photo          *ChatPhoto
	Raw            *protoTypes.Group
}

func GroupFromProto(value *protoTypes.Group) *Group {
	if value == nil {
		return nil
	}
	return &Group{
		ID:             ID(value.GetId()),
		Name:           value.GetName(),
		Owner:          value.GetOwner(),
		ParticipantIDs: idsFromUint64(value.GetParticipantIds()),
		Photo:          ChatPhotoFromProto(value.GetPhoto()),
		Raw:            value,
	}
}

// Channel is a community channel model.
type Channel struct {
	Partial              bool
	ID                   ID
	CommunityID          ID
	Name                 string
	Type                 ChannelType
	Position             uint32
	ParentID             *ID
	Flags                uint64
	HighlightedMessageID *ID
	Description          *string
	SlowmodeSeconds      *uint32
	PreferredRegion      *string
	Raw                  *protoTypes.Channel
	client               *ObjectClient
}

func ChannelFromProto(value *protoTypes.Channel, clients ...*ObjectClient) *Channel {
	if value == nil {
		return nil
	}
	return &Channel{
		ID:                   ID(value.GetId()),
		CommunityID:          ID(value.GetCommunityId()),
		Name:                 value.GetName(),
		Type:                 value.GetType(),
		Position:             value.GetPosition(),
		ParentID:             cloneID(value.ParentId),
		Flags:                value.GetFlags(),
		HighlightedMessageID: cloneID(value.HighlightedMsgId),
		Description:          cloneString(value.Description),
		SlowmodeSeconds:      cloneUint32(value.SlowmodeSeconds),
		PreferredRegion:      cloneString(value.PreferredRegion),
		Raw:                  value,
		client:               firstObjectClient(clients...),
	}
}

// Community is a community model.
type Community struct {
	Partial     bool
	ID          ID
	Owner       bool
	Name        string
	Photo       *ChatPhoto
	Permissions uint64
	NotifPrefs  NotifPrefs
	Username    *string
	Raw         *protoTypes.Community
	client      *ObjectClient
}

func CommunityFromProto(value *protoTypes.Community, clients ...*ObjectClient) *Community {
	if value == nil {
		return nil
	}
	return &Community{
		ID:          ID(value.GetId()),
		Owner:       value.GetOwner(),
		Name:        value.GetName(),
		Photo:       ChatPhotoFromProto(value.GetPhoto()),
		Permissions: value.GetPermissions(),
		NotifPrefs:  value.GetNotifPrefs(),
		Username:    cloneString(value.Username),
		Raw:         value,
		client:      firstObjectClient(clients...),
	}
}

// CommunityMember is a user's membership in a community.
type CommunityMember struct {
	Partial     bool
	ID          ID
	CommunityID ID
	RoleIDs     []ID
	Nickname    *string
	User        *User
	Raw         *protoTypes.CommunityMember
	client      *ObjectClient
}

func CommunityMemberFromProto(value *protoTypes.CommunityMember, clients ...*ObjectClient) *CommunityMember {
	if value == nil {
		return nil
	}
	model := &CommunityMember{
		ID:          ID(value.GetId()),
		CommunityID: ID(value.GetCommunityId()),
		RoleIDs:     idsFromUint64(value.GetRoleIds()),
		Nickname:    cloneString(value.Nickname),
		Raw:         value,
		client:      firstObjectClient(clients...),
	}
	if model.client != nil {
		model.User, _ = model.client.Managers().Users.Get(model.ID)
	}
	return model
}

// MemberListEntry is one ordered entry returned for a community channel.
// Exactly one of User or Divider is normally set.
type MemberListEntry struct {
	User     *User
	Nickname *string
	Divider  *MemberListDivider
	Raw      *protoUpdates.MemberListEntry
}

func MemberListEntryFromProto(value *protoUpdates.MemberListEntry, clients ...*ObjectClient) *MemberListEntry {
	if value == nil {
		return nil
	}
	entry := &MemberListEntry{Raw: value}
	if user := value.GetUser(); user != nil {
		entry.User = UserFromProto(user.GetUser(), clients...)
		entry.Nickname = cloneString(user.Nickname)
	}
	if divider := value.GetDivider(); divider != nil {
		entry.Divider = MemberListDividerFromProto(divider)
	}
	return entry
}

// MemberListDivider groups channel members by online state or role.
type MemberListDivider struct {
	Online      *bool
	RoleID      ID
	MemberCount uint32
	Raw         *protoUpdates.MemberListEntryDivider
}

func MemberListDividerFromProto(value *protoUpdates.MemberListEntryDivider) *MemberListDivider {
	if value == nil {
		return nil
	}
	divider := &MemberListDivider{
		MemberCount: value.GetMemberCount(),
		Raw:         value,
	}
	switch inner := value.GetInner().(type) {
	case *protoUpdates.MemberListEntryDivider_Online:
		online := inner.Online
		divider.Online = &online
	case *protoUpdates.MemberListEntryDivider_RoleId:
		divider.RoleID = ID(inner.RoleId)
	}
	return divider
}

// ChatMember is a membership entry returned for a chat.
type ChatMember struct {
	UserID      ID
	Permissions *uint64
	Raw         *protoTypes.ChatMember
}

func ChatMemberFromProto(value *protoTypes.ChatMember) *ChatMember {
	if value == nil {
		return nil
	}
	return &ChatMember{
		UserID:      ID(value.GetUserId()),
		Permissions: cloneUint64(value.Permissions),
		Raw:         value,
	}
}

func idsFromUint64(values []uint64) []ID {
	if len(values) == 0 {
		return nil
	}
	ids := make([]ID, len(values))
	for i, value := range values {
		ids[i] = ID(value)
	}
	return ids
}

func cloneID(value *uint64) *ID {
	if value == nil {
		return nil
	}
	cloned := ID(*value)
	return &cloned
}

func cloneUint64(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneUint32(value *uint32) *uint32 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneMessageMedia(values []*protoMedia.MessageMedia) []*MessageMedia {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]*MessageMedia, len(values))
	for i, value := range values {
		if value != nil {
			cloned[i] = proto.Clone(value).(*protoMedia.MessageMedia)
		}
	}
	return cloned
}

func cloneMessageEntities(values []*protoTypes.MessageEntity) []*MessageEntity {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]*MessageEntity, len(values))
	for i, value := range values {
		if value != nil {
			cloned[i] = proto.Clone(value).(*protoTypes.MessageEntity)
		}
	}
	return cloned
}

// These leaf protocol types are intentionally aliases: duplicating their
// nested oneofs and enums would make the public API larger without removing
// useful protocol detail. The common mutable/reference values above provide
// the stable layer; complex oneof leaves remain available through Raw data.
type MessageEntity = protoTypes.MessageEntity
type MessageMedia = protoMedia.MessageMedia
type MessageType = protoTypes.Message_MessageType
type TypingAction = protoChats.TypingAction
type MessageReactions = protoReactions.MessageReactions
type ChannelType = protoTypes.ChannelType
type NotifPrefs = protoTypes.NotifPrefs
type CommunityPermission = protoTypes.CommunityPermission
