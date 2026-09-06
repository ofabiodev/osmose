---
title: Services
description: Use Osmose services for common bot operations.
group: Guides
order: 2
layout: doc
---

Services accept `context.Context` and small parameter structs.

Managers and rich objects are the primary API for stateful community and message
operations. Specialized services remain for protocol areas without a dedicated
manager, such as chat operations and voice.
For cache lookups and scoped managers, see [managers and state](../state-management/).

## Messages

```go
sent, err := client.Messages.In(types.SelfChat()).Create(ctx, "Hello from Osmose")
if err != nil {
	return err
}

history, err := client.Messages.In(types.SelfChat()).List(ctx, types.MessageHistoryParams{
	Limit: 50,
})

matches, err := client.Channels.In(communityID).Ref(channelID).Search(ctx, types.MessageSearchParams{
	Query: "release",
})
```

`MessageSendParams` also supports protocol media references, entities, reply quotes,
bot identity, and buttons:

```go
_, err := client.Messages.In(types.SelfChat()).CreateWith(ctx, types.MessageSendParams{
	Content: "Choose:",
	BotInfo: &types.MessageBotInfo{Buttons: types.MessageButtons{{
		{Label: "Website", URL: "https://osmium.chat"},
		{Label: "Continue", Interaction: "continue"},
	}}},
})
```

Available manager operations are `Create`, `CreateWith`, `List`, `Search`,
`PinnedMessages`, `UnreadMentions`, `Edit`, and `Delete`. Rich message objects
also provide `Send`, `Reply`, `Edit`, `Delete`, and reaction/pin operations.

## Chats and communities

```go
chat, err := client.Chats.Get(ctx, types.UserChat(userID))
members, err := client.Chats.Members(ctx, types.GroupChat(groupID))
communities, err := client.Communities.List(ctx)
channels, err := client.Communities.Ref(communityID).Channels().List(ctx)
channelMembers, err := client.Channels.In(communityID).Ref(channelID).Members(ctx)
```

`Chats.Members` is for private or group chats. Use the channel object's
`Members` method for the ordered member list of a community channel:

```go
for _, entry := range channelMembers {
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

Community and message models returned by managers keep their client reference,
so common operations can be called directly:

```go
list, err := client.Communities.List(ctx)
if err != nil {
	return err
}

community := list[0]
channels, err := community.Channels().List(ctx)
if err != nil {
	return err
}

message, err := channels[0].Send(ctx, types.MessageSendParams{Content: "Hello"})
if err != nil {
	return err
}

if _, err := message.Reply(ctx, "Thanks!"); err != nil {
	return err
}
```

The rich object operations are:

| Object | Common operations |
| --- | --- |
| `User` | `Fetch` |
| `Community` | `Fetch`, `Channels`, `Members`, `Roles`, `Edit`, `Delete`, `Leave`, `CreateChannel`, `CreateRole`, `AddMember`, `Unban`, `SetDefaultPermissions` |
| `Channel` | `Fetch`, `Messages`, `Send`, `SendText`, `Search`, `PinnedMessages`, `Members`, `Edit`, `Delete`, `CreateInvite`, `Invites`, `DeleteInvite` |
| `Message` | `Fetch`, `Community`, `Channel`, `Member`, `Reply`, `ReplyWith`, `Edit`, `EditWith`, `Delete`, `React`, `Unreact`, `Pin`, `Unpin`, `SetPinned`, `Forward` |
| `Member` | `Fetch`, `Edit`, `SetRoles`, `AddRole`, `RemoveRole`, `Ban`, `Kick`, `Delete`, `Send`, `SendText` |
| `Role` | `Fetch`, `Edit`, `Delete`, `SetPermissions`, `AddPermissions`, `RemovePermissions` |

`types.MessageReply` is exposed as `Message.ReplyInfo` because `Message.Reply`
is now the reply operation. The original protobuf message remains available in
`Message.Raw`.

## Users and reactions

```go
user, err := client.Users.Lookup(ctx, "some-user")
profile, err := user.Profile(ctx)

message, err := client.Messages.In(types.SelfChat()).Fetch(ctx, messageID)
err = message.React(ctx, types.Emoji{Unicode: "👍"})
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
