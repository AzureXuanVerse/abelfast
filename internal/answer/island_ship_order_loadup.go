package answer

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func IslandShipOrderLoadUp(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21416
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21417, err
	}

	response := &protobuf.SC_21417{Result: proto.Uint32(0), GetTime: proto.Uint32(0), DropList: []*protobuf.DROPINFO{}}
	if payload.GetShipSlotId() == 0 || len(payload.GetItemId()) == 0 {
		response.Result = proto.Uint32(1)
		return client.SendMessage(21417, response)
	}
	if err := ensureCommanderLoaded(client, "Island/ShipOrderLoadUp"); err != nil {
		response.Result = proto.Uint32(5)
		return client.SendMessage(21417, response)
	}

	coeff, found, err := loadIslandSetKeyValue("order_ship_award_coefficient")
	if err != nil {
		response.Result = proto.Uint32(5)
		return client.SendMessage(21417, response)
	}
	if !found || len(coeff) < 3 {
		response.Result = proto.Uint32(1)
		return client.SendMessage(21417, response)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		slot, err := orm.GetIslandShipOrderSlotForUpdateTx(context.Background(), tx, client.Commander.CommanderID, payload.GetShipSlotId())
		if err != nil {
			if db.IsNotFound(err) {
				response.Result = proto.Uint32(2)
				return nil
			}
			response.Result = proto.Uint32(5)
			return err
		}

		for _, requestedItemID := range payload.GetItemId() {
			matchedIndex := -1
			for i := range slot.CostList {
				if slot.CostList[i].ID == requestedItemID && slot.CostList[i].State == 0 {
					matchedIndex = i
					break
				}
			}
			if matchedIndex < 0 {
				response.Result = proto.Uint32(1)
				return nil
			}

			cost := slot.CostList[matchedIndex]
			if err := orm.ConsumeIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, cost.ID, cost.Num); err != nil {
				if db.IsNotFound(err) {
					response.Result = proto.Uint32(3)
					return nil
				}
				response.Result = proto.Uint32(5)
				return err
			}

			orderPrice, found, err := loadIslandItemOrderPrice(cost.ID)
			if err != nil {
				response.Result = proto.Uint32(5)
				return err
			}
			if !found {
				response.Result = proto.Uint32(1)
				return nil
			}
			baseValue := orderPrice * cost.Num
			mainAward := (baseValue * coeff[1]) / 100
			otherAward := (baseValue * coeff[2]) / 100
			if mainAward > 0 {
				response.DropList = append(response.DropList, newDropInfo(consts.DROP_TYPE_ISLAND_ITEM, coeff[0], mainAward))
				if err := orm.AddIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, coeff[0], mainAward); err != nil {
					response.Result = proto.Uint32(5)
					return err
				}
			}
			if otherAward > 0 {
				response.DropList = append(response.DropList, newDropInfo(consts.DROP_TYPE_RESOURCE, 2, otherAward))
				if err := client.Commander.AddResourceTx(context.Background(), tx, 2, otherAward); err != nil {
					response.Result = proto.Uint32(5)
					return err
				}
			}

			slot.CostList[matchedIndex].State = 1
		}

		complete := true
		for i := range slot.CostList {
			if slot.CostList[i].State == 0 {
				complete = false
				break
			}
		}
		if complete {
			slot.State = 1
			slot.GetTime = nowUnix()
			response.GetTime = proto.Uint32(slot.GetTime)
		}
		if err := orm.UpsertIslandShipOrderSlotTx(context.Background(), tx, slot); err != nil {
			response.Result = proto.Uint32(5)
			return err
		}
		return nil
	})
	if err != nil {
		return client.SendMessage(21417, response)
	}
	return client.SendMessage(21417, response)
}
