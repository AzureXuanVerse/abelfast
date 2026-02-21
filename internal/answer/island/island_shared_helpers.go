package island

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

const (
	taskResultSuccess = uint32(0)
	taskResultFailed  = uint32(1)
)

type activityTemplate struct {
	ID           uint32          `json:"id"`
	Type         uint32          `json:"type"`
	ConfigID     uint32          `json:"config_id"`
	Time         json.RawMessage `json:"time"`
	ConfigClient json.RawMessage `json:"config_client"`
	ConfigData   json.RawMessage `json:"config_data"`
}

func buildAwardDrops(display [][]uint32) ([]*protobuf.DROPINFO, error) {
	drops := make([]*protobuf.DROPINFO, 0, len(display))
	for _, entry := range display {
		if len(entry) < 3 {
			return nil, errors.New("award display entry missing fields")
		}
		drops = append(drops, newDropInfo(entry[0], entry[1], entry[2]))
	}
	return drops, nil
}

func ensureCommanderLoaded(client *connection.Client, scope string) error {
	if client.Commander.CommanderItemsMap != nil && client.Commander.MiscItemsMap != nil && client.Commander.OwnedResourcesMap != nil && client.Commander.OwnedShipsMap != nil {
		return nil
	}
	logger.LogEvent(scope, "Load", "commander maps missing, reloading commander", logger.LOG_LEVEL_INFO)
	if err := client.Commander.Load(); err != nil {
		logger.LogEvent(scope, "Load", "commander load failed", logger.LOG_LEVEL_ERROR)
		return err
	}
	return nil
}

func newDropInfo(dropType uint32, dropID uint32, count uint32) *protobuf.DROPINFO {
	return &protobuf.DROPINFO{
		Type:   proto.Uint32(dropType),
		Id:     proto.Uint32(dropID),
		Number: proto.Uint32(count),
	}
}

func containsUint32(list []uint32, value uint32) bool {
	for _, current := range list {
		if current == value {
			return true
		}
	}
	return false
}

func rawUint32(raw json.RawMessage) (uint32, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var value uint64
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, false
	}
	if value > math.MaxUint32 {
		return 0, false
	}
	return uint32(value), true
}

func consumeCommanderItemTx(ctx context.Context, tx pgx.Tx, commanderID uint32, itemID uint32, count uint32) (bool, error) {
	result, err := tx.Exec(ctx, `
UPDATE commander_items
SET count = count - $3
WHERE commander_id = $1 AND item_id = $2 AND count >= $3
`, int64(commanderID), int64(itemID), int64(count))
	if err != nil {
		return false, err
	}
	if result.RowsAffected() == 1 {
		return true, nil
	}

	result, err = tx.Exec(ctx, `
UPDATE commander_misc_items
SET data = data - $3
WHERE commander_id = $1 AND item_id = $2 AND data >= $3
`, int64(commanderID), int64(itemID), int64(count))
	if err != nil {
		return false, err
	}
	if result.RowsAffected() == 1 {
		return true, nil
	}
	return false, nil
}

func loadActivityTemplate(id uint32) (activityTemplate, error) {
	entry, err := orm.GetConfigEntry("ShareCfg/activity_template.json", strconv.FormatUint(uint64(id), 10))
	if err != nil {
		return activityTemplate{}, err
	}
	var template activityTemplate
	if err := json.Unmarshal(entry.Data, &template); err != nil {
		return activityTemplate{}, err
	}
	return template, nil
}

func normalizeUsageArg(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		text = strings.TrimSpace(text)
		if text == "" {
			text = "[]"
		}
		if !json.Valid([]byte(text)) {
			return nil, fmt.Errorf("invalid usage_arg: %s", text)
		}
		return json.RawMessage([]byte(text)), nil
	}
	return raw, nil
}

func decodeUsageArg(raw json.RawMessage, out any) error {
	normalized, err := normalizeUsageArg(raw)
	if err != nil {
		return err
	}
	if len(normalized) == 0 {
		return nil
	}
	return json.Unmarshal(normalized, out)
}

