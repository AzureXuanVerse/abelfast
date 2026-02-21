package island

import (
	"context"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func IslandBookPointAwardClaim(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21347
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21348, err
	}

	response := &protobuf.SC_21348{Result: proto.Uint32(1), DropList: []*protobuf.DROPINFO{}}
	if err := ensureCommanderLoaded(client, "Island/BookPointAward"); err != nil {
		return client.SendMessage(21348, response)
	}

	rewardByID, rewardIDsByType, err := loadIslandCollectionRewardConfig()
	if err != nil {
		return client.SendMessage(21348, response)
	}
	rewardCfg, ok := rewardByID[payload.GetLv()]
	if !ok {
		return client.SendMessage(21348, response)
	}

	guideByID, err := loadIslandIllustratedGuideConfig()
	if err != nil {
		return client.SendMessage(21348, response)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.GetIslandBookStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			return err
		}

		claimed := make(map[uint32]struct{}, len(state.BookAwards))
		for _, awardID := range state.BookAwards {
			claimed[awardID] = struct{}{}
		}
		if _, exists := claimed[rewardCfg.ID]; exists {
			return nil
		}

		currentPoints := islandBookPointsByType(state, guideByID)
		if currentPoints[rewardCfg.Type] < rewardCfg.NeedExp {
			return nil
		}

		for _, previousID := range rewardIDsByType[rewardCfg.Type] {
			if previousID == rewardCfg.ID {
				break
			}
			if _, exists := claimed[previousID]; !exists {
				return nil
			}
		}

		awardRows, parseErr := parseIslandRewardDisplay(rewardCfg.AwardDisplay)
		if parseErr != nil {
			return nil
		}
		drops, dropErr := buildAwardDrops(awardRows)
		if dropErr != nil {
			return nil
		}
		if err := applyIslandDropsTx(context.Background(), tx, client, drops); err != nil {
			return err
		}

		state.BookAwards = append(state.BookAwards, rewardCfg.ID)
		if err := orm.SaveIslandBookStateTx(context.Background(), tx, state); err != nil {
			return err
		}

		response.Result = proto.Uint32(0)
		response.DropList = mergeDropList(drops)
		return nil
	})
	if err != nil {
		return client.SendMessage(21348, response)
	}

	return client.SendMessage(21348, response)
}

func islandBookPointsByType(state *orm.IslandBookState, guideByID map[uint32]islandIllustratedGuideConfig) map[uint32]uint32 {
	points := make(map[uint32]uint32)
	for _, collect := range state.BookCollects {
		cfg, ok := guideByID[collect.ID]
		if !ok {
			continue
		}
		total := collect.Base
		for _, lv := range collect.LvList {
			total += lv.Value
		}
		for _, star := range collect.StarList {
			total += star.Value
		}
		points[cfg.Type] += total
	}
	return points
}
