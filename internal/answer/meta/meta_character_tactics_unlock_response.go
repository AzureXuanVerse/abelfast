package meta

import (
	"context"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

func MetaCharacterTacticsUnlockCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_63311
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 63312, err
	}

	response := protobuf.SC_63312{Result: proto.Uint32(1)}
	if err := ensureCommanderMetaLoaded(client.Commander); err != nil {
		return 0, 63312, err
	}

	ship, ok := client.Commander.OwnedShipsMap[payload.GetShipId()]
	if !ok {
		return client.SendMessage(63312, &response)
	}
	_, skillPos, err := metaSkillSlots(ship)
	if err != nil || len(skillPos) == 0 {
		return client.SendMessage(63312, &response)
	}
	pos, ok := skillPos[payload.GetSkillId()]
	if !ok {
		return client.SendMessage(63312, &response)
	}

	skillCfg, err := orm.GetShipMetaSkillTaskConfig(payload.GetSkillId(), 1)
	if err != nil {
		return client.SendMessage(63312, &response)
	}
	requiredItem := uint32(0)
	requiredCount := uint32(0)
	for _, unlock := range skillCfg.SkillUnlock {
		if len(unlock) < 3 {
			continue
		}
		if unlock[0] == payload.GetIndex() {
			requiredItem = unlock[1]
			requiredCount = unlock[2]
			break
		}
	}
	if requiredItem == 0 || requiredCount == 0 || !client.Commander.HasEnoughItem(requiredItem, requiredCount) {
		return client.SendMessage(63312, &response)
	}

	ctx := context.Background()
	err = orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		state, err := orm.GetOrCreateCommanderMetaTacticsStateTx(ctx, tx, client.Commander.CommanderID, ship.ID)
		if err != nil {
			return err
		}
		skillState, err := orm.GetOrCreateCommanderMetaTacticsSkillStateTx(ctx, tx, client.Commander.CommanderID, ship.ID, payload.GetSkillId(), pos)
		if err != nil {
			return err
		}
		if skillState.Level > 0 {
			return nil
		}
		if err := client.Commander.ConsumeItemTx(ctx, tx, requiredItem, requiredCount); err != nil {
			return err
		}
		skillState.Level = 1
		skillState.Exp = 0
		if err := orm.SaveCommanderMetaTacticsSkillStateTx(ctx, tx, skillState); err != nil {
			return err
		}
		if state.CurrentSkillID == 0 {
			state.CurrentSkillID = payload.GetSkillId()
			if err := orm.SaveCommanderMetaTacticsStateTx(ctx, tx, state); err != nil {
				return err
			}
		}
		response.Result = proto.Uint32(0)
		return nil
	})
	if err != nil {
		return client.SendMessage(63312, &response)
	}
	return client.SendMessage(63312, &response)
}
