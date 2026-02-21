package guild

import (
	"strconv"
	"strings"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GuildSearch(buffer *[]byte, client *connection.Client) (int, int, error) {
	request := &protobuf.CS_60028{}
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, 60029, err
	}

	response := &protobuf.SC_60029{
		Result: proto.Uint32(guildResultFailure),
		Guild:  []*protobuf.GUILD_SIMPLE_INFO{},
	}

	keyword := strings.TrimSpace(request.GetKeyword())
	if keyword == "" {
		return client.SendMessage(60029, response)
	}

	var (
		entries []orm.GuildDirectoryEntry
		err     error
	)

	switch request.GetType() {
	case 0:
		guildID, parseErr := strconv.ParseUint(keyword, 10, 32)
		if parseErr != nil {
			return client.SendMessage(60029, response)
		}
		entries, err = orm.SearchGuildDirectoryByID(uint32(guildID))
	case 1:
		entries, err = orm.SearchGuildDirectoryByName(keyword)
	default:
		return client.SendMessage(60029, response)
	}
	if err != nil {
		return 0, 60029, err
	}
	if len(entries) == 0 {
		return client.SendMessage(60029, response)
	}

	response.Guild = make([]*protobuf.GUILD_SIMPLE_INFO, 0, len(entries))
	for _, entry := range entries {
		response.Guild = append(response.Guild, buildGuildSimpleInfo(entry))
	}
	response.Result = proto.Uint32(guildResultSuccess)
	return client.SendMessage(60029, response)
}

func buildGuildSimpleInfo(entry orm.GuildDirectoryEntry) *protobuf.GUILD_SIMPLE_INFO {
	return &protobuf.GUILD_SIMPLE_INFO{
		Base: buildGuildBaseInfo(&entry.Guild),
		Leader: &protobuf.PLAYER_INFO_P60{
			Id:      proto.Uint32(entry.Leader.CommanderID),
			Name:    proto.String(entry.Leader.Name),
			Lv:      proto.Uint32(entry.Leader.Level),
			Display: buildDisplayInfo(entry.Leader),
		},
		TechSeat: proto.Uint32(entry.TechSeat),
	}
}
