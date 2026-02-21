package guild

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GuildRefreshAssaultRecommendationsCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_61035
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 61036, err
	}

	response := &protobuf.SC_61036{Recommends: []*protobuf.TEAM_CELL{}}
	if client.Commander == nil {
		return client.SendMessage(61036, response)
	}
	if payload.GetType() != 0 {
		return client.SendMessage(61036, response)
	}

	ctx, err := activeGuildEventContext(client.Commander.CommanderID)
	if err != nil {
		if isGuildEventContextError(err) {
			return client.SendMessage(61036, response)
		}
		return 0, 61036, err
	}

	recommendations, err := orm.ListGuildAssaultRecommendations(ctx.GuildID)
	if err != nil {
		return 0, 61036, err
	}
	response.Recommends = make([]*protobuf.TEAM_CELL, 0, len(recommendations))
	for _, recommendation := range recommendations {
		response.Recommends = append(response.Recommends, &protobuf.TEAM_CELL{
			UserId: proto.Uint32(recommendation.CommanderID),
			ShipId: proto.Uint32(recommendation.ShipID),
		})
	}

	return client.SendMessage(61036, response)
}
