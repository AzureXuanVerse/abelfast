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

const (
	islandProsperityAwardOK      = uint32(0)
	islandProsperityAwardInvalid = uint32(1)
	islandProsperityAwardState   = uint32(2)
	islandProsperityAwardPersist = uint32(3)
)

func IslandClaimProsperityReward(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21010
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21011, err
	}

	response := &protobuf.SC_21011{Ret: proto.Uint32(islandProsperityAwardInvalid), DropList: []*protobuf.DROPINFO{}}
	level := payload.GetLevel()
	if level == 0 {
		return client.SendMessage(21011, response)
	}
	if err := ensureCommanderLoaded(client, "Island/ProsperityAward"); err != nil {
		response.Ret = proto.Uint32(islandProsperityAwardPersist)
		return client.SendMessage(21011, response)
	}

	config, found, err := loadIslandProsperityConfig(level)
	if err != nil {
		response.Ret = proto.Uint32(islandProsperityAwardPersist)
		return client.SendMessage(21011, response)
	}
	if !found {
		return client.SendMessage(21011, response)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.GetIslandProsperityStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			response.Ret = proto.Uint32(islandProsperityAwardPersist)
			return err
		}
		for _, claimed := range state.ClaimedLevels {
			if claimed == level {
				response.Ret = proto.Uint32(islandProsperityAwardState)
				return nil
			}
		}
		if state.Prosperity < config.Prosperity {
			response.Ret = proto.Uint32(islandProsperityAwardState)
			return nil
		}

		drops, err := buildAwardDrops(config.AwardDisplay)
		if err != nil {
			response.Ret = proto.Uint32(islandProsperityAwardInvalid)
			return nil
		}
		if err := applyIslandDropsTx(context.Background(), tx, client, drops); err != nil {
			response.Ret = proto.Uint32(islandProsperityAwardPersist)
			return err
		}

		state.ClaimedLevels = append(state.ClaimedLevels, level)
		if err := orm.SaveIslandProsperityStateTx(context.Background(), tx, state); err != nil {
			response.Ret = proto.Uint32(islandProsperityAwardPersist)
			return err
		}

		response.Ret = proto.Uint32(islandProsperityAwardOK)
		response.DropList = mergeDropList(drops)
		return nil
	})
	if err != nil {
		return client.SendMessage(21011, response)
	}

	return client.SendMessage(21011, response)
}
