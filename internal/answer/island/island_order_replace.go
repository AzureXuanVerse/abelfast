package island

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	islandOrderReplaceOK      = uint32(0)
	islandOrderReplaceInvalid = uint32(1)
	islandOrderReplaceState   = uint32(2)
	islandOrderReplacePersist = uint32(3)
)

func IslandReplaceOrder(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21403
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21404, err
	}

	response := &protobuf.SC_21404{Result: proto.Uint32(islandOrderReplaceInvalid), Slot: emptyIslandOrderSlot()}
	slotID := payload.GetSlotId()
	if slotID == 0 {
		return client.SendMessage(21404, response)
	}

	dialogIDs, err := loadIslandRandomDialogIDs()
	if err != nil {
		response.Result = proto.Uint32(islandOrderReplacePersist)
		return client.SendMessage(21404, response)
	}
	refreshSeconds, _, err := loadIslandSetIntConfig("order_complete_refresh_time")
	if err != nil {
		response.Result = proto.Uint32(islandOrderReplacePersist)
		return client.SendMessage(21404, response)
	}
	now := uint32(time.Now().Unix())

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		slot, err := orm.GetIslandOrderSlotForUpdateTx(context.Background(), tx, client.Commander.CommanderID, slotID)
		if err != nil {
			if db.IsNotFound(err) {
				response.Result = proto.Uint32(islandOrderReplaceState)
				return nil
			}
			response.Result = proto.Uint32(islandOrderReplacePersist)
			return err
		}

		if slot.GetType() == 2 || slot.GetCurSelect() == 0 || slot.GetSubmitTime() > now {
			response.Result = proto.Uint32(islandOrderReplaceState)
			return nil
		}

		replaceIslandOrderSlot(slot, dialogIDs, now, refreshSeconds)
		if err := orm.UpsertIslandOrderSlotTx(context.Background(), tx, client.Commander.CommanderID, slot); err != nil {
			response.Result = proto.Uint32(islandOrderReplacePersist)
			return err
		}

		response.Result = proto.Uint32(islandOrderReplaceOK)
		response.Slot = slot
		return nil
	})
	if err != nil {
		return client.SendMessage(21404, response)
	}

	return client.SendMessage(21404, response)
}

func replaceIslandOrderSlot(slot *protobuf.PB_ISLAND_ORDER_SLOT, dialogIDs []uint32, now uint32, refreshSeconds uint32) {
	if len(dialogIDs) > 0 {
		index := 0
		for i := range dialogIDs {
			if dialogIDs[i] == slot.GetDialogId() {
				index = (i + 1) % len(dialogIDs)
				break
			}
		}
		slot.DialogId = proto.Uint32(dialogIDs[index])
	} else {
		slot.DialogId = proto.Uint32(slot.GetDialogId() + 1)
	}

	slot.StartTime = proto.Uint32(now)
	slot.SubmitTime = proto.Uint32(now + refreshSeconds)
	if len(slot.GetCost()) > 0 {
		first := slot.Cost[0]
		first.Num = proto.Uint32(first.GetNum() + 1)
	} else {
		slot.Cost = []*protobuf.PB_ISLAND_ITEM{{Id: proto.Uint32(1), Num: proto.Uint32(1)}}
	}
}

func emptyIslandOrderSlot() *protobuf.PB_ISLAND_ORDER_SLOT {
	return &protobuf.PB_ISLAND_ORDER_SLOT{
		Id:         proto.Uint32(0),
		Type:       proto.Uint32(0),
		CurSelect:  proto.Uint32(0),
		StartTime:  proto.Uint32(0),
		SubmitTime: proto.Uint32(0),
		Position:   proto.Uint32(0),
		DialogId:   proto.Uint32(0),
		Cost:       []*protobuf.PB_ISLAND_ITEM{},
		OrderLv:    proto.Uint32(0),
		ViewFlag:   proto.Uint32(0),
	}
}
