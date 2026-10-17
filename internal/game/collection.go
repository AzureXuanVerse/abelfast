package game

import (
	"context"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/gamedata"
	"github.com/ggmolly/belfast/internal/protobuf"
)

// storeup_data_template row: an archives milestone (model/vo/favorite.lua),
// award_display[i] given once the stars of the char_list ship groups the
// commander has reach level[i].
type storeupConfig struct {
	CharList     []uint32   `json:"char_list"` // ship_data_template.group_type
	Level        []uint32   `json:"level"`
	AwardDisplay [][]uint32 `json:"award_display"`
}

var storeupConfigs map[uint32]storeupConfig

func loadCollectionData() error {
	return gamedata.Load("storeup_data_template", &storeupConfigs)()
}

// archiveStar is a ship group's SHIP_STATISTICS_INFO star: its base
// template's (ShipGroup.star, what Favorite.getStarCount adds up).
func archiveStar(groupID uint32) uint32 {
	return shipTemplates[groupID*10+1].Star
}

// collectionAwards is SC_17001 ship_award_list: the award indexes claimed
// per storeup_data_template id. CollectionProxy only keeps the highest one
// (the next to claim follows it, Favorite.getNextAwardIndex).
func collectionAwards(ctx context.Context, queries *db.Queries, commanderID uint32) ([]*protobuf.SHIP_STATISTICS_AWARD, error) {
	rows, err := queries.ListCollectionAwards(ctx, commanderID)
	if err != nil {
		return nil, err
	}
	var list []*protobuf.SHIP_STATISTICS_AWARD
	for _, row := range rows {
		if len(list) == 0 || list[len(list)-1].GetId() != row.StoreupID {
			list = append(list, &protobuf.SHIP_STATISTICS_AWARD{Id: proto.Uint32(row.StoreupID)})
		}
		last := list[len(list)-1]
		last.AwardIndex = append(last.AwardIndex, row.AwardIndex)
	}
	return list, nil
}

// handleCollectionAward answers CollectionGetAwardCommand (CS_17005): the
// award_index (1-based) of milestone id, which Favorite.canGetRes offers when
// it follows the last one claimed and the stars of the milestone's ship
// groups in the archives reach its level. On result 0 the client adds the
// award_display drop itself (resources, items, equipment, furniture) but
// for a ship, which only gets a tip: the ship comes in SC_12042.
func (game *Game) handleCollectionAward(client *connection.Client, req *protobuf.CS_17005) error {
	ctx := context.Background()
	err := game.tx(ctx, client, func(tx *Tx) error {
		config, ok := storeupConfigs[req.GetId()]
		index := req.GetAwardIndex()
		if !ok || index == 0 || int(index) > len(config.AwardDisplay) || int(index) > len(config.Level) {
			return errRefused
		}
		last, err := tx.LastCollectionAward(ctx, db.LastCollectionAwardParams{CommanderID: tx.commanderID, StoreupID: req.GetId()})
		if err != nil {
			return err
		}
		if int64(index) != last+1 {
			return errRefused
		}
		groups, err := tx.ListShipGroups(ctx, tx.commanderID)
		if err != nil {
			return err
		}
		owned := make(map[uint32]bool, len(groups))
		for _, group := range groups {
			owned[group.GroupID] = true
		}
		stars := uint32(0)
		for _, groupID := range config.CharList {
			if owned[groupID] {
				stars += archiveStar(groupID)
			}
		}
		if stars < config.Level[index-1] {
			return errRefused
		}
		added, err := tx.AddCollectionAward(ctx, db.AddCollectionAwardParams{CommanderID: tx.commanderID, StoreupID: req.GetId(), AwardIndex: index})
		if err != nil {
			return err
		}
		if added == 0 {
			return errRefused
		}
		return tx.grantDrops(configDrops(config.AwardDisplay[index-1 : index]))
	})
	if err != nil && !refused(err) {
		return err
	}
	return client.Send(&protobuf.SC_17006{Result: result(err)})
}
