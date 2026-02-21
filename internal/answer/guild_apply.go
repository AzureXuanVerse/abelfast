package answer

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	guildApplyResultSuccess = uint32(0)
	guildApplyResultFailure = uint32(1)
	guildApplyResultJoinCD  = uint32(4)
	guildApplyResultMaxed   = uint32(6)
	guildApplyResultFrozen  = uint32(4305)
	guildApplyResultFull    = uint32(4306)

	guildApplyContentLimit     = 20
	guildApplyOutstandingLimit = 10
)

func GuildApply(buffer *[]byte, client *connection.Client) (int, int, error) {
	request := &protobuf.CS_60005{}
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, 60006, err
	}

	response := &protobuf.SC_60006{Result: proto.Uint32(guildApplyResultFailure)}
	if client.Commander == nil {
		return client.SendMessage(60006, response)
	}

	guildID := request.GetId()
	if guildID == 0 {
		response.Result = proto.Uint32(guildApplyResultFrozen)
		return client.SendMessage(60006, response)
	}

	if _, err := orm.GetCommanderGuildMembership(client.Commander.CommanderID); err == nil {
		response.Result = proto.Uint32(guildApplyResultJoinCD)
		return client.SendMessage(60006, response)
	} else if !errors.Is(err, db.ErrNotFound) {
		return 0, 60006, err
	}

	waitTime, err := orm.GetCommanderGuildWaitTime(client.Commander.CommanderID)
	if err != nil {
		return 0, 60006, err
	}
	if waitTime > uint32(time.Now().Unix()) {
		response.Result = proto.Uint32(guildApplyResultJoinCD)
		return client.SendMessage(60006, response)
	}

	guild, err := orm.GetGuildByID(guildID)
	if errors.Is(err, db.ErrNotFound) {
		response.Result = proto.Uint32(guildApplyResultFrozen)
		return client.SendMessage(60006, response)
	}
	if err != nil {
		return 0, 60006, err
	}

	memberLimit, err := orm.GetGuildDataLevelMemberLimit(guild.Level)
	if err != nil {
		return 0, 60006, err
	}
	if guild.MemberCount >= memberLimit {
		response.Result = proto.Uint32(guildApplyResultFull)
		return client.SendMessage(60006, response)
	}

	alreadyApplied, err := orm.HasGuildJoinRequest(guildID, client.Commander.CommanderID)
	if err != nil {
		return 0, 60006, err
	}
	if !alreadyApplied {
		pendingCount, err := orm.CountGuildJoinRequestsByApplicant(client.Commander.CommanderID)
		if err != nil {
			return 0, 60006, err
		}
		if pendingCount >= guildApplyOutstandingLimit {
			response.Result = proto.Uint32(guildApplyResultMaxed)
			return client.SendMessage(60006, response)
		}
	}

	content := clampGuildApplyContent(strings.TrimSpace(request.GetContent()))
	if err := orm.UpsertGuildJoinRequest(guildID, client.Commander.CommanderID, content, time.Now().UTC()); err != nil {
		return 0, 60006, err
	}

	response.Result = proto.Uint32(guildApplyResultSuccess)
	return client.SendMessage(60006, response)
}

func clampGuildApplyContent(content string) string {
	if utf8.RuneCountInString(content) <= guildApplyContentLimit {
		return content
	}
	runes := []rune(content)
	return string(runes[:guildApplyContentLimit])
}
