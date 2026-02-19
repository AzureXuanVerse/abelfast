package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const friendRecommendationLimit = 20

func FriendSearchList(buffer *[]byte, client *connection.Client) (int, int, error) {
	request := &protobuf.CS_50014{}
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, 50015, err
	}

	response := &protobuf.SC_50015{PlayerList: []*protobuf.PLAYER_INFO_P50{}}
	if request.GetType() != 0 {
		return client.SendMessage(50015, response)
	}

	profiles, err := orm.ListCommanderSocialProfilesForRecommendations(client.Commander.CommanderID, friendRecommendationLimit)
	if err != nil {
		return client.SendMessage(50015, response)
	}

	response.PlayerList = make([]*protobuf.PLAYER_INFO_P50, 0, len(profiles))
	for _, profile := range profiles {
		response.PlayerList = append(response.PlayerList, buildPlayerInfoP50(profile))
	}

	return client.SendMessage(50015, response)
}
