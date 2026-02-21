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

func IslandCollectSlot(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21507
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21508, err
	}

	response := &protobuf.SC_21508{Result: proto.Uint32(1), RefreshTime: proto.Uint32(0), DropList: []*protobuf.DROPINFO{}}
	slotType := payload.GetType()
	if payload.GetBuildId() == 0 || payload.GetAreaId() == 0 || (slotType != 1 && slotType != 2) {
		return client.SendMessage(21508, response)
	}
	if err := ensureCommanderLoaded(client, "Island/CollectSlot"); err != nil {
		return client.SendMessage(21508, response)
	}

	slotsCfg, err := loadIslandProductionSlots()
	if err != nil {
		return client.SendMessage(21508, response)
	}
	slotCfg, ok := slotsCfg[payload.GetAreaId()]
	if !ok || slotCfg.Place != payload.GetBuildId() || slotCfg.Type != slotType {
		return client.SendMessage(21508, response)
	}
	if len(slotCfg.Formula) == 0 || slotCfg.Formula[0] == 0 {
		return client.SendMessage(21508, response)
	}

	formula, exists, err := loadIslandHandPlantFormula(slotCfg.Formula[0])
	if err != nil || !exists {
		return client.SendMessage(21508, response)
	}

	now := nowUnix()
	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.GetIslandSlotCollectStateTx(context.Background(), tx, client.Commander.CommanderID, payload.GetBuildId(), payload.GetAreaId(), slotType)
		if err != nil {
			return err
		}
		if state == nil {
			state = &orm.IslandSlotCollectState{
				CommanderID: client.Commander.CommanderID,
				BuildID:     payload.GetBuildId(),
				AreaID:      payload.GetAreaId(),
				SlotType:    slotType,
			}
		}

		if slotType == 1 && state.NextRefreshTime > now {
			return nil
		}
		if slotType == 2 && state.Consumed {
			return nil
		}

		drops := islandFormulaDrops(formula)
		if len(drops) == 0 {
			return nil
		}
		if err := applyIslandDropsTx(context.Background(), tx, client, drops); err != nil {
			return err
		}

		state.CollectedCount++
		if slotType == 1 {
			recovery := loadIslandSetInt("mining_recovery_time", 300)
			state.NextRefreshTime = now + recovery
			response.RefreshTime = proto.Uint32(state.NextRefreshTime)
		} else {
			state.Consumed = true
			state.NextRefreshTime = 0
			response.RefreshTime = proto.Uint32(0)
		}

		if err := orm.UpsertIslandSlotCollectStateTx(context.Background(), tx, state); err != nil {
			return err
		}

		response.Result = proto.Uint32(0)
		response.DropList = mergeDropList(drops)
		return nil
	})
	if err != nil {
		return client.SendMessage(21508, response)
	}

	return client.SendMessage(21508, response)
}
