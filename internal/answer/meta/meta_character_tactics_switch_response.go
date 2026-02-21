package meta

import (
	"context"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

func MetaCharacterTacticsSwitchCommandResponse(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_63307
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 63308, err
	}

	response := protobuf.SC_63308{Result: proto.Uint32(1), SwitchCnt: proto.Uint32(0)}
	if err := ensureCommanderMetaLoaded(client.Commander); err != nil {
		return 0, 63308, err
	}

	ship, ok := client.Commander.OwnedShipsMap[payload.GetShipId()]
	if !ok {
		return client.SendMessage(63308, &response)
	}
	slots, skillPos, err := metaSkillSlots(ship)
	if err != nil || len(slots) == 0 {
		return client.SendMessage(63308, &response)
	}
	targetPos, ok := skillPos[payload.GetSkillId()]
	if !ok {
		return client.SendMessage(63308, &response)
	}

	ctx := context.Background()
	err = orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		state, err := orm.GetOrCreateCommanderMetaTacticsStateTx(ctx, tx, client.Commander.CommanderID, ship.ID)
		if err != nil {
			return err
		}
		response.SwitchCnt = proto.Uint32(state.SwitchCnt)

		skillState, err := orm.GetOrCreateCommanderMetaTacticsSkillStateTx(ctx, tx, client.Commander.CommanderID, ship.ID, payload.GetSkillId(), targetPos)
		if err != nil {
			return err
		}
		if skillState.Level == 0 {
			return nil
		}

		if state.CurrentSkillID != payload.GetSkillId() {
			if state.SwitchCnt == 0 {
				return nil
			}
			state.CurrentSkillID = payload.GetSkillId()
			state.SwitchCnt--
			if err := orm.SaveCommanderMetaTacticsStateTx(ctx, tx, state); err != nil {
				return err
			}
		}

		response.Result = proto.Uint32(0)
		response.SwitchCnt = proto.Uint32(state.SwitchCnt)
		return nil
	})
	if err != nil {
		return 0, 63308, err
	}
	return client.SendMessage(63308, &response)
}
