package messages

import (
	"context"
	"fmt"

	"github.com/ofabiodev/osmose/internal/rpc"
	"github.com/ofabiodev/osmose/proto/core"
	protoMedia "github.com/ofabiodev/osmose/proto/media"
	protoMessages "github.com/ofabiodev/osmose/proto/messages"
	"github.com/ofabiodev/osmose/types"
	"google.golang.org/protobuf/proto"
)

// Service provides the message operations useful to bots.
type Service struct {
	call         func(context.Context, proto.Message) (*core.RPCResult, error)
	objectClient *types.ObjectClient
}

func New(call func(context.Context, proto.Message) (*core.RPCResult, error)) *Service {
	return &Service{call: call, objectClient: types.NewObjectClient(call)}
}

type Button = types.MessageButton
type Buttons = types.MessageButtons
type BotInfo = types.MessageBotInfo

func LinkButton(label, url string) Button { return Button{Label: label, URL: url} }
func InteractionButton(label, data string) Button {
	return Button{Label: label, Interaction: data}
}
func ClipboardButton(label, text string) Button {
	return Button{Label: label, Clipboard: text}
}

type SendParams struct {
	Chat           types.ChatRef
	Content        string
	ReplyTo        types.ID
	ReplyQuote     *types.MessageQuote
	Media          []*types.MediaRef
	Entities       []*types.MessageEntity
	SuppressEmbeds bool
	BotInfo        *types.MessageBotInfo
}

type SentMessage struct {
	ID  types.ID
	Raw *protoMessages.SentMessage
}

type HistoryParams = types.MessageHistoryParams

type History struct {
	Messages []*types.Message
	Users    []*types.User
	Raw      *protoMessages.Messages
}

// SearchResult is the same response shape as History, exposed under the
// operation's name for discoverability.
type SearchResult = History

type SearchParams = types.MessageSearchParams

type EditParams = types.MessageEditParams

type DeleteParams = types.MessageDeleteParams

func (s *Service) Send(ctx context.Context, params SendParams) (*SentMessage, error) {
	if params.Content == "" && len(params.Media) == 0 && params.BotInfo == nil {
		return nil, fmt.Errorf("message content is required")
	}
	for i, media := range params.Media {
		if media == nil {
			return nil, fmt.Errorf("message media %d is nil", i)
		}
	}
	chat, err := params.Chat.ToProto()
	if err != nil {
		return nil, err
	}
	media, err := mediaRefsToProto(params.Media)
	if err != nil {
		return nil, err
	}
	request := &protoMessages.SendMessage{
		ChatRef:        chat,
		Message:        params.Content,
		Media:          media,
		Entities:       params.Entities,
		SuppressEmbeds: params.SuppressEmbeds,
	}
	if params.ReplyQuote != nil && params.ReplyTo == 0 {
		return nil, fmt.Errorf("reply quote requires a reply message ID")
	}
	if params.ReplyTo != 0 {
		request.ReplyTo = &protoMessages.SendMessage_ReplyTo{MessageId: uint64(params.ReplyTo)}
		if params.ReplyQuote != nil {
			request.ReplyTo.Quote = &protoMessages.SendMessage_ReplyTo_Quote{
				Message:  params.ReplyQuote.Content,
				Entities: params.ReplyQuote.Entities,
				Offset:   params.ReplyQuote.Offset,
			}
		}
	}
	if params.BotInfo != nil {
		request.BotInfo, err = botInfoToProto(params.BotInfo)
		if err != nil {
			return nil, err
		}
	}
	result, err := s.do(ctx, request)
	if err != nil {
		return nil, err
	}
	sent := result.GetSentMessage()
	if sent == nil {
		return nil, &rpc.UnexpectedResultError{Method: "messages.sendMessage"}
	}
	return &SentMessage{ID: types.ID(sent.GetMessageId()), Raw: sent}, nil
}

func (s *Service) Reply(ctx context.Context, message *types.Message, content string) (*SentMessage, error) {
	if message == nil {
		return nil, fmt.Errorf("message is required")
	}
	if message.ID == 0 {
		return nil, fmt.Errorf("message ID is required")
	}
	return s.Send(ctx, SendParams{Chat: message.Chat, Content: content, ReplyTo: message.ID})
}

