package island

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	islandShipOrderExchangeOK      = uint32(0)
	islandShipOrderExchangeSlotErr = uint32(1)
	islandShipOrderExchangeAppoint = uint32(2)
	islandShipOrderExchangePersist = uint32(4)
)

func IslandExchangeShipOrderDelegate(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21431
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21432, err
	}

	response := &protobuf.SC_21432{Result: proto.Uint32(islandShipOrderExchangeSlotErr), Appoint: emptyShipOrderAppoint()}
	slotID := payload.GetSlotId()
	appointID := payload.GetAppointId()
	if slotID == 0 || appointID == 0 {
		return client.SendMessage(21432, response)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		slot, err := orm.GetIslandShipOrderSlotForUpdateTx(context.Background(), tx, client.Commander.CommanderID, slotID)
		if err != nil {
			if db.IsNotFound(err) {
				response.Result = proto.Uint32(islandShipOrderExchangeSlotErr)
				return nil
			}
			response.Result = proto.Uint32(islandShipOrderExchangePersist)
			return err
		}
		appoint, err := orm.GetIslandShipOrderAppointForUpdateTx(context.Background(), tx, client.Commander.CommanderID, appointID)
		if err != nil {
			if db.IsNotFound(err) {
				response.Result = proto.Uint32(islandShipOrderExchangeAppoint)
				return nil
			}
			response.Result = proto.Uint32(islandShipOrderExchangePersist)
			return err
		}

		if slotHasAnyLoadedCost(slot) {
			slot.FinishNum = proto.Uint32(slot.GetFinishNum() + 1)
		}
		slot.Cost = convertAppointCostToShipLoad(appoint.GetCost())
		slot.Reward = cloneIslandItems(appoint.GetReward())

		if err := orm.UpsertIslandShipOrderSlotDataTx(context.Background(), tx, client.Commander.CommanderID, slot); err != nil {
			response.Result = proto.Uint32(islandShipOrderExchangePersist)
			return err
		}
		if err := orm.DeleteIslandShipOrderAppointTx(context.Background(), tx, client.Commander.CommanderID, appointID); err != nil {
			response.Result = proto.Uint32(islandShipOrderExchangePersist)
			return err
		}

		newAppoint := &protobuf.PB_SHIP_ORDER_APPOINT{
			Id:       proto.Uint32(appoint.GetId() + 1000000),
			ViewTime: proto.Uint32(appoint.GetViewTime()),
			Cost:     cloneIslandItems(appoint.GetCost()),
			Reward:   cloneIslandItems(appoint.GetReward()),
		}
		if err := orm.UpsertIslandShipOrderAppointTx(context.Background(), tx, client.Commander.CommanderID, newAppoint); err != nil {
			response.Result = proto.Uint32(islandShipOrderExchangePersist)
			return err
		}

		response.Result = proto.Uint32(islandShipOrderExchangeOK)
		response.Appoint = newAppoint
		return nil
	})
	if err != nil {
		return client.SendMessage(21432, response)
	}

	return client.SendMessage(21432, response)
}

func slotHasAnyLoadedCost(slot *protobuf.PB_ISLAND_ORDER_SHIP_SLOT) bool {
	for _, cost := range slot.GetCost() {
		if cost.GetState() != 0 {
			return true
		}
	}
	return false
}

func convertAppointCostToShipLoad(values []*protobuf.PB_ISLAND_ITEM) []*protobuf.PB_ISLAND_ORDER_SHIP_LOAD {
	loads := make([]*protobuf.PB_ISLAND_ORDER_SHIP_LOAD, 0, len(values))
	for _, value := range values {
		loads = append(loads, &protobuf.PB_ISLAND_ORDER_SHIP_LOAD{
			Id:    proto.Uint32(value.GetId()),
			Num:   proto.Uint32(value.GetNum()),
			State: proto.Uint32(0),
		})
	}
	return loads
}

func cloneIslandItems(values []*protobuf.PB_ISLAND_ITEM) []*protobuf.PB_ISLAND_ITEM {
	out := make([]*protobuf.PB_ISLAND_ITEM, 0, len(values))
	for _, value := range values {
		out = append(out, &protobuf.PB_ISLAND_ITEM{Id: proto.Uint32(value.GetId()), Num: proto.Uint32(value.GetNum())})
	}
	return out
}

func emptyShipOrderAppoint() *protobuf.PB_SHIP_ORDER_APPOINT {
	return &protobuf.PB_SHIP_ORDER_APPOINT{Id: proto.Uint32(0), ViewTime: proto.Uint32(0), Cost: []*protobuf.PB_ISLAND_ITEM{}, Reward: []*protobuf.PB_ISLAND_ITEM{}}
}
