package answer

import (
	"context"
	"sort"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	islandOrderFavorClaimOK             = uint32(0)
	islandOrderFavorClaimInvalid        = uint32(1)
	islandOrderFavorClaimNotEligible    = uint32(2)
	islandOrderFavorClaimAlreadyClaimed = uint32(3)
	islandOrderFavorClaimPersistError   = uint32(4)
)

func IslandClaimOrderFavorReward(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21412
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21413, err
	}

	response := &protobuf.SC_21413{Result: proto.Uint32(islandOrderFavorClaimInvalid), DropList: []*protobuf.DROPINFO{}}
	level := payload.GetLv()
	if level == 0 {
		return client.SendMessage(21413, response)
	}
	if err := ensureCommanderLoaded(client, "Island/OrderFavorClaim"); err != nil {
		response.Result = proto.Uint32(islandOrderFavorClaimPersistError)
		return client.SendMessage(21413, response)
	}

	favorConfig, err := loadIslandOrderFavorConfig()
	if err != nil {
		response.Result = proto.Uint32(islandOrderFavorClaimPersistError)
		return client.SendMessage(21413, response)
	}
	claimConfig, ok := favorConfig[level]
	if !ok {
		return client.SendMessage(21413, response)
	}

	requiredExp := requiredFavorExp(level, favorConfig)
	if requiredExp == 0 && level != 1 {
		return client.SendMessage(21413, response)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.GetIslandOrderStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			response.Result = proto.Uint32(islandOrderFavorClaimPersistError)
			return err
		}
		if state.Favor < requiredExp {
			response.Result = proto.Uint32(islandOrderFavorClaimNotEligible)
			return nil
		}

		inserted, err := orm.AddIslandOrderFavorClaimTx(context.Background(), tx, client.Commander.CommanderID, level)
		if err != nil {
			response.Result = proto.Uint32(islandOrderFavorClaimPersistError)
			return err
		}
		if !inserted {
			response.Result = proto.Uint32(islandOrderFavorClaimAlreadyClaimed)
			return nil
		}

		drops, err := buildAwardDrops(claimConfig.AwardDisplay)
		if err != nil {
			response.Result = proto.Uint32(islandOrderFavorClaimInvalid)
			return nil
		}
		if err := applyIslandDropsTx(context.Background(), tx, client, drops); err != nil {
			response.Result = proto.Uint32(islandOrderFavorClaimPersistError)
			return err
		}

		response.Result = proto.Uint32(islandOrderFavorClaimOK)
		response.DropList = mergeDropList(drops)
		return nil
	})
	if err != nil {
		return client.SendMessage(21413, response)
	}

	return client.SendMessage(21413, response)
}

func requiredFavorExp(level uint32, cfg map[uint32]islandOrderFavorConfig) uint32 {
	levels := make([]uint32, 0, len(cfg))
	for k := range cfg {
		if k <= level {
			levels = append(levels, k)
		}
	}
	sort.Slice(levels, func(i, j int) bool { return levels[i] < levels[j] })
	if len(levels) != int(level) {
		return 0
	}
	total := uint32(0)
	for _, lv := range levels {
		total += cfg[lv].Exp
	}
	return total
}
