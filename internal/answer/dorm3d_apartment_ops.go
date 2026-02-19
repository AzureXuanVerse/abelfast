package answer

import (
	"errors"
	"strings"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const dorm3dCallNameCooldownSeconds = 172800

func Dorm3dCollectionItem(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_28011
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 28012, err
	}
	if client.Commander == nil {
		return 0, 28012, errors.New("missing commander")
	}
	response := protobuf.SC_28012{Result: proto.Uint32(0)}
	if err := orm.MarkDorm3dCollection(client.Commander.CommanderID, payload.GetRoomId(), payload.GetCollectionId(), payload.GetShipGroup()); err != nil {
		response.Result = proto.Uint32(1)
	}
	return client.SendMessage(28012, &response)
}

func Dorm3dChangeSkin(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_28013
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 28014, err
	}
	if client.Commander == nil {
		return 0, 28014, errors.New("missing commander")
	}
	response := protobuf.SC_28014{Result: proto.Uint32(0)}
	if err := orm.ChangeDorm3dShipSkin(client.Commander.CommanderID, payload.GetShipGroup(), payload.GetSkin()); err != nil {
		response.Result = proto.Uint32(1)
	}
	return client.SendMessage(28014, &response)
}

func Dorm3dTalk(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_28015
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 28016, err
	}
	if client.Commander == nil {
		return 0, 28016, errors.New("missing commander")
	}
	response := protobuf.SC_28016{Result: proto.Uint32(0), DropList: []*protobuf.DROPINFO{}}
	if err := orm.MarkDorm3dDialogueSeen(client.Commander.CommanderID, payload.GetDialogId()); err != nil {
		response.Result = proto.Uint32(1)
	}
	return client.SendMessage(28016, &response)
}

func Dorm3dSetCall(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_28021
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 28022, err
	}
	if client.Commander == nil {
		return 0, 28022, errors.New("missing commander")
	}
	response := protobuf.SC_28022{Result: proto.Uint32(0)}
	name := strings.TrimSpace(payload.GetName())
	now := uint32(time.Now().Unix())
	if err := orm.SetDorm3dCallName(client.Commander.CommanderID, payload.GetShipGroup(), name, now, now+dorm3dCallNameCooldownSeconds); err != nil {
		response.Result = proto.Uint32(1)
	}
	return client.SendMessage(28022, &response)
}

func Dorm3dSetSkinHiddenParts(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_28038
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 28039, err
	}
	if client.Commander == nil {
		return 0, 28039, errors.New("missing commander")
	}
	response := protobuf.SC_28039{Result: proto.Uint32(0)}
	if err := orm.UpdateDorm3dSkinHiddenParts(client.Commander.CommanderID, payload.GetShipGroup(), payload.GetSkinId(), payload.GetHiddenParts()); err != nil {
		response.Result = proto.Uint32(1)
	}
	return client.SendMessage(28039, &response)
}
