package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func CommanderFriendBatchGet(buffer *[]byte, client *connection.Client) (int, int, error) {
	request := &protobuf.CS_50018{}
	if err := proto.Unmarshal(*buffer, request); err != nil {
		return 0, 50019, err
	}

	response := &protobuf.SC_50019{UserList: []*protobuf.FRIEND_INFO{}}
	ids := normalizeCommanderIDs(request.GetUserIdList(), 0)
	if len(ids) == 0 {
		return client.SendMessage(50019, response)
	}

	profilesByID, err := orm.GetCommanderSocialProfilesByIDs(ids)
	if err != nil {
		return client.SendMessage(50019, response)
	}

	response.UserList = make([]*protobuf.FRIEND_INFO, 0, len(ids))
	for _, commanderID := range ids {
		profile, ok := profilesByID[commanderID]
		if !ok {
			continue
		}
		response.UserList = append(response.UserList, buildFriendInfo(profile, client))
	}

	return client.SendMessage(50019, response)
}
