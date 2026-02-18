package answer

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

type islandDelegationExtraEffect struct {
	Num        uint32
	CostExtra  uint32
	MainExtra  uint32
	OtherExtra uint32
}

func IslandAddDelegation(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21537
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21538, err
	}

	response := &protobuf.SC_21538{Result: proto.Uint32(1), CostTimeList: []uint32{}, TimesExtra: []*protobuf.PB_ISLAND_PART_EFFECT{}}
	if payload.GetBuildId() == 0 || payload.GetAreaId() == 0 || payload.GetAddNum() == 0 {
		return client.SendMessage(21538, response)
	}
	if err := ensureCommanderLoaded(client, "Island/AddDelegation"); err != nil {
		return client.SendMessage(21538, response)
	}

	err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		slot, err := orm.GetIslandDelegationForUpdateTx(context.Background(), tx, client.Commander.CommanderID, payload.GetBuildId(), payload.GetAreaId())
		if err != nil {
			if db.IsNotFound(err) {
				return nil
			}
			return err
		}
		if !slot.HasRole || slot.FormulaID == 0 || slot.ShipID == 0 {
			return nil
		}

		startFormula, startExists, startErr := loadIslandStartDelegationFormula(slot.FormulaID)
		if startErr != nil || !startExists {
			return startErr
		}

		maxAllowed := startFormula.ProductionLimit
		if maxAllowed > 0 && slot.MaxTimes+payload.GetAddNum() > maxAllowed {
			return nil
		}

		energyCost := startFormula.StaminaCost * payload.GetAddNum()
		currentEnergy, err := orm.ConsumeOwnedShipEnergyTx(context.Background(), tx, client.Commander.CommanderID, slot.ShipID, energyCost)
		if err != nil {
			return nil
		}

		for _, cost := range startFormula.CommissionCost {
			if len(cost) < 2 {
				continue
			}
			if err := orm.ConsumeIslandInventoryCheckedTx(context.Background(), tx, client.Commander.CommanderID, cost[0], cost[1]*payload.GetAddNum()); err != nil {
				return err
			}
		}

		lastFinish := slot.StartTime
		if len(slot.CostTimeList) > 0 {
			lastFinish = slot.CostTimeList[len(slot.CostTimeList)-1]
		}
		duration := startFormula.Duration
		if duration == 0 {
			duration = startFormula.Workload
		}
		if duration == 0 {
			duration = 60
		}

		extra := decodeIslandPartEffects(slot.TimesExtra)
		response.CostTimeList = make([]uint32, 0, payload.GetAddNum())
		response.TimesExtra = make([]*protobuf.PB_ISLAND_PART_EFFECT, 0, payload.GetAddNum())
		for i := uint32(0); i < payload.GetAddNum(); i++ {
			lastFinish += duration
			slot.CostTimeList = append(slot.CostTimeList, lastFinish)
			response.CostTimeList = append(response.CostTimeList, lastFinish)

			effect := islandDelegationExtraEffect{Num: slot.MaxTimes + i + 1, CostExtra: 0, MainExtra: 0, OtherExtra: 0}
			extra = append(extra, effect)
			response.TimesExtra = append(response.TimesExtra, &protobuf.PB_ISLAND_PART_EFFECT{
				Num:        proto.Uint32(effect.Num),
				CostExtra:  proto.Uint32(effect.CostExtra),
				MainExtra:  proto.Uint32(effect.MainExtra),
				OtherExtra: proto.Uint32(effect.OtherExtra),
			})
		}

		slot.TimesExtra = encodeIslandPartEffects(extra)
		slot.MaxTimes += payload.GetAddNum()
		slot.ReturnNum += payload.GetAddNum()
		slot.AddExp += payload.GetAddNum() * 10
		slot.RecoverTime = lastFinish
		if err := orm.UpsertIslandDelegationTx(context.Background(), tx, slot); err != nil {
			return err
		}

		response.Result = proto.Uint32(0)
		if ownedShip, ok := client.Commander.OwnedShipsMap[slot.ShipID]; ok {
			ownedShip.Energy = currentEnergy
		}
		return nil
	})
	if err != nil {
		return client.SendMessage(21538, response)
	}

	return client.SendMessage(21538, response)
}

func decodeIslandPartEffects(values []uint32) []islandDelegationExtraEffect {
	if len(values) < 4 {
		return []islandDelegationExtraEffect{}
	}
	out := make([]islandDelegationExtraEffect, 0, len(values)/4)
	for i := 0; i+3 < len(values); i += 4 {
		out = append(out, islandDelegationExtraEffect{
			Num:        values[i],
			CostExtra:  values[i+1],
			MainExtra:  values[i+2],
			OtherExtra: values[i+3],
		})
	}
	return out
}

func encodeIslandPartEffects(values []islandDelegationExtraEffect) []uint32 {
	out := make([]uint32, 0, len(values)*4)
	for _, effect := range values {
		out = append(out, effect.Num, effect.CostExtra, effect.MainExtra, effect.OtherExtra)
	}
	return out
}
