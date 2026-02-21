package meta

import (
	"context"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

func MetaCharacterTacticsLevelUpCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_63309
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 63310, err
	}

	response := protobuf.SC_63310{Result: proto.Uint32(1), SwitchCnt: proto.Uint32(0)}
	if err := ensureCommanderMetaLoaded(client.Commander); err != nil {
		return 0, 63310, err
	}
	ship, ok := client.Commander.OwnedShipsMap[payload.GetShipId()]
	if !ok {
		return client.SendMessage(63310, &response)
	}
	_, skillPos, err := metaSkillSlots(ship)
	if err != nil || len(skillPos) == 0 {
		return client.SendMessage(63310, &response)
	}
	pos, ok := skillPos[payload.GetSkillId()]
	if !ok {
		return client.SendMessage(63310, &response)
	}

	ctx := context.Background()
	err = orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		state, err := orm.GetOrCreateCommanderMetaTacticsStateTx(ctx, tx, client.Commander.CommanderID, ship.ID)
		if err != nil {
			return err
		}
		response.SwitchCnt = proto.Uint32(state.SwitchCnt)

		skillState, err := orm.GetOrCreateCommanderMetaTacticsSkillStateTx(ctx, tx, client.Commander.CommanderID, ship.ID, payload.GetSkillId(), pos)
		if err != nil {
			return err
		}
		if skillState.Level == 0 {
			return nil
		}

		skillConfig, err := orm.GetSkillDataTemplateConfig(skillState.SkillID)
		if err != nil {
			return nil
		}
		if skillState.Level >= skillConfig.MaxLevel {
			return nil
		}

		levelCfg, err := orm.GetShipMetaSkillTaskConfig(skillState.SkillID, skillState.Level)
		if err != nil {
			return nil
		}
		if skillState.Exp < levelCfg.NeedExp {
			return nil
		}

		skillState.Level++
		skillState.Exp = 0
		if err := orm.SaveCommanderMetaTacticsSkillStateTx(ctx, tx, skillState); err != nil {
			return err
		}
		response.Result = proto.Uint32(0)
		response.SwitchCnt = proto.Uint32(state.SwitchCnt)
		return nil
	})
	if err != nil {
		return 0, 63310, err
	}
	return client.SendMessage(63310, &response)
}
