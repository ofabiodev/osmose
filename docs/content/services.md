---
title: Services
description: Use Osmose services for common bot operations.
group: Guides
order: 2
layout: doc
---

Services accept `context.Context` and small parameter structs.

## Messages

```go
sent, err := client.Messages.Send(ctx, messages.SendParams{
	Chat:    types.SelfChat(),
	Content: "Hello from Osmose",
})
if err != nil {
	return err
}

history, err := client.Messages.History(ctx, messages.HistoryParams{
	Chat:  types.SelfChat(),
	Limit: 50,
})

matches, err := client.Messages.Search(ctx, messages.SearchParams{
	Query: "release",
})
```

`SendParams` also supports protocol media references, entities, reply quotes,
bot identity, and buttons:

```go
_, err := client.Messages.Send(ctx, messages.SendParams{
	Chat:    types.SelfChat(),
	Content: "Choose:",
	BotInfo: &types.MessageBotInfo{Buttons: types.MessageButtons{{
		messages.LinkButton("Website", "https://osmium.chat"),
		messages.InteractionButton("Continue", "continue"),
	}}},
})
```

Available message operations are `Send`, `Reply`, `History`, `Search`,
`PinnedMessages`, `UnreadMentions`, `Edit`, and `Delete`.

## Chats and communities

```go
chat, err := client.Chats.Get(ctx, types.UserChat(userID))
members, err := client.Chats.Members(ctx, types.GroupChat(groupID))
communities, err := client.Communities.List(ctx)
channels, err := client.Communities.Channels(ctx, communityID)
channelMembers, err := client.Communities.ChannelMembers(ctx, communityID, channelID)
```

`Chats.Members` is for private or group chats. Use
`Communities.ChannelMembers` for the ordered member list of a community
channel:

```go
for _, entry := range channelMembers.Entries {
	if entry.User != nil {
		fmt.Println(entry.User.Username, entry.Nickname)
	}
}
```

Common chat references are:

```go
types.SelfChat()
types.UserChat(userID)
types.GroupChat(groupID)
types.ChannelChat(communityID, channelID)
```

## Rich objects

Community and message models returned by services keep their client reference,
so common operations can be called directly:

```go
list, err := client.Communities.List(ctx)
if err != nil {
	return err
}

community := list.Communities[0]
channels, err := community.Channels(ctx)
if err != nil {
	return err
}

message, err := channels[0].Send(ctx, types.MessageSendParams{Content: "Hello"})
if err != nil {
	return err
}

if err := message.Reply(ctx, "Thanks!"); err != nil {
	return err
}
```

The rich object operations are:

| Object | Common operations |
| --- | --- |
| `Community` | `Channels`, `Members`, `Roles`, `Edit`, `Delete`, `Leave`, `CreateChannel`, `CreateRole`, `AddMember`, `Unban` |
| `Channel` | `Send`, `Messages`, `History`, `Search`, `PinnedMessages`, `Members`, `Edit`, `Delete`, `CreateInvite` |
| `Message` | `Reply`, `Edit`, `Delete`, `React`, `Pin`, `Unpin`, `Forward` |
| `Member` | `Edit`, `SetRoles`, `AddRole`, `RemoveRole`, `Ban`, `Kick`, `Send` |
| `Role` | `Edit`, `Delete`, `SetPermissions`, `AddPermissions`, `RemovePermissions` |

`types.MessageReply` is exposed as `Message.ReplyInfo` because `Message.Reply`
is now the reply operation. The original protobuf message remains available in
`Message.Raw`.

## Users and reactions

```go
user, err := client.Users.Get(ctx, "some-user")
profile, err := client.Users.Profile(ctx, types.UserRef{ID: userID})

err = client.Reactions.Add(ctx, reactions.Params{
	Chat:      types.SelfChat(),
	MessageID: messageID,
	Emoji:     reactions.Emoji{Unicode: "👍"},
})
```

## Voice control plane

The current protocol exposes room discovery and participant state, not audio
packets:

```go
room, err := client.Voice.RequestRoom(ctx, types.ChannelChat(communityID, channelID))
states, err := client.Voice.RoomStates(ctx)
err = client.Voice.DisconnectUser(ctx, voice.DisconnectParams{
	Chat: types.ChannelChat(communityID, channelID), UserID: userID,
})
```

Use `OnVoiceRoomState` and `OnVoiceRoomParticipant` for live control-plane
updates. An audio/WebRTC transport will only be added when the Osmium protocol
defines one.

The services expose the bot-facing operations supported by the current Osmium
protocol. Use the [raw API](../protocol/) when an operation is not wrapped yet.
