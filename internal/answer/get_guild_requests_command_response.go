package answer

import (
	"errors"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GetGuildRequestsCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	request := &protobuf.CS_60003{}
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, 60004, err
	}

	response := &protobuf.SC_60004{RequestList: []*protobuf.MSG_INFO_P60{}}
	if client.Commander == nil {
		return client.SendMessage(60004, response)
	}

	membership, err := orm.GetCommanderGuildMembership(client.Commander.CommanderID)
	if errors.Is(err, db.ErrNotFound) {
		return client.SendMessage(60004, response)
	}
	if err != nil {
		return 0, 60004, err
	}
	if request.GetId() != 0 && request.GetId() != membership.GuildID {
		return client.SendMessage(60004, response)
	}

	requests, err := orm.ListGuildJoinRequests(membership.GuildID)
	if err != nil {
		return 0, 60004, err
	}

	response.RequestList = make([]*protobuf.MSG_INFO_P60, 0, len(requests))
	for _, guildRequest := range requests {
		response.RequestList = append(response.RequestList, &protobuf.MSG_INFO_P60{
			Timestamp: proto.Uint32(uint32(guildRequest.RequestedAt.Unix())),
			Player: &protobuf.PLAYER_INFO_P60{
				Id:      proto.Uint32(guildRequest.Applicant.CommanderID),
				Name:    proto.String(guildRequest.Applicant.Name),
				Lv:      proto.Uint32(guildRequest.Applicant.Level),
				Display: buildDisplayInfo(guildRequest.Applicant),
			},
			Content: proto.String(guildRequest.Content),
		})
	}

	return client.SendMessage(60004, response)
}
