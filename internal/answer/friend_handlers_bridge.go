package answer

import (
	answerfriend "github.com/ggmolly/belfast/internal/answer/friend"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	friendBlacklistResultSuccess       = uint32(0)
	friendBlacklistResultInvalidTarget = uint32(1)
	friendBlacklistResultNotFound      = uint32(2)

	friendOperationSuccess uint32 = 0
	friendOperationFailure uint32 = 1
	friendOperationMaxed   uint32 = 6
	maxFriendCount         uint32 = 50
)

func AcceptFriendRequest(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerfriend.AcceptFriendRequest(buffer, client)
}

func RejectFriendRequest(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerfriend.RejectFriendRequest(buffer, client)
}

func DeleteFriend(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerfriend.DeleteFriend(buffer, client)
}

func AddFriendBlacklist(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerfriend.AddFriendBlacklist(buffer, client)
}

func GetFriendBlacklist(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerfriend.GetFriendBlacklist(buffer, client)
}

func RelieveFriendBlacklist(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerfriend.RelieveFriendBlacklist(buffer, client)
}

func SendFriendRequest(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerfriend.SendFriendRequest(buffer, client)
}

func SearchFriend(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerfriend.SearchFriend(buffer, client)
}

func FriendSearchList(buffer *[]byte, client *connection.Client) (int, int, error) {
	return answerfriend.FriendSearchList(buffer, client)
}

func buildDisplayInfo(profile orm.CommanderSocialProfile) *protobuf.DISPLAYINFO {
	return answerfriend.BuildDisplayInfo(profile)
}

func buildFriendInfo(profile orm.CommanderSocialProfile, client *connection.Client) *protobuf.FRIEND_INFO {
	return answerfriend.BuildFriendInfo(profile, client)
}

func buildPlayerInfoP50(profile orm.CommanderSocialProfile) *protobuf.PLAYER_INFO_P50 {
	return answerfriend.BuildPlayerInfoP50(profile)
}

func buildDetailInfo(profile orm.CommanderSocialProfile, client *connection.Client, medalIDs []uint32) *protobuf.DETAIL_INFO {
	return answerfriend.BuildDetailInfo(profile, client, medalIDs)
}
