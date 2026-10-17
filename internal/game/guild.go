package game

import (
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/ggmolly/belfast/internal/protocol"
)

// handleGuildInfo answers GetGuildInfoCommand. A guild id of 0 means the
// commander isn't in one (GuildProxy then exits it).
func (game *Game) handleGuildInfo(client *connection.Client, req *protobuf.CS_60037) error {
	return client.Send(&protobuf.SC_60000{Guild: &protobuf.GUILD_INFO{
		Base: &protobuf.GUILD_BASE_INFO{
			Id: proto.Uint32(0), Policy: proto.Uint32(0), Faction: proto.Uint32(0), Name: proto.String(""),
			Level: proto.Uint32(0), Announce: proto.String(""), Manifesto: proto.String(""), Exp: proto.Uint32(0),
			MemberCount: proto.Uint32(0), ChangeFactionCd: proto.Uint32(0), KickLeaderCd: proto.Uint32(0),
		},
		GuildEx: &protobuf.GUILD_EXPANSION_INFO{
			Capital:               proto.Uint32(0),
			ThisWeeklyTasks:       &protobuf.WEEKLY_TASK{Id: proto.Uint32(0), Progress: proto.Uint32(0), Monday_0Clock: proto.Uint32(0)},
			BenefitFinishTime:     proto.Uint32(0),
			RetreatCnt:            proto.Uint32(0),
			TechCancelCnt:         proto.Uint32(0),
			LastBenefitFinishTime: proto.Uint32(0),
			ActiveEventCnt:        proto.Uint32(0),
		},
	}})
}

// handlePublicGuildTech answers GetPublicGuildUserDataCommand: no guild
// technology outside a guild.
func (game *Game) handlePublicGuildTech(client *connection.Client, req *protobuf.CS_62100) error {
	return client.Send(&protobuf.SC_62101{})
}

// handleGuildUserInfo answers GuildGetUserInfoCommand / the public guild
// data: no donations, benefits or guild tech.
func (game *Game) handleGuildUserInfo(client *connection.Client, req *protobuf.CS_60102) error {
	return client.Send(protocol.FillRequired(&protobuf.SC_60103{}))
}
