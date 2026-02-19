package answer

import (
	"errors"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func Dorm3dChatSetBackground(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_28030
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 28031, err
	}
	if client.Commander == nil {
		return 0, 28031, errors.New("missing commander")
	}
	response := protobuf.SC_28031{Result: proto.Uint32(0)}
	if err := orm.UpdateDorm3dInsBackground(client.Commander.CommanderID, payload.GetShipId(), payload.GetBackId()); err != nil {
		response.Result = proto.Uint32(1)
	}
	return client.SendMessage(28031, &response)
}

func Dorm3dChatSetCare(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_28032
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 28033, err
	}
	if client.Commander == nil {
		return 0, 28033, errors.New("missing commander")
	}
	response := protobuf.SC_28033{Result: proto.Uint32(0)}
	if err := orm.UpdateDorm3dInsCareFlag(client.Commander.CommanderID, payload.GetShipId(), payload.GetValue()); err != nil {
		response.Result = proto.Uint32(1)
	}
	return client.SendMessage(28033, &response)
}

func Dorm3dInstagramSetTopic(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_28034
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 28035, err
	}
	if client.Commander == nil {
		return 0, 28035, errors.New("missing commander")
	}
	response := protobuf.SC_28035{Result: proto.Uint32(0)}
	if err := orm.SetDorm3dCurrentCommID(client.Commander.CommanderID, payload.GetShipId(), payload.GetCommId()); err != nil {
		response.Result = proto.Uint32(1)
	}
	return client.SendMessage(28035, &response)
}

func Dorm3dRecordVisit(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_28036
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 28037, err
	}
	if client.Commander == nil {
		return 0, 28037, errors.New("missing commander")
	}
	response := protobuf.SC_28037{Result: proto.Uint32(0)}
	if err := orm.UpdateDorm3dVisitTime(client.Commander.CommanderID, payload.GetShipId(), uint32(time.Now().Unix())); err != nil {
		response.Result = proto.Uint32(1)
	}
	return client.SendMessage(28037, &response)
}
