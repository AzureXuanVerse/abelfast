package island

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func IslandGetNpcActionAward(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21702
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21703, err
	}

	response := &protobuf.SC_21703{Result: proto.Uint32(1), DropList: []*protobuf.DROPINFO{}}
	if err := ensureCommanderLoaded(client, "Island/NPCActionAward"); err != nil {
		return client.SendMessage(21703, response)
	}

	npcID := payload.GetNpcId()
	feedbackID := payload.GetActionFeedbackId()
	if npcID == 0 || feedbackID == 0 {
		return client.SendMessage(21703, response)
	}

	err := orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		npcCfg, found, err := loadIslandStrollNPCConfig(npcID)
		if err != nil || !found || npcCfg.ActionFeedback == 0 {
			return nil
		}
		if npcCfg.ActionFeedback != feedbackID {
			return nil
		}
		feedbackCfg, found, err := loadIslandActionFeedbackConfig(feedbackID)
		if err != nil || !found {
			return nil
		}

		state, err := orm.GetIslandNPCFeedbackState(client.Commander.CommanderID)
		if err != nil {
			state = &orm.IslandNPCFeedbackState{CommanderID: client.Commander.CommanderID, DayStartUnix: 0, ClaimedNPCIDs: []uint32{}}
		}
		today := currentDayStartUnix(time.Now().UTC())
		if state.DayStartUnix != today {
			state.DayStartUnix = today
			state.ClaimedNPCIDs = []uint32{}
		}
		for i := range state.ClaimedNPCIDs {
			if state.ClaimedNPCIDs[i] == npcID {
				return nil
			}
		}
		limit := parseIslandSetUintValue("island_feedback_award_times", 3)
		if uint32(len(state.ClaimedNPCIDs)) >= limit {
			return nil
		}

		drops, err := loadIslandDropList(feedbackCfg.DropID)
		if err != nil {
			return nil
		}
		if len(drops) > 0 {
			if err := applyIslandDropsTx(context.Background(), tx, client, drops); err != nil {
				return err
			}
		}
		state.ClaimedNPCIDs = append(state.ClaimedNPCIDs, npcID)
		if err := orm.UpsertIslandNPCFeedbackState(state); err != nil {
			return err
		}

		response.Result = proto.Uint32(0)
		response.DropList = mergeDropList(drops)
		return nil
	})
	if err != nil {
		_ = client.Commander.Load()
	}

	return client.SendMessage(21703, response)
}
