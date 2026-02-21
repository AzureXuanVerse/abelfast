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
	islandFirmSubmitOK           = uint32(0)
	islandFirmSubmitInvalid      = uint32(1)
	islandFirmSubmitState        = uint32(2)
	islandFirmSubmitInsufficient = uint32(3)
	islandFirmSubmitPersist      = uint32(4)
)

var (
	errIslandFirmSubmitInsufficientRollback = errors.New("island firm submit insufficient rollback")
	errIslandFirmSubmitInvalidRollback      = errors.New("island firm submit invalid rollback")
)

func IslandSubmitFirmOrder(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21414
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21415, err
	}

	response := &protobuf.SC_21415{Result: proto.Uint32(islandFirmSubmitInvalid), DropList: []*protobuf.DROPINFO{}}
	orderID := payload.GetOrderId()
	if orderID == 0 {
		return client.SendMessage(21415, response)
	}
	if err := ensureCommanderLoaded(client, "Island/FirmSubmit"); err != nil {
		response.Result = proto.Uint32(islandFirmSubmitPersist)
		return client.SendMessage(21415, response)
	}

	orderFavorGain, _, err := loadIslandSetIntConfig("order_favor")
	if err != nil {
		response.Result = proto.Uint32(islandFirmSubmitPersist)
		return client.SendMessage(21415, response)
	}

	now := uint32(time.Now().Unix())
	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		slot, err := orm.GetIslandOrderSlotForUpdateTx(context.Background(), tx, client.Commander.CommanderID, orderID)
		if err != nil {
			if db.IsNotFound(err) {
				response.Result = proto.Uint32(islandFirmSubmitState)
				return nil
			}
			response.Result = proto.Uint32(islandFirmSubmitPersist)
			return err
		}
		if slot.GetType() != 4 || slot.GetCurSelect() == 0 || slot.GetSubmitTime() > now {
			response.Result = proto.Uint32(islandFirmSubmitState)
			return nil
		}

		for _, cost := range slot.GetCost() {
			err := orm.ConsumeIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, cost.GetId(), cost.GetNum())
			if err != nil {
				if errors.Is(err, orm.ErrInsufficientIslandInventory) {
					response.Result = proto.Uint32(islandFirmSubmitInsufficient)
					return errIslandFirmSubmitInsufficientRollback
				}
				response.Result = proto.Uint32(islandFirmSubmitPersist)
				return err
			}
		}

		orderCfg, found, err := loadIslandFirmOrderConfig(slot.GetDialogId())
		if err != nil {
			response.Result = proto.Uint32(islandFirmSubmitPersist)
			return err
		}
		if !found {
			response.Result = proto.Uint32(islandFirmSubmitInvalid)
			return errIslandFirmSubmitInvalidRollback
		}

		drops := make([]*protobuf.DROPINFO, 0)
		if len(orderCfg.Award) >= 2 && orderCfg.Award[0] > 0 && orderCfg.Award[1] > 0 {
			drops = append(drops, newDropInfo(consts.DROP_TYPE_ISLAND_ITEM, orderCfg.Award[0], orderCfg.Award[1]))
		}
		if len(drops) > 0 {
			if err := applyIslandDropsTx(context.Background(), tx, client, drops); err != nil {
				response.Result = proto.Uint32(islandFirmSubmitPersist)
				return err
			}
		}
		if orderCfg.SeasonPTNum > 0 {
			if err := orm.AddIslandSeasonPTTx(context.Background(), tx, client.Commander.CommanderID, orderCfg.SeasonPTNum); err != nil {
				response.Result = proto.Uint32(islandFirmSubmitPersist)
				return err
			}
		}

		isActivityFirm := orderCfg.Type == 3 && orderCfg.ActivityID != 0
		if !isActivityFirm {
			if err := orm.AddIslandOrderFavorTx(context.Background(), tx, client.Commander.CommanderID, orderFavorGain); err != nil {
				response.Result = proto.Uint32(islandFirmSubmitPersist)
				return err
			}
		}
		if isActivityFirm && orderCfg.NextOrder == 0 && orderCfg.GroupID > 0 {
			if _, err := orm.AddIslandOrderActGroupTx(context.Background(), tx, client.Commander.CommanderID, orderCfg.ActivityID, orderCfg.GroupID); err != nil {
				response.Result = proto.Uint32(islandFirmSubmitPersist)
				return err
			}
		}

		if err := orm.DeleteIslandOrderSlotTx(context.Background(), tx, client.Commander.CommanderID, orderID); err != nil {
			response.Result = proto.Uint32(islandFirmSubmitPersist)
			return err
		}

		response.Result = proto.Uint32(islandFirmSubmitOK)
		response.DropList = mergeDropList(drops)
		return nil
	})
	if err != nil {
		if errors.Is(err, errIslandFirmSubmitInsufficientRollback) || errors.Is(err, errIslandFirmSubmitInvalidRollback) {
			return client.SendMessage(21415, response)
		}
		return client.SendMessage(21415, response)
	}

	return client.SendMessage(21415, response)
}
