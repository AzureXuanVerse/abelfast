package answer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	islandShipBreakoutConfigCategory = "ShareCfg/island_chara_template.json"

	islandShipBreakoutResultSuccess = uint32(0)
	islandShipBreakoutResultFailure = uint32(1)
)

type islandShipBreakoutConfig struct {
	ID              uint32       `json:"id"`
	UpgradeLevel    []uint32     `json:"upgrade_level"`
	UpgradeMaterial [][][]uint32 `json:"upgrade_material"`
}

func HandleIslandShipBreakout(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21601
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21602, err
	}

	response := &protobuf.SC_21602{Result: proto.Uint32(islandShipBreakoutResultFailure)}
	if err := ensureCommanderLoaded(client, "Island/ShipBreakout"); err != nil {
		return client.SendMessage(21602, response)
	}

	shipID := payload.GetShipId()
	if shipID == 0 {
		logIslandBreakoutFailure(client.Commander.CommanderID, shipID, "invalid_ship")
		return client.SendMessage(21602, response)
	}

	err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ship, err := orm.GetIslandShipForUpdateTx(context.Background(), tx, client.Commander.CommanderID, shipID)
		if err != nil {
			if !db.IsNotFound(err) {
				return err
			}
			logIslandBreakoutFailure(client.Commander.CommanderID, shipID, "missing_ship")
			return nil
		}

		cfg, ok, err := loadIslandShipBreakoutConfig(ship.ShipID)
		if err != nil {
			return err
		}
		if !ok || len(cfg.UpgradeLevel) < 2 {
			logIslandBreakoutFailure(client.Commander.CommanderID, shipID, "missing_config")
			return nil
		}

		breakLevel := ship.BreakLv
		if breakLevel < 1 {
			logIslandBreakoutFailure(client.Commander.CommanderID, shipID, "invalid_break_level")
			return nil
		}
		maxBreakLevel := cfg.UpgradeLevel[1] + 1
		if breakLevel >= maxBreakLevel {
			logIslandBreakoutFailure(client.Commander.CommanderID, shipID, "max_break")
			return nil
		}

		breakoutGate := cfg.UpgradeLevel[0]
		if breakoutGate == 0 || ship.Level%breakoutGate != 0 {
			logIslandBreakoutFailure(client.Commander.CommanderID, shipID, "gate_fail")
			return nil
		}

		materials, ok := islandShipBreakoutMaterials(cfg.UpgradeMaterial, breakLevel)
		if !ok {
			logIslandBreakoutFailure(client.Commander.CommanderID, shipID, "missing_material")
			return nil
		}

		for i := range materials {
			if err := orm.ConsumeIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, materials[i][0], materials[i][1]); err != nil {
				return err
			}
		}

		if err := orm.IncrementIslandShipBreakoutTx(context.Background(), tx, client.Commander.CommanderID, shipID); err != nil {
			return err
		}

		response.Result = proto.Uint32(islandShipBreakoutResultSuccess)
		return nil
	})
	if err != nil {
		if err == orm.ErrInsufficientIslandInventory {
			logIslandBreakoutFailure(client.Commander.CommanderID, shipID, "insufficient_items")
		}
		return client.SendMessage(21602, response)
	}

	return client.SendMessage(21602, response)
}

func islandShipBreakoutMaterials(material [][][]uint32, breakLevel uint32) ([][2]uint32, bool) {
	idx := int(breakLevel - 1)
	if idx < 0 || idx >= len(material) {
		return nil, false
	}
	stage := material[idx]
	if len(stage) == 0 {
		return nil, false
	}
	result := make([][2]uint32, 0, len(stage))
	for i := range stage {
		if len(stage[i]) < 2 || stage[i][0] == 0 || stage[i][1] == 0 {
			return nil, false
		}
		result = append(result, [2]uint32{stage[i][0], stage[i][1]})
	}
	return result, true
}

func loadIslandShipBreakoutConfig(shipID uint32) (*islandShipBreakoutConfig, bool, error) {
	key := fmt.Sprintf("%d", shipID)
	if entry, err := orm.GetConfigEntry(islandShipBreakoutConfigCategory, key); err == nil {
		var cfg islandShipBreakoutConfig
		if err := json.Unmarshal(entry.Data, &cfg); err == nil {
			if cfg.ID == 0 {
				cfg.ID = shipID
			}
			return &cfg, true, nil
		}
	}

	entries, err := orm.ListConfigEntries(islandShipBreakoutConfigCategory)
	if err != nil {
		return nil, false, err
	}
	for i := range entries {
		var direct islandShipBreakoutConfig
		if err := json.Unmarshal(entries[i].Data, &direct); err == nil {
			if direct.ID == shipID {
				return &direct, true, nil
			}
		}
		var list []islandShipBreakoutConfig
		if err := json.Unmarshal(entries[i].Data, &list); err == nil {
			for j := range list {
				if list[j].ID == shipID {
					return &list[j], true, nil
				}
			}
		}
	}

	return nil, false, nil
}

func logIslandBreakoutFailure(commanderID uint32, shipID uint32, reason string) {
	logger.WithFields(
		"Island/ShipBreakout",
		logger.FieldValue("commander", commanderID),
		logger.FieldValue("ship_id", shipID),
		logger.FieldValue("reason", reason),
	).Info("breakout rejected")
}
