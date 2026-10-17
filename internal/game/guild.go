package game

import (
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// handleGuildInfo answers GetGuildInfoCommand. A guild id of 0 means the
// commander isn't in one (GuildProxy then exits it); the official server
// still fills policy, faction and level with 1.
func (game *Game) handleGuildInfo(client *connection.Client, req *protobuf.CS_60037) error {
	return client.Send(&protobuf.SC_60000{Guild: &protobuf.GUILD_INFO{
		Base: &protobuf.GUILD_BASE_INFO{
			Policy:  proto.Uint32(1),
			Faction: proto.Uint32(1),
			Level:   proto.Uint32(1),
		},
	}})
}

// publicGuildTechs are the public guild technologies a commander outside a
// guild sees (GetPublicGuildUserDataCommand), with the progress the official
// server reports for a new commander (captured).
var publicGuildTechs = [][2]uint32{{6012, 0}, {5009, 20}, {4009, 20}, {3030, 0}, {2030, 0}}

// handlePublicGuildTech answers GetPublicGuildUserDataCommand: the public
// guild technologies, none started.
func (game *Game) handlePublicGuildTech(client *connection.Client, req *protobuf.CS_62100) error {
	reply := &protobuf.SC_62101{}
	for _, tech := range publicGuildTechs {
		reply.Technologys = append(reply.Technologys, &protobuf.GUILD_TECHNOLOGY{
			Id: proto.Uint32(tech[0]), Progress: proto.Uint32(tech[1]),
		})
	}
	return client.Send(reply)
}

// handleGuildUserInfo answers GuildGetUserInfoCommand: no donations or
// benefits yet, today's donation tasks (guild_donate_task ids, captured).
func (game *Game) handleGuildUserInfo(client *connection.Client, req *protobuf.CS_60102) error {
	return client.Send(&protobuf.SC_60103{UserInfo: &protobuf.USER_GUILD_INFO{
		DonateTasks: []uint32{1, 2, 20},
	}})
}

// handleGuildShop answers GetGuildShopCommand: outside a guild the shop is
// empty and never refreshes.
func (game *Game) handleGuildShop(client *connection.Client, req *protobuf.CS_60033) error {
	return client.Send(&protobuf.SC_60034{Result: proto.Uint32(0)})
}