func (s *Service) History(ctx context.Context, params HistoryParams) (*History, error) {
	chat, err := params.Chat.ToProto()
	if err != nil {
		return nil, err
	}
	request := &protoMessages.GetHistory{ChatRef: chat}
	if params.Limit != 0 {
		request.Limit = &params.Limit
	}
	if params.Before != 0 {
		request.Offset = &protoMessages.GetHistory_Before{Before: uint64(params.Before)}
	}
	result, err := s.do(ctx, request)
	if err != nil {
		return nil, err
	}
	history := result.GetMessages()
	if history == nil {
		return nil, &rpc.UnexpectedResultError{Method: "messages.getHistory"}
	}
	return mapHistory(history, s.objectClient), nil
}

// Search finds messages using the protocol's global or chat-scoped search.
// A zero Chat is valid only for an unscoped search.
func (s *Service) Search(ctx context.Context, params SearchParams) (*SearchResult, error) {
	if params.Query == "" {
		return nil, fmt.Errorf("search query is required")
	}
	if params.Since != 0 && params.Before != 0 {
		return nil, fmt.Errorf("search offset must be since or before")
	}
	request := &protoMessages.Search{Query: params.Query, Scoped: params.Scoped}
	if params.Chat != (types.ChatRef{}) {
		if !params.Chat.Valid() {
			return nil, fmt.Errorf("invalid chat reference")
		}
		request.ChatRef, _ = params.Chat.ToProto()
	} else if params.Scoped {
		return nil, fmt.Errorf("scoped search requires a chat reference")
	}
	if params.Since != 0 {
		request.Offset = &protoMessages.Search_Since{Since: uint64(params.Since)}
	}
	if params.Before != 0 {
		request.Offset = &protoMessages.Search_Before{Before: uint64(params.Before)}
	}
	result, err := s.do(ctx, request)
	if err != nil {
		return nil, err
	}
	value := result.GetMessages()
	if value == nil {
		return nil, &rpc.UnexpectedResultError{Method: "messages.search"}
	}
	return mapHistory(value, s.objectClient), nil
}

// PinnedMessages returns the pinned messages in a chat.
func (s *Service) PinnedMessages(ctx context.Context, chat types.ChatRef) (*History, error) {
	ref, err := chat.ToProto()
	if err != nil {
		return nil, err
	}
	result, err := s.do(ctx, &protoMessages.GetPinnedMessages{ChatRef: ref})
	if err != nil {
		return nil, err
	}
	value := result.GetMessages()
	if value == nil {
		return nil, &rpc.UnexpectedResultError{Method: "messages.getPinnedMessages"}
	}
	return mapHistory(value, s.objectClient), nil
}

// UnreadMentions returns the unread mention IDs in a chat.
func (s *Service) UnreadMentions(ctx context.Context, chat types.ChatRef) ([]types.ID, error) {
	ref, err := chat.ToProto()
	if err != nil {
		return nil, err
	}
	result, err := s.do(ctx, &protoMessages.GetUnreadMentions{ChatRef: ref})
	if err != nil {
		return nil, err
	}
	value := result.GetUnreadMentions()
	if value == nil {
		return nil, &rpc.UnexpectedResultError{Method: "messages.getUnreadMentions"}
	}
	ids := make([]types.ID, len(value.GetMessageIds()))
	for i, id := range value.GetMessageIds() {
		ids[i] = types.ID(id)
	}
	return ids, nil
}

func (s *Service) Edit(ctx context.Context, params EditParams) error {
	if params.MessageID == 0 {
		return fmt.Errorf("message ID is required")
	}
	chat, err := params.Chat.ToProto()
	if err != nil {
		return err
	}
	media, err := mediaRefsToProto(params.Media)
	if err != nil {
		return err
	}
	request := &protoMessages.EditMessage{
		ChatRef:        chat,
		MessageId:      uint64(params.MessageID),
		Message:        params.Content,
		RemoveMedia:    params.RemoveMedia,
		Media:          media,
		Entities:       params.Entities,
		SuppressEmbeds: params.SuppressEmbeds,
	}
	if params.Buttons != nil {
		request.Buttons, err = buttonsToProto(*params.Buttons)
		if err != nil {
			return err
		}
	}
	result, err := s.do(ctx, request)
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "messages.editMessage")
}

func (s *Service) Delete(ctx context.Context, params DeleteParams) error {
	if len(params.MessageIDs) == 0 {
		return fmt.Errorf("message IDs are required")
	}
	for _, id := range params.MessageIDs {
		if id == 0 {
			return fmt.Errorf("message IDs must be non-zero")
		}
	}
	chat, err := params.Chat.ToProto()
	if err != nil {
		return err
	}
	ids := make([]uint64, len(params.MessageIDs))
	for i, id := range params.MessageIDs {
		ids[i] = uint64(id)
	}
	result, err := s.do(ctx, &protoMessages.DeleteMessage{ChatRef: chat, MessageIds: ids})
	if err != nil {
		return err
	}
	return rpc.EnsureVoid(result, "messages.deleteMessage")
}

