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

func IslandFinishDelegation(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21503
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21504, err
	}

	response := &protobuf.SC_21504{
		Result:      proto.Uint32(0),
		ShipId:      proto.Uint32(0),
		CurEnergy:   proto.Uint32(0),
		AddExp:      proto.Uint32(0),
		RecoverTime: proto.Uint32(0),
		Award:       []*protobuf.PB_ISLAND_APPOINT_AREA_AWARD{},
		ReturnNum:   proto.Uint32(0),
	}
	if payload.GetBuildId() == 0 || payload.GetAreaId() == 0 {
		response.Result = proto.Uint32(1)
		return client.SendMessage(21504, response)
	}
	if err := ensureCommanderLoaded(client, "Island/FinishDelegation"); err != nil {
		response.Result = proto.Uint32(4)
		return client.SendMessage(21504, response)
	}

	now := nowUnix()
	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		slot, err := orm.GetIslandDelegationForUpdateTx(context.Background(), tx, client.Commander.CommanderID, payload.GetBuildId(), payload.GetAreaId())
		if err != nil {
			if db.IsNotFound(err) {
				response.Result = proto.Uint32(2)
				return nil
			}
			response.Result = proto.Uint32(4)
			return err
		}
		if !slot.HasRole {
			response.Result = proto.Uint32(2)
			return nil
		}

		finishTime := slot.StartTime
		if len(slot.CostTimeList) > 0 {
			finishTime = slot.CostTimeList[len(slot.CostTimeList)-1]
		}
		finished := finishTime <= now

		slot.HasRole = false
		slot.RecoverTime = finishTime
		response.ShipId = proto.Uint32(slot.ShipID)
		response.AddExp = proto.Uint32(slot.AddExp)
		response.RecoverTime = proto.Uint32(slot.RecoverTime)
		if ownedShip, ok := client.Commander.OwnedShipsMap[slot.ShipID]; ok {
			response.CurEnergy = proto.Uint32(ownedShip.Energy)
		}

		if finished {
			slot.RewardReady = true
			slot.MainNum = slot.MaxTimes
			slot.ReturnNum = 0
			award := &protobuf.PB_ISLAND_APPOINT_AREA_AWARD{
				Id:        proto.Uint32(slot.AreaID),
				ShipId:    proto.Uint32(slot.ShipID),
				Exp:       proto.Uint32(slot.AddExp),
				FormulaId: proto.Uint32(slot.FormulaID),
				FormulaDropList: []*protobuf.PB_FORMULA_DROP_INFO{
					{Id: proto.Uint32(0), Num: proto.Uint32(slot.MainNum)},
				},
				MainNum:  proto.Uint32(slot.MainNum),
				OtherNum: proto.Uint32(slot.OtherNum),
			}
			response.Award = []*protobuf.PB_ISLAND_APPOINT_AREA_AWARD{award}
		} else {
			slot.RewardReady = false
			slot.ReturnNum = slot.MaxTimes
			response.ReturnNum = proto.Uint32(slot.ReturnNum)
		}

		if err := orm.UpsertIslandDelegationTx(context.Background(), tx, slot); err != nil {
			response.Result = proto.Uint32(4)
			return err
		}
		return nil
	})
	if err != nil {
		return client.SendMessage(21504, response)
	}
	return client.SendMessage(21504, response)
}