func applyDrop(client *connection.Client, dropType uint32, dropID uint32, dropCount uint32) (bool, error) {
	switch dropType {
	case consts.DROP_TYPE_RESOURCE:
		return true, client.Commander.AddResource(dropID, dropCount)
	case consts.DROP_TYPE_ITEM:
		return true, client.Commander.AddItem(dropID, dropCount)
	case consts.DROP_TYPE_SHIP:
		for i := uint32(0); i < dropCount; i++ {
			if _, err := client.Commander.AddShip(dropID); err != nil {
				return true, err
			}
		}
		return true, nil
	case consts.DROP_TYPE_SKIN:
		for i := uint32(0); i < dropCount; i++ {
			if err := client.Commander.GiveSkin(dropID); err != nil {
				return true, err
			}
		}
		return true, nil
	case consts.DROP_TYPE_VITEM:
		return true, nil
	default:
		return false, nil
	}
}

func accumulateDrop(drops map[string]*protobuf.DROPINFO, dropType uint32, dropID uint32, count uint32) {
	key := fmt.Sprintf("%d_%d", dropType, dropID)
	entry := drops[key]
	if entry == nil {
		drops[key] = &protobuf.DROPINFO{
			Type:   proto.Uint32(dropType),
			Id:     proto.Uint32(dropID),
			Number: proto.Uint32(count),
		}
		return
	}
	entry.Number = proto.Uint32(entry.GetNumber() + count)
}

func dropMapToSortedList(drops map[string]*protobuf.DROPINFO) []*protobuf.DROPINFO {
	list := make([]*protobuf.DROPINFO, 0, len(drops))
	for _, drop := range drops {
		list = append(list, drop)
	}
	sort.Slice(list, func(i int, j int) bool {
		if list[i].GetType() == list[j].GetType() {
			return list[i].GetId() < list[j].GetId()
		}
		return list[i].GetType() < list[j].GetType()
	})
	return list
}

func applyLoveLetterDropsTx(ctx context.Context, tx pgx.Tx, client *connection.Client, drops map[string]*protobuf.DROPINFO) error {
	for _, drop := range drops {
		dropType := drop.GetType()
		dropID := drop.GetId()
		dropCount := drop.GetNumber()
		switch dropType {
		case consts.DROP_TYPE_RESOURCE:
			if err := client.Commander.AddResourceTx(ctx, tx, dropID, dropCount); err != nil {
				return err
			}
		case consts.DROP_TYPE_ITEM, consts.DROP_TYPE_LOVE_LETTER:
			if err := client.Commander.AddItemTx(ctx, tx, dropID, dropCount); err != nil {
				return err
			}
		case consts.DROP_TYPE_EQUIP:
			if err := addOwnedEquipmentPGXTx(ctx, tx, client.Commander, dropID, dropCount); err != nil {
				return err
			}
		case consts.DROP_TYPE_SHIP:
			for i := uint32(0); i < dropCount; i++ {
				if _, err := client.Commander.AddShipTx(ctx, tx, dropID); err != nil {
					return err
				}
			}
		case consts.DROP_TYPE_FURNITURE:
			now := uint32(time.Now().Unix())
			if err := orm.AddCommanderFurnitureTx(ctx, tx, client.Commander.CommanderID, dropID, dropCount, now); err != nil {
				return err
			}
		case consts.DROP_TYPE_SKIN:
			for i := uint32(0); i < dropCount; i++ {
				if err := client.Commander.GiveSkinTx(ctx, tx, dropID); err != nil {
					return err
				}
			}
		case consts.DROP_TYPE_VITEM:
			continue
		default:
			return fmt.Errorf("unsupported reward drop type %d", dropType)
		}
	}
	return nil
}

func addOwnedEquipmentPGXTx(ctx context.Context, tx pgx.Tx, commander *orm.Commander, equipmentID uint32, count uint32) error {
	if count == 0 {
		return nil
	}
	if commander.OwnedEquipmentMap == nil {
		commander.RebuildOwnedEquipmentMap()
	}
	_, err := tx.Exec(ctx, `
INSERT INTO owned_equipments (commander_id, equipment_id, count)
VALUES ($1, $2, $3)
ON CONFLICT (commander_id, equipment_id)
DO UPDATE SET count = owned_equipments.count + EXCLUDED.count
`, int64(commander.CommanderID), int64(equipmentID), int64(count))
	if err != nil {
		return err
	}
	if existing, ok := commander.OwnedEquipmentMap[equipmentID]; ok {
		existing.Count += count
		return nil
	}
	commander.OwnedEquipments = append(commander.OwnedEquipments, orm.OwnedEquipment{CommanderID: commander.CommanderID, EquipmentID: equipmentID, Count: count})
	commander.RebuildOwnedEquipmentMap()
	return nil
}
