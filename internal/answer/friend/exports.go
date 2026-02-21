package friend

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func BuildDisplayInfo(profile orm.CommanderSocialProfile) *protobuf.DISPLAYINFO {
	return buildDisplayInfo(profile)
}

func BuildFriendInfo(profile orm.CommanderSocialProfile, client *connection.Client) *protobuf.FRIEND_INFO {
	return buildFriendInfo(profile, client)
}

func BuildPlayerInfoP50(profile orm.CommanderSocialProfile) *protobuf.PLAYER_INFO_P50 {
	return buildPlayerInfoP50(profile)
}

func BuildDetailInfo(profile orm.CommanderSocialProfile, client *connection.Client, medalIDs []uint32) *protobuf.DETAIL_INFO {
	return buildDetailInfo(profile, client, medalIDs)
}
