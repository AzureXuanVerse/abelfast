package answer

import (
	"errors"
	"fmt"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	friendDMResultSuccess = uint32(0)
	friendDMResultOffline = uint32(28)
	friendDMResultFailed  = uint32(1)
)

func SendFriendMessage(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_50105
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 50106, fmt.Errorf("invalid CS_50105 packet: %w", err)
	}

	isFriend, err := orm.IsFriend(client.Commander.CommanderID, request.GetId())
	if err != nil {
		return 0, 50106, err
	}
	if !isFriend {
		response := protobuf.SC_50106{Result: proto.Uint32(friendDMResultFailed)}
		return client.SendMessage(50106, &response)
	}

	if client.Server == nil {
		response := protobuf.SC_50106{Result: proto.Uint32(friendDMResultOffline)}
		return client.SendMessage(50106, &response)
	}
	targetClient, ok := client.Server.FindClientByCommander(request.GetId())
	if !ok {
		response := protobuf.SC_50106{Result: proto.Uint32(friendDMResultOffline)}
		return client.SendMessage(50106, &response)
	}

	now := uint32(time.Now().Unix())
	if _, err := orm.CreateFriendDirectMessage(client.Commander.CommanderID, request.GetId(), request.GetContent(), now); err != nil {
		response := protobuf.SC_50106{Result: proto.Uint32(friendDMResultFailed)}
		return client.SendMessage(50106, &response)
	}

	msg := &protobuf.SC_50104{
		Msg: &protobuf.MSG_INFO_P50{
			Timestamp: proto.Uint32(now),
			Player:    buildSocialPlayerInfo(client.Commander),
			Content:   proto.String(request.GetContent()),
		},
	}
	_, _, _ = targetClient.SendMessage(50104, msg)

	response := protobuf.SC_50106{Result: proto.Uint32(friendDMResultSuccess)}
	return client.SendMessage(50106, &response)
}

func ReportPlayer(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_50111
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 50112, fmt.Errorf("invalid CS_50111 packet: %w", err)
	}

	if _, err := orm.CreatePlayerInform(client.Commander.CommanderID, request.GetId(), request.GetInfo(), request.GetContent(), uint32(time.Now().Unix())); err != nil {
		response := protobuf.SC_50112{Result: proto.Uint32(1)}
		return client.SendMessage(50112, &response)
	}

	response := protobuf.SC_50112{Result: proto.Uint32(0)}
	return client.SendMessage(50112, &response)
}

func GetThemeTemplatePlayerInfo(buffer *[]byte, client *connection.Client) (int, int, error) {
	var request protobuf.CS_50113
	if err := proto.Unmarshal(*buffer, &request); err != nil {
		return 0, 50114, fmt.Errorf("invalid CS_50113 packet: %w", err)
	}

	commander, err := orm.LoadCommanderSocialDisplay(request.GetUserId())
	if err != nil {
		if !errors.Is(err, db.ErrNotFound) {
			return 0, 50114, err
		}
		response := protobuf.SC_50114{
			Result: proto.Uint32(1),
			Player: emptySocialPlayerInfo(),
		}
		return client.SendMessage(50114, &response)
	}

	response := protobuf.SC_50114{
		Result: proto.Uint32(0),
		Player: buildSocialPlayerInfo(commander),
	}
	return client.SendMessage(50114, &response)
}

func buildSocialPlayerInfo(commander *orm.Commander) *protobuf.PLAYER_INFO_P50 {
	return &protobuf.PLAYER_INFO_P50{
		Id:   proto.Uint32(commander.CommanderID),
		Name: proto.String(commander.Name),
		Lv:   proto.Uint32(uint32(commander.Level)),
		Display: &protobuf.DISPLAYINFO{
			Icon:          proto.Uint32(commander.DisplayIconID),
			Skin:          proto.Uint32(commander.DisplaySkinID),
			IconFrame:     proto.Uint32(commander.SelectedIconFrameID),
			ChatFrame:     proto.Uint32(commander.SelectedChatFrameID),
			IconTheme:     proto.Uint32(commander.DisplayIconThemeID),
			MarryFlag:     proto.Uint32(0),
			TransformFlag: proto.Uint32(0),
		},
	}
}

func emptySocialPlayerInfo() *protobuf.PLAYER_INFO_P50 {
	return &protobuf.PLAYER_INFO_P50{
		Id:   proto.Uint32(0),
		Name: proto.String(""),
		Lv:   proto.Uint32(0),
	}
}