func mapHistory(value *protoMessages.Messages, client *types.ObjectClient) *History {
	history := &History{Raw: value}
	users := make(map[types.ID]*types.User, len(value.GetUsers()))
	for _, user := range value.GetUsers() {
		model := types.UserFromProto(user)
		history.Users = append(history.Users, model)
		if model != nil {
			users[model.ID] = model
		}
	}
	for _, message := range value.GetMessages() {
		model := types.MessageFromProto(message, client)
		if model != nil {
			model.Author = users[model.AuthorID]
		}
		history.Messages = append(history.Messages, model)
	}
	return history
}

func botInfoToProto(value *types.MessageBotInfo) (*protoMessages.SendMessage_BotInfo, error) {
	if value == nil {
		return nil, nil
	}
	info := &protoMessages.SendMessage_BotInfo{}
	if value.Cloak != nil {
		info.Cloak = &protoMessages.SendMessage_BotInfo_MessageCloak{Name: value.Cloak.Name}
		if value.Cloak.PhotoID != 0 {
			photoID := uint64(value.Cloak.PhotoID)
			info.Cloak.PhotoId = &photoID
		}
	}
	if value.Buttons != nil {
		buttons, err := buttonsToProto(value.Buttons)
		if err != nil {
			return nil, err
		}
		info.Buttons = buttons
	}
	return info, nil
}

func buttonsToProto(value types.MessageButtons) (*protoMessages.SendMessage_BotInfo_Buttons, error) {
	if len(value) > 5 {
		return nil, fmt.Errorf("message buttons cannot have more than 5 rows")
	}
	buttons := &protoMessages.SendMessage_BotInfo_Buttons{Rows: make([]*protoMessages.SendMessage_BotInfo_Buttons_ButtonRow, len(value))}
	for i, row := range value {
		if len(row) > 5 {
			return nil, fmt.Errorf("message button row %d cannot have more than 5 buttons", i)
		}
		buttons.Rows[i] = &protoMessages.SendMessage_BotInfo_Buttons_ButtonRow{Buttons: make([]*protoMessages.MessageButton, len(row))}
		for j, button := range row {
			converted, err := buttonToProto(button)
			if err != nil {
				return nil, fmt.Errorf("button %d in row %d: %w", j, i, err)
			}
			buttons.Rows[i].Buttons[j] = converted
		}
	}
	return buttons, nil
}

func buttonToProto(value types.MessageButton) (*protoMessages.MessageButton, error) {
	if value.Label == "" {
		return nil, fmt.Errorf("button label is required")
	}
	actions := 0
	button := &protoMessages.MessageButton{Label: value.Label}
	if value.URL != "" {
		actions++
		button.Action = &protoMessages.MessageButton_Url{Url: &protoMessages.MessageButton_MessageButtonUrl{Url: value.URL}}
	}
	if value.Interaction != "" {
		actions++
		button.Action = &protoMessages.MessageButton_Interaction{Interaction: &protoMessages.MessageButton_MessageButtonInteraction{Data: value.Interaction}}
	}
	if value.Clipboard != "" {
		actions++
		button.Action = &protoMessages.MessageButton_Clipboard{Clipboard: &protoMessages.MessageButton_MessageButtonClipboard{Text: value.Clipboard}}
	}
	if actions != 1 {
		return nil, fmt.Errorf("button must define exactly one action")
	}
	return button, nil
}

func (s *Service) do(ctx context.Context, request proto.Message) (*core.RPCResult, error) {
	if s == nil || s.call == nil {
		return nil, fmt.Errorf("messages service is not initialized")
	}
	return s.call(ctx, request)
}

func mediaRefsToProto(values []*types.MediaRef) ([]*protoMedia.MediaRef, error) {
	if len(values) == 0 {
		return nil, nil
	}
	result := make([]*protoMedia.MediaRef, len(values))
	for i, value := range values {
		if value == nil {
			return nil, fmt.Errorf("message media %d is nil", i)
		}
		converted, err := value.ToProto()
		if err != nil {
			return nil, fmt.Errorf("message media %d: %w", i, err)
		}
		result[i] = converted
	}
	return result, nil
}
