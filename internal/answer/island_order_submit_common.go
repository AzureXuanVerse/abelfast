package answer

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
	islandCommonSubmitOK           = uint32(0)
	islandCommonSubmitInvalid      = uint32(1)
	islandCommonSubmitState        = uint32(2)
	islandCommonSubmitInsufficient = uint32(3)
	islandCommonSubmitPersist      = uint32(4)
)

var (
	errIslandCommonSubmitInsufficientRollback = errors.New("island common submit insufficient rollback")
	errIslandCommonSubmitInvalidRollback      = errors.New("island common submit invalid rollback")
)

func IslandSubmitCommonOrder(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21401
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21402, err
	}

	response := &protobuf.SC_21402{Result: proto.Uint32(islandCommonSubmitInvalid), Slot: emptyIslandOrderSlot(), DropList: []*protobuf.DROPINFO{}}
	slotID := payload.GetSlotId()
	if slotID == 0 {
		return client.SendMessage(21402, response)
	}
	if err := ensureCommanderLoaded(client, "Island/CommonSubmit"); err != nil {
		response.Result = proto.Uint32(islandCommonSubmitPersist)
		return client.SendMessage(21402, response)
	}

	orderFavorGain, _, err := loadIslandSetIntConfig("order_favor")
	if err != nil {
		response.Result = proto.Uint32(islandCommonSubmitPersist)
		return client.SendMessage(21402, response)
	}
	dialogIDs, err := loadIslandRandomDialogIDs()
	if err != nil {
		response.Result = proto.Uint32(islandCommonSubmitPersist)
		return client.SendMessage(21402, response)
	}
	refreshSeconds, _, err := loadIslandSetIntConfig("order_complete_refresh_time")
	if err != nil {
		response.Result = proto.Uint32(islandCommonSubmitPersist)
		return client.SendMessage(21402, response)
	}

	now := uint32(time.Now().Unix())
	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		slot, err := orm.GetIslandOrderSlotForUpdateTx(context.Background(), tx, client.Commander.CommanderID, slotID)
		if err != nil {
			if db.IsNotFound(err) {
				response.Result = proto.Uint32(islandCommonSubmitState)
				return nil
			}
			response.Result = proto.Uint32(islandCommonSubmitPersist)
			return err
		}
		if slot.GetType() != 1 || slot.GetCurSelect() == 0 || slot.GetSubmitTime() > now {
			response.Result = proto.Uint32(islandCommonSubmitState)
			return nil
		}

		for _, cost := range slot.GetCost() {
			err := orm.ConsumeIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, cost.GetId(), cost.GetNum())
			if err != nil {
				if errors.Is(err, orm.ErrInsufficientIslandInventory) {
					response.Result = proto.Uint32(islandCommonSubmitInsufficient)
					return errIslandCommonSubmitInsufficientRollback
				}
				response.Result = proto.Uint32(islandCommonSubmitPersist)
				return err
			}
		}

		price, found, err := loadIslandOrderPriceConfig(slot.GetOrderLv())
		if err != nil {
			response.Result = proto.Uint32(islandCommonSubmitPersist)
			return err
		}
		drop, ok := buildCommonOrderAwardDrop(price, found, slot.GetCurSelect())
		if !ok {
			response.Result = proto.Uint32(islandCommonSubmitInvalid)
			return errIslandCommonSubmitInvalidRollback
		}
		if err := applyIslandDropsTx(context.Background(), tx, client, []*protobuf.DROPINFO{drop}); err != nil {
			response.Result = proto.Uint32(islandCommonSubmitPersist)
			return err
		}

		state, err := orm.GetIslandOrderStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			response.Result = proto.Uint32(islandCommonSubmitPersist)
			return err
		}
		state.DailySlotNum++
		if err := orm.SaveIslandOrderStateTx(context.Background(), tx, state); err != nil {
			response.Result = proto.Uint32(islandCommonSubmitPersist)
			return err
		}
		if err := orm.AddIslandOrderFavorTx(context.Background(), tx, client.Commander.CommanderID, orderFavorGain); err != nil {
			response.Result = proto.Uint32(islandCommonSubmitPersist)
			return err
		}

		replaceIslandOrderSlot(slot, dialogIDs, now, refreshSeconds)
		if err := orm.UpsertIslandOrderSlotTx(context.Background(), tx, client.Commander.CommanderID, slot); err != nil {
			response.Result = proto.Uint32(islandCommonSubmitPersist)
			return err
		}

		response.Result = proto.Uint32(islandCommonSubmitOK)
		response.Slot = slot
		response.DropList = []*protobuf.DROPINFO{drop}
		return nil
	})
	if err != nil {
		if errors.Is(err, errIslandCommonSubmitInsufficientRollback) || errors.Is(err, errIslandCommonSubmitInvalidRollback) {
			return client.SendMessage(21402, response)
		}
		return client.SendMessage(21402, response)
	}

	return client.SendMessage(21402, response)
}

func buildCommonOrderAwardDrop(price *islandOrderPriceConfig, found bool, tendency uint32) (*protobuf.DROPINFO, bool) {
	if !found || price == nil {
		return nil, false
	}
	var award []uint32
	switch tendency {
	case 2:
		award = price.OrderEasyAward
	case 3:
		award = price.OrderAwardChallenge
	default:
		award = price.OrderAward
	}
	if len(award) < 2 || award[0] == 0 || award[1] == 0 {
		return nil, false
	}
	return newDropInfo(consts.DROP_TYPE_ISLAND_ITEM, award[0], award[1]), true
}
