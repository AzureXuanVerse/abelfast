package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func CommanderFriendList(buffer *[]byte, client *connection.Client) (int, int, error) {
	response := protobuf.SC_50000{
		FriendList:  []*protobuf.FRIEND_INFO{},
		RequestList: []*protobuf.MSG_INFO_P50{},
	}

	friendIDs, err := orm.ListCommanderFriendIDs(client.Commander.CommanderID)
	if err != nil || len(friendIDs) == 0 {
		return client.SendMessage(50000, &response)
	}

	profilesByID, err := orm.GetCommanderSocialProfilesByIDs(friendIDs)
	if err != nil {
		return client.SendMessage(50000, &response)
	}

	response.FriendList = make([]*protobuf.FRIEND_INFO, 0, len(friendIDs))
	for _, friendID := range friendIDs {
		profile, ok := profilesByID[friendID]
		if !ok {
			continue
		}
		response.FriendList = append(response.FriendList, buildFriendInfo(profile, client))
	}

	return client.SendMessage(50000, &response)
}
