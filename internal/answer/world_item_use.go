package answer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

var errWorldItemUseInsufficient = errors.New("world item insufficient")

const (
	worldItemUseResultSuccess = 0
	worldItemUseResultFailure = 1

	worldUsageDrop          = "usage_drop"
	worldUsageDropAppointed = "usage_drop_appointed"
	worldUsageHealing       = "usage_world_healing"
	worldUsageHealingValue  = "usage_world_healing_value"
	worldUsageRecoverAP     = "usage_world_recoverAP"
	worldUsageSLGBuff       = "usage_worldSLGbuff"
	worldUsageClean         = "usage_world_clean"
	worldUsageFlag          = "usage_world_flag"
	worldUsageMap           = "usage_world_map"
)

type worldItemConfig struct {
	ID       uint32          `json:"id"`
	Usage    string          `json:"usage"`
	UsageArg json.RawMessage `json:"usage_arg"`
}

func WorldItemUse(buffer *[]byte, client *connection.Client) (int, int, error) {
	payload := &protobuf.CS_33301{}
	if err := proto.Unmarshal(*buffer, payload); err != nil {
		return 0, 33302, err
	}

	response := &protobuf.SC_33302{Result: proto.Uint32(worldItemUseResultFailure), DropList: []*protobuf.DROPINFO{}}
	if payload.GetId() == 0 || payload.GetCount() == 0 {
		return connection.SendProtoMessage(33302, client, response)
	}

	config, err := loadWorldItemConfig(payload.GetId())
	if err != nil || config == nil {
		return connection.SendProtoMessage(33302, client, response)
	}

	drops, ok, err := resolveWorldItemDrops(config, payload.GetArg(), payload.GetCount())
	if err != nil || !ok {
		return connection.SendProtoMessage(33302, client, response)
	}

	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		consumed, txErr := consumeCommanderItemTx(context.Background(), tx, client.Commander.CommanderID, payload.GetId(), payload.GetCount())
		if txErr != nil {
			return txErr
		}
		if !consumed {
			return errWorldItemUseInsufficient
		}
		if len(drops) == 0 {
			return nil
		}
		return applyLoveLetterDropsTx(context.Background(), tx, client, drops)
	})
	if err != nil {
		return connection.SendProtoMessage(33302, client, response)
	}

	response.Result = proto.Uint32(worldItemUseResultSuccess)
	response.DropList = dropMapToSortedList(drops)
	return connection.SendProtoMessage(33302, client, response)
}

func loadWorldItemConfig(itemID uint32) (*worldItemConfig, error) {
	entry, err := orm.GetConfigEntry("ShareCfg/world_item_data_template.json", fmt.Sprintf("%d", itemID))
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	config := &worldItemConfig{}
	if err := json.Unmarshal(entry.Data, config); err != nil {
		return nil, err
	}
	return config, nil
}

func resolveWorldItemDrops(config *worldItemConfig, arg []uint32, count uint32) (map[string]*protobuf.DROPINFO, bool, error) {
	if !validateWorldItemArgs(config.Usage, arg) {
		return nil, false, nil
	}
	drops := make(map[string]*protobuf.DROPINFO)
	switch config.Usage {
	case worldUsageDrop:
		var dropID uint32
		if err := decodeUsageArg(config.UsageArg, &dropID); err != nil {
			return nil, false, err
		}
		entries, err := listDropRestoreEntries(dropID)
		if err != nil {
			return nil, false, err
		}
		if len(entries) == 0 {
			return nil, false, nil
		}
		for _, entry := range entries {
			accumulateDrop(drops, normalizeWorldDropType(entry.Type), entry.ResourceType, entry.ResourceNum*count)
		}
	case worldUsageDropAppointed:
		if len(arg) < 3 {
			return nil, false, nil
		}
		var options [][]uint32
		if err := decodeUsageArg(config.UsageArg, &options); err != nil {
			return nil, false, err
		}
		selection, found := selectDropOption(options, arg)
		if !found || len(selection) < 3 {
			return nil, false, nil
		}
		accumulateDrop(drops, normalizeWorldDropType(selection[0]), selection[1], selection[2]*count)
	case worldUsageHealing, worldUsageHealingValue, worldUsageRecoverAP, worldUsageSLGBuff, worldUsageClean, worldUsageFlag, worldUsageMap:
		return drops, true, nil
	default:
		return nil, false, nil
	}
	return drops, true, nil
}

func validateWorldItemArgs(usage string, arg []uint32) bool {
	switch usage {
	case worldUsageHealing, worldUsageHealingValue:
		if len(arg) == 0 {
			return false
		}
		for _, id := range arg {
			if id == 0 {
				return false
			}
		}
		return true
	default:
		return true
	}
}

func normalizeWorldDropType(dropType uint32) uint32 {
	if dropType == 2 {
		return 2
	}
	return dropType
}
