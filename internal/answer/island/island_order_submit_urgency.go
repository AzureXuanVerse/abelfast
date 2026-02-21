package island

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	islandUrgencySubmitOK           = uint32(0)
	islandUrgencySubmitInvalid      = uint32(1)
	islandUrgencySubmitState        = uint32(2)
	islandUrgencySubmitInsufficient = uint32(3)
	islandUrgencySubmitPersist      = uint32(4)
)

var errIslandUrgencySubmitInsufficientRollback = errors.New("island urgency submit insufficient rollback")

func IslandSubmitUrgencyOrder(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21405
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21406, err
	}

	response := &protobuf.SC_21406{Result: proto.Uint32(islandUrgencySubmitInvalid), DropList: []*protobuf.DROPINFO{}}
	slotID := payload.GetSlotId()
	if slotID == 0 {
		return client.SendMessage(21406, response)
	}
	if err := ensureCommanderLoaded(client, "Island/UrgencySubmit"); err != nil {
		response.Result = proto.Uint32(islandUrgencySubmitPersist)
		return client.SendMessage(21406, response)
	}

	now := uint32(time.Now().Unix())
	orderFavorGain, _, err := loadIslandSetIntConfig("order_favor")
	if err != nil {
		response.Result = proto.Uint32(islandUrgencySubmitPersist)
		return client.SendMessage(21406, response)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		slot, err := orm.GetIslandOrderSlotForUpdateTx(context.Background(), tx, client.Commander.CommanderID, slotID)
		if err != nil {
			if db.IsNotFound(err) {
				response.Result = proto.Uint32(islandUrgencySubmitState)
				return nil
			}
			response.Result = proto.Uint32(islandUrgencySubmitPersist)
			return err
		}
		if slot.GetType() != 2 {
			response.Result = proto.Uint32(islandUrgencySubmitState)
			return nil
		}
		if slot.GetSubmitTime() > now {
			response.Result = proto.Uint32(islandUrgencySubmitState)
			return nil
		}

		for _, cost := range slot.GetCost() {
			err := orm.ConsumeIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, cost.GetId(), cost.GetNum())
			if err != nil {
				if errors.Is(err, orm.ErrInsufficientIslandInventory) {
					response.Result = proto.Uint32(islandUrgencySubmitInsufficient)
					return errIslandUrgencySubmitInsufficientRollback
				}
				response.Result = proto.Uint32(islandUrgencySubmitPersist)
				return err
			}
		}

		price, found, err := loadIslandOrderPriceConfig(slot.GetOrderLv())
		if err != nil {
			response.Result = proto.Uint32(islandUrgencySubmitPersist)
			return err
		}
		if !found || len(price.OrderAwardSpecial) < 2 {
			response.Result = proto.Uint32(islandUrgencySubmitInvalid)
			return nil
		}

		drop := newDropInfo(consts.DROP_TYPE_ISLAND_ITEM, price.OrderAwardSpecial[0], price.OrderAwardSpecial[1])
		if err := orm.AddIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, drop.GetId(), drop.GetNumber()); err != nil {
			response.Result = proto.Uint32(islandUrgencySubmitPersist)
			return err
		}

		state, err := orm.GetIslandOrderStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			response.Result = proto.Uint32(islandUrgencySubmitPersist)
			return err
		}
		state.UrgencyFinishCount++
		if err := orm.SaveIslandOrderStateTx(context.Background(), tx, state); err != nil {
			response.Result = proto.Uint32(islandUrgencySubmitPersist)
			return err
		}
		if err := orm.AddIslandOrderFavorTx(context.Background(), tx, client.Commander.CommanderID, orderFavorGain); err != nil {
			response.Result = proto.Uint32(islandUrgencySubmitPersist)
			return err
		}
		if err := orm.DeleteIslandOrderSlotTx(context.Background(), tx, client.Commander.CommanderID, slotID); err != nil {
			response.Result = proto.Uint32(islandUrgencySubmitPersist)
			return err
		}

		response.Result = proto.Uint32(islandUrgencySubmitOK)
		response.DropList = []*protobuf.DROPINFO{drop}
		return nil
	})
	if err != nil {
		if errors.Is(err, errIslandUrgencySubmitInsufficientRollback) {
			return client.SendMessage(21406, response)
		}
		return client.SendMessage(21406, response)
	}

	return client.SendMessage(21406, response)
}
