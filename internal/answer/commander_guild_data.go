package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"

	"github.com/ggmolly/belfast/internal/protobuf"
)

func CommanderGuildData(buffer *[]byte, client *connection.Client) (int, int, error) {
	guild, _, err := orm.GetGuildForCommander(client.Commander.CommanderID)
	if err != nil && err != db.ErrNotFound {
		return 0, 60000, err
	}
	var members []*protobuf.MEMBER_INFO
	if guild != nil {
		guildMembers, err := orm.ListGuildMembers(guild.ID)
		if err != nil {
			return 0, 60000, err
		}
		members = make([]*protobuf.MEMBER_INFO, 0, len(guildMembers))
		for _, member := range guildMembers {
			members = append(members, buildGuildMemberInfo(member))
		}
	}
	response := protobuf.SC_60000{
		Guild: &protobuf.GUILD_INFO{
			Base:    buildGuildBaseInfo(guild),
			Member:  members,
			GuildEx: buildGuildExpansionInfo(guild),
		},
	}
	return client.SendMessage(60000, &response)
}
