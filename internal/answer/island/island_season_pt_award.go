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

const (
	islandSeasonPTAwardOK      = uint32(0)
	islandSeasonPTAwardInvalid = uint32(1)
	islandSeasonPTAwardState   = uint32(2)
	islandSeasonPTAwardPersist = uint32(3)
)

func IslandClaimSeasonPTReward(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21022
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21023, err
	}

	response := &protobuf.SC_21023{Result: proto.Uint32(islandSeasonPTAwardInvalid), DropList: []*protobuf.DROPINFO{}}
	if err := ensureCommanderLoaded(client, "Island/SeasonPTClaim"); err != nil {
		response.Result = proto.Uint32(islandSeasonPTAwardPersist)
		return client.SendMessage(21023, response)
	}

	seasonConfig, found, err := loadIslandSeasonConfig()
	if err != nil {
		response.Result = proto.Uint32(islandSeasonPTAwardPersist)
		return client.SendMessage(21023, response)
	}
	if !found || len(seasonConfig.Target) == 0 || len(seasonConfig.Target) != len(seasonConfig.PTAwardDisplay) {
		return client.SendMessage(21023, response)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		season, err := orm.GetIslandSeason(client.Commander.CommanderID)
		if err != nil && !db.IsNotFound(err) {
			response.Result = proto.Uint32(islandSeasonPTAwardPersist)
			return err
		}
		pt := uint32(0)
		if season != nil {
			pt = season.PT
		}

		claimed, err := orm.ListIslandSeasonRewardClaimsTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			response.Result = proto.Uint32(islandSeasonPTAwardPersist)
			return err
		}
		claimedSet := make(map[uint32]struct{}, len(claimed))
		for _, value := range claimed {
			claimedSet[value] = struct{}{}
		}

		target := payload.GetTargetPt()
		pending := make([]uint32, 0)
		if target == 0 {
			for _, value := range seasonConfig.Target {
				if value > pt {
					continue
				}
				if _, ok := claimedSet[value]; ok {
					continue
				}
				pending = append(pending, value)
			}
		} else {
			if target > pt {
				response.Result = proto.Uint32(islandSeasonPTAwardState)
				return nil
			}
			if _, ok := claimedSet[target]; ok {
				response.Result = proto.Uint32(islandSeasonPTAwardState)
				return nil
			}
			if _, ok := seasonTargetDropByPT(seasonConfig, target); !ok {
				response.Result = proto.Uint32(islandSeasonPTAwardInvalid)
				return nil
			}
			pending = append(pending, target)
		}

		if len(pending) == 0 {
			response.Result = proto.Uint32(islandSeasonPTAwardState)
			return nil
		}

		drops := make([]*protobuf.DROPINFO, 0)
		for _, value := range pending {
			inserted, err := orm.AddIslandSeasonRewardClaimTx(context.Background(), tx, client.Commander.CommanderID, value)
			if err != nil {
				response.Result = proto.Uint32(islandSeasonPTAwardPersist)
				return err
			}
			if !inserted {
				continue
			}

			display, _ := seasonTargetDropByPT(seasonConfig, value)
			awardDrops, err := buildAwardDrops([][]uint32{display})
			if err != nil {
				response.Result = proto.Uint32(islandSeasonPTAwardInvalid)
				return nil
			}
			drops = append(drops, awardDrops...)
		}

		if len(drops) == 0 {
			response.Result = proto.Uint32(islandSeasonPTAwardState)
			return nil
		}

		if err := applyIslandDropsTx(context.Background(), tx, client, drops); err != nil {
			response.Result = proto.Uint32(islandSeasonPTAwardPersist)
			return err
		}

		response.Result = proto.Uint32(islandSeasonPTAwardOK)
		response.DropList = mergeDropList(drops)
		return nil
	})
	if err != nil {
		return client.SendMessage(21023, response)
	}

	return client.SendMessage(21023, response)
}

func seasonTargetDropByPT(config *islandSeasonConfig, target uint32) ([]uint32, bool) {
	for i, value := range config.Target {
		if value == target {
			return config.PTAwardDisplay[i], true
		}
	}
	return nil, false
}
