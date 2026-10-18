package game

import (
	"context"
	"slices"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// guildChatCount is GuildConst.CHAT_LOG_MAX_COUNT: the guild chat messages
// kept, as many as GuildProxy keeps and GetGuildChatListCommand asks for.
const guildChatCount = 100

// guildChat is a stored guild chat message from commander.
func (game *Game) guildChat(row db.GuildChat, commander db.Commander) *protobuf.GUIDE_CHAT {
	return &protobuf.GUIDE_CHAT{
		Player:  game.playerInfoP60(commander),
		Content: proto.String(row.Content),
		Time:    proto.Uint32(row.Time),
	}
}

// handleGuildSendMsg takes GuildSendMsgCommand's guild chat message
// (CS_60007, from the guild's chat or the chat window's guild channel,
// already filtered by the client's wordVer), which has no reply: it is kept
// (the newest GuildConst.CHAT_LOG_MAX_COUNT of the guild) and pushed in
// SC_60008 to the members online, the sender included since GuildProxy only
// shows what SC_60008 brings. Guild:warpChatInfo looks the sender up among
// the members (its duty) and turns an emoji code into the emoji, so the text
// is kept as sent. Built from the Lua alone.
func (game *Game) handleGuildSendMsg(client *connection.Client, req *protobuf.CS_60007) error {
	ctx := context.Background()
	content := req.GetChat()
	if content == "" || !utf8.ValidString(content) {
		return nil
	}
	var msg *protobuf.SC_60008
	var members []uint32
	err := game.DB.InTx(ctx, func(queries *db.Queries) error {
		member, ok, err := guildMembership(ctx, queries, client.CommanderID)
		if err != nil || !ok {
			return err
		}
		sender, err := queries.GetCommander(ctx, client.CommanderID)
		if err != nil {
			return err
		}
		row, err := queries.AddGuildChat(ctx, db.AddGuildChatParams{
			GuildID:     member.GuildID,
			CommanderID: client.CommanderID,
			Content:     content,
			Time:        uint32(time.Now().Unix()),
		})
		if err != nil {
			return err
		}
		if err := queries.TrimGuildChat(ctx, db.TrimGuildChatParams{GuildID: member.GuildID, Keep: guildChatCount}); err != nil {
			return err
		}
		if members, err = queries.ListGuildMemberIDs(ctx, member.GuildID); err != nil {
			return err
		}
		msg = &protobuf.SC_60008{Chat: game.guildChat(row, sender)}
		return nil
	})
	if err != nil || msg == nil {
		return err
	}
	game.tellGuild(client, members, 0, msg)
	return nil
}

// handleGuildChatList answers GetGuildChatListCommand (CS_60100), sent once
// the guild is known (GuildProxy on SC_60000): the guild's latest count
// messages, oldest first as GuildProxy appends them. Only messages from
// current members are sent: Guild:warpChatInfo asserts the sender is one.
// Outside a guild the list is empty. Built from the Lua alone.
func (game *Game) handleGuildChatList(client *connection.Client, req *protobuf.CS_60100) error {
	ctx := context.Background()
	member, ok, err := guildMembership(ctx, game.DB.Queries, client.CommanderID)
	if err != nil {
		return err
	}
	reply := &protobuf.SC_60101{}
	if !ok {
		return client.Send(reply)
	}
	count := req.GetCount()
	if count == 0 || count > guildChatCount {
		count = guildChatCount
	}
	rows, err := game.DB.ListGuildChat(ctx, db.ListGuildChatParams{GuildID: member.GuildID, Count: int64(count)})
	if err != nil {
		return err
	}
	for _, row := range slices.Backward(rows) {
		reply.ChatList = append(reply.ChatList, game.guildChat(row.GuildChat, row.Commander))
	}
	return client.Send(reply)
}
