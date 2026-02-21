package answer

import (
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

var exchangeShipAddOwnedShip = func(client *connection.Client, shipID uint32) (*orm.OwnedShip, error) {
	return client.Commander.AddShip(shipID)
}

var exchangeShipCommitCommander = func(client *connection.Client) error {
	return client.Commander.Commit()
}

func ExchangeShip(buffer *[]byte, client *connection.Client) (int, int, error) {
	var data protobuf.CS_12047
	if err := proto.Unmarshal(*buffer, &data); err != nil {
		return 0, 12048, err
	}
	response := protobuf.SC_12048{
		Result: proto.Uint32(1),
	}
	if client.Commander.ExchangeCount < 400 {
		return client.SendMessage(12048, &response)
	}

	if data.GetShipTid() != 105171 && data.GetShipTid() != 307081 {
		response.Result = proto.Uint32(2)
		return client.SendMessage(12048, &response)
	}

	client.Commander.ExchangeCount -= 400
	newShip, err := exchangeShipAddOwnedShip(client, data.GetShipTid())
	if err != nil {
		response.Result = proto.Uint32(3)
		return client.SendMessage(12048, &response)
	}
	response.Result = proto.Uint32(0)
	response.DropList = []*protobuf.DROPINFO{
		{
			Type:   proto.Uint32(consts.DROP_TYPE_SHIP),
			Id:     data.ShipTid,
			Number: proto.Uint32(1),
		},
	}
	if err := exchangeShipCommitCommander(client); err != nil {
		response.Result = proto.Uint32(3)
		return client.SendMessage(12048, &response)
	}

	if _, _, err := client.SendMessage(12048, &response); err != nil {
		return 0, 12048, err
	}
	if _, err := pushNewShips(client, []*orm.OwnedShip{newShip}); err != nil {
		return 0, 12042, err
	}
	return 0, 12048, nil
}
