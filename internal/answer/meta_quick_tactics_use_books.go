package answer

import (
	"context"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

func MetaQuickTacticsUseBooks(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_63319
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 63320, err
	}

	response := protobuf.SC_63320{Ret: proto.Uint32(1)}
	if err := ensureCommanderMetaLoaded(client.Commander); err != nil {
		return 0, 63320, err
	}
	ship, ok := client.Commander.OwnedShipsMap[payload.GetShipId()]
	if !ok || payload.GetSkillId() == 0 {
		return client.SendMessage(63320, &response)
	}

	_, skillPos, err := metaSkillSlots(ship)
	if err != nil || len(skillPos) == 0 {
		return client.SendMessage(63320, &response)
	}
	pos, ok := skillPos[payload.GetSkillId()]
	if !ok {
		return client.SendMessage(63320, &response)
	}

	bookCounts, ok := normalizeShipExpBooks(payload.GetBooks())
	if !ok {
		return client.SendMessage(63320, &response)
	}
	totalExp := uint32(0)
	for itemID, count := range bookCounts {
		cfg, err := orm.GetItemDataStatisticsConfig(itemID)
		if err != nil || cfg.Type != 25 {
			return client.SendMessage(63320, &response)
		}
		if !client.Commander.HasEnoughItem(itemID, count) {
			return client.SendMessage(63320, &response)
		}
		expPerBook, err := parseUsageArgExpValue(cfg.UsageArg)
		if err != nil {
			return client.SendMessage(63320, &response)
		}
		totalExp += expPerBook * count
	}

	ctx := context.Background()
	err = orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		skillState, err := orm.GetOrCreateCommanderMetaTacticsSkillStateTx(ctx, tx, client.Commander.CommanderID, ship.ID, payload.GetSkillId(), pos)
		if err != nil {
			return err
		}
		if skillState.Level == 0 {
			return nil
		}

		maxCfg, err := orm.GetSkillDataTemplateConfig(skillState.SkillID)
		if err != nil {
			return nil
		}
		newLevel := skillState.Level
		newExp := skillState.Exp
		remaining := totalExp
		for remaining > 0 && newLevel < maxCfg.MaxLevel {
			lvlCfg, err := orm.GetShipMetaSkillTaskConfig(skillState.SkillID, newLevel)
			if err != nil || lvlCfg.NeedExp == 0 {
				break
			}
			need := lvlCfg.NeedExp
			if newExp+remaining < need {
				newExp += remaining
				remaining = 0
				break
			}
			remaining -= (need - newExp)
			newLevel++
			newExp = 0
		}
		if newLevel >= maxCfg.MaxLevel {
			newLevel = maxCfg.MaxLevel
			newExp = 0
		}

		for itemID, count := range bookCounts {
			if err := client.Commander.ConsumeItemTx(ctx, tx, itemID, count); err != nil {
				return err
			}
		}

		skillState.Level = newLevel
		skillState.Exp = newExp
		if err := orm.SaveCommanderMetaTacticsSkillStateTx(ctx, tx, skillState); err != nil {
			return err
		}
		response.Ret = proto.Uint32(0)
		response.Level = proto.Uint32(newLevel)
		response.Exp = proto.Uint32(newExp)
		return nil
	})
	if err != nil {
		return client.SendMessage(63320, &response)
	}
	return client.SendMessage(63320, &response)
}
