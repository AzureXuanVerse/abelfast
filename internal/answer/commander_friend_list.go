package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func CommanderFriendList(buffer *[]byte, client *connection.Client) (int, int, error) {
	response := protobuf.SC_50000{
		FriendList:  []*protobuf.FRIEND_INFO{},
		RequestList: []*protobuf.MSG_INFO_P50{},
	}

	friendIDs, err := orm.ListCommanderFriendIDs(client.Commander.CommanderID)
	if err == nil && len(friendIDs) > 0 {
		profilesByID, profileErr := orm.GetCommanderSocialProfilesByIDs(friendIDs)
		if profileErr == nil {
			response.FriendList = make([]*protobuf.FRIEND_INFO, 0, len(friendIDs))
			for _, friendID := range friendIDs {
				profile, ok := profilesByID[friendID]
				if !ok {
					continue
				}
				response.FriendList = append(response.FriendList, buildFriendInfo(profile, client))
			}
		}
	}
	if len(response.FriendList) == 0 {
		legacyProfiles, profileErr := orm.ListFriendProfiles(client.Commander.CommanderID)
		if profileErr == nil && len(legacyProfiles) > 0 {
			response.FriendList = make([]*protobuf.FRIEND_INFO, 0, len(legacyProfiles))
			for _, profile := range legacyProfiles {
				response.FriendList = append(response.FriendList, buildFriendInfo(profile, client))
			}
		}
	}

	requests, err := orm.ListIncomingFriendRequests(client.Commander.CommanderID)
	if err == nil {
		response.RequestList = make([]*protobuf.MSG_INFO_P50, 0, len(requests))
		for _, request := range requests {
			response.RequestList = append(response.RequestList, &protobuf.MSG_INFO_P50{
				Timestamp: proto.Uint32(uint32(request.CreatedAt.UTC().Unix())),
				Player:    buildPlayerInfoP50(request.Requester),
				Content:   proto.String(request.Content),
			})
		}
	}

	return client.SendMessage(50000, &response)
}
