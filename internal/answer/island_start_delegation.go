package answer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

type islandStartDelegationFormula struct {
	ID              uint32     `json:"id"`
	StaminaCost     uint32     `json:"stamina_cost"`
	CommissionCost  [][]uint32 `json:"commission_cost"`
	Duration        uint32     `json:"duration"`
	Workload        uint32     `json:"workload"`
	ProductionLimit uint32     `json:"production_limit"`
}

func IslandStartDelegation(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21501
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21502, err
	}

	response := &protobuf.SC_21502{
		Result:    proto.Uint32(0),
		ShipPower: proto.Uint32(0),
		ShipAppoint: &protobuf.PB_ISLAND_SHIP_APPOINT{
			Id:           proto.Uint32(0),
			ShipId:       proto.Uint32(0),
			MaxTimes:     proto.Uint32(0),
			GetTimes:     proto.Uint32(0),
			FormulaId:    proto.Uint32(0),
			StartTime:    proto.Uint32(0),
			CostTimeList: []uint32{},
			SpeedTime:    proto.Uint32(0),
			TimesExtra:   []*protobuf.PB_ISLAND_PART_EFFECT{},
		},
	}

	if payload.GetBuildId() == 0 || payload.GetAreaId() == 0 || payload.GetShipId() == 0 || payload.GetFormulaId() == 0 || payload.GetNum() == 0 {
		response.Result = proto.Uint32(1)
		return client.SendMessage(21502, response)
	}
	if err := ensureCommanderLoaded(client, "Island/StartDelegation"); err != nil {
		response.Result = proto.Uint32(5)
		return client.SendMessage(21502, response)
	}

	formula, ok, err := loadIslandStartDelegationFormula(payload.GetFormulaId())
	if err != nil {
		response.Result = proto.Uint32(5)
		return client.SendMessage(21502, response)
	}
	if !ok {
		response.Result = proto.Uint32(1)
		return client.SendMessage(21502, response)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		slot, err := orm.GetIslandDelegationForUpdateTx(context.Background(), tx, client.Commander.CommanderID, payload.GetBuildId(), payload.GetAreaId())
		if err != nil && !db.IsNotFound(err) {
			response.Result = proto.Uint32(5)
			return err
		}
		if slot == nil {
			slot = &orm.IslandDelegation{CommanderID: client.Commander.CommanderID, BuildID: payload.GetBuildId(), AreaID: payload.GetAreaId()}
		}
		if slot.HasRole {
			response.Result = proto.Uint32(2)
			return nil
		}

		energyCost := formula.StaminaCost * payload.GetNum()
		currentEnergy, err := orm.ConsumeOwnedShipEnergyTx(context.Background(), tx, client.Commander.CommanderID, payload.GetShipId(), energyCost)
		if err != nil {
			if db.IsNotFound(err) {
				response.Result = proto.Uint32(3)
				return nil
			}
			response.Result = proto.Uint32(5)
			return err
		}
		for i := range formula.CommissionCost {
			if len(formula.CommissionCost[i]) < 2 {
				continue
			}
			itemID := formula.CommissionCost[i][0]
			itemCount := formula.CommissionCost[i][1] * payload.GetNum()
			if err := orm.ConsumeIslandInventoryCheckedTx(context.Background(), tx, client.Commander.CommanderID, itemID, itemCount); err != nil {
				if db.IsNotFound(err) {
					response.Result = proto.Uint32(4)
					return err
				}
				response.Result = proto.Uint32(5)
				return err
			}
		}

		now := nowUnix()
		duration := formula.Duration
		if duration == 0 {
			duration = formula.Workload
		}
		if duration == 0 {
			duration = 60
		}
		finishTime := now + duration*payload.GetNum()
		slot.HasRole = true
		slot.RewardReady = false
		slot.ShipID = payload.GetShipId()
		slot.FormulaID = payload.GetFormulaId()
		slot.MaxTimes = payload.GetNum()
		slot.GetTimes = 0
		slot.StartTime = now
		slot.CostTimeList = []uint32{finishTime}
		slot.SpeedTime = 0
		slot.TimesExtra = []uint32{}
		slot.RecoverTime = finishTime
		slot.AddExp = payload.GetNum() * 10
		slot.ReturnNum = payload.GetNum()
		if err := orm.UpsertIslandDelegationTx(context.Background(), tx, slot); err != nil {
			response.Result = proto.Uint32(5)
			return err
		}

		response.ShipPower = proto.Uint32(currentEnergy)
		response.ShipAppoint = &protobuf.PB_ISLAND_SHIP_APPOINT{
			Id:           proto.Uint32(payload.GetAreaId()),
			ShipId:       proto.Uint32(payload.GetShipId()),
			MaxTimes:     proto.Uint32(slot.MaxTimes),
			GetTimes:     proto.Uint32(slot.GetTimes),
			FormulaId:    proto.Uint32(slot.FormulaID),
			StartTime:    proto.Uint32(slot.StartTime),
			CostTimeList: append([]uint32{}, slot.CostTimeList...),
			SpeedTime:    proto.Uint32(slot.SpeedTime),
			TimesExtra:   []*protobuf.PB_ISLAND_PART_EFFECT{},
		}
		if ownedShip, ok := client.Commander.OwnedShipsMap[payload.GetShipId()]; ok {
			ownedShip.Energy = currentEnergy
		}
		return nil
	})
	if err != nil {
		return client.SendMessage(21502, response)
	}
	return client.SendMessage(21502, response)
}

func loadIslandStartDelegationFormula(formulaID uint32) (*islandStartDelegationFormula, bool, error) {
	entry, err := orm.GetConfigEntry(islandFormulaCategory, fmt.Sprintf("%d", formulaID))
	if err != nil {
		return nil, false, nil
	}
	var cfg islandStartDelegationFormula
	if err := json.Unmarshal(entry.Data, &cfg); err != nil {
		return nil, false, err
	}
	if cfg.ID == 0 {
		cfg.ID = formulaID
	}
	return &cfg, true, nil
}
