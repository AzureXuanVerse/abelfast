package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func buildDisplayInfo(profile orm.CommanderSocialProfile) *protobuf.DISPLAYINFO {
	return &protobuf.DISPLAYINFO{
		Icon:          proto.Uint32(profile.DisplayIconID),
		Skin:          proto.Uint32(profile.DisplaySkinID),
		IconFrame:     proto.Uint32(profile.SelectedIconFrameID),
		ChatFrame:     proto.Uint32(profile.SelectedChatFrameID),
		IconTheme:     proto.Uint32(profile.DisplayIconThemeID),
		MarryFlag:     proto.Uint32(0),
		TransformFlag: proto.Uint32(0),
	}
}

func onlineState(targetCommanderID uint32, client *connection.Client, profile orm.CommanderSocialProfile) (uint32, uint32) {
	if client != nil && client.Server != nil {
		if _, ok := client.Server.FindClientByCommander(targetCommanderID); ok {
			return 1, 0
		}
	}
	return 0, profile.LastLoginUnix
}

func buildFriendInfo(profile orm.CommanderSocialProfile, client *connection.Client) *protobuf.FRIEND_INFO {
	online, preOnline := onlineState(profile.CommanderID, client, profile)
	return &protobuf.FRIEND_INFO{
		Id:            proto.Uint32(profile.CommanderID),
		Name:          proto.String(profile.Name),
		Lv:            proto.Uint32(profile.Level),
		Adv:           proto.String(profile.Manifesto),
		Online:        proto.Uint32(online),
		PreOnlineTime: proto.Uint32(preOnline),
		Display:       buildDisplayInfo(profile),
	}
}

func buildPlayerInfoP50(profile orm.CommanderSocialProfile) *protobuf.PLAYER_INFO_P50 {
	return &protobuf.PLAYER_INFO_P50{
		Id:      proto.Uint32(profile.CommanderID),
		Name:    proto.String(profile.Name),
		Lv:      proto.Uint32(profile.Level),
		Display: buildDisplayInfo(profile),
	}
}

func buildDetailInfo(profile orm.CommanderSocialProfile, client *connection.Client, medalIDs []uint32) *protobuf.DETAIL_INFO {
	online, preOnline := onlineState(profile.CommanderID, client, profile)
	return &protobuf.DETAIL_INFO{
		Id:                 proto.Uint32(profile.CommanderID),
		Name:               proto.String(profile.Name),
		Title:              proto.Uint32(0),
		Lv:                 proto.Uint32(profile.Level),
		ShipCount:          proto.Uint32(profile.ShipCount),
		CollectionCount:    proto.Uint32(profile.CollectionCount),
		PvpAttackCount:     proto.Uint32(profile.PvpAttackCount),
		PvpWinCount:        proto.Uint32(profile.PvpWinCount),
		CollectAttackCount: proto.Uint32(profile.CollectAttackCount),
		AttackCount:        proto.Uint32(0),
		WinCount:           proto.Uint32(0),
		Adv:                proto.String(profile.Manifesto),
		Online:             proto.Uint32(online),
		PreOnlineTime:      proto.Uint32(preOnline),
		Score:              proto.Uint32(0),
		MedalId:            medalIDs,
		Display:            buildDisplayInfo(profile),
	}
}
