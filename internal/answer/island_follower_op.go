package answer

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	islandFollowerConfigCategory = "ShareCfg/island_set.json"

	islandFollowerOpAdd    = uint32(1)
	islandFollowerOpRemove = uint32(2)

	islandFollowerResultSuccess      = uint32(0)
	islandFollowerResultInvalidOp    = uint32(1)
	islandFollowerResultInvalidShip  = uint32(2)
	islandFollowerResultMaxReached   = uint32(3)
	islandFollowerResultPersistError = uint32(4)
)

type islandFollowerSetConfigEntry struct {
	Key         string `json:"key"`
	KeyValueInt uint32 `json:"key_value_int"`
}

func IslandFollowerOp(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21630
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21631, err
	}

	response := &protobuf.SC_21631{Result: proto.Uint32(islandFollowerResultPersistError)}
	if err := ensureCommanderLoaded(client, "Island/FollowerOp"); err != nil {
		return client.SendMessage(21631, response)
	}

	opType := payload.GetType()
	if opType != islandFollowerOpAdd && opType != islandFollowerOpRemove {
		response.Result = proto.Uint32(islandFollowerResultInvalidOp)
		return client.SendMessage(21631, response)
	}

	shipID := payload.GetShipId()
	if shipID == 0 {
		response.Result = proto.Uint32(islandFollowerResultInvalidShip)
		return client.SendMessage(21631, response)
	}

	maxFollowers, err := loadIslandMaxFollowerCount()
	if err != nil {
		return client.SendMessage(21631, response)
	}

	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		ship, err := orm.GetIslandShipForUpdateTx(context.Background(), tx, client.Commander.CommanderID, shipID)
		if err != nil {
			if !db.IsNotFound(err) {
				return err
			}
			response.Result = proto.Uint32(islandFollowerResultInvalidShip)
			return nil
		}

		followers, err := orm.ListIslandFollowersForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			return err
		}

		existingIdx := -1
		for i := range followers {
			if followers[i].ShipID == shipID {
				existingIdx = i
				break
			}
		}

		switch opType {
		case islandFollowerOpAdd:
			if !ship.CanFollow {
				response.Result = proto.Uint32(islandFollowerResultInvalidShip)
				return nil
			}
			if existingIdx >= 0 {
				response.Result = proto.Uint32(islandFollowerResultSuccess)
				return nil
			}
			if maxFollowers > 0 && len(followers) >= int(maxFollowers) {
				response.Result = proto.Uint32(islandFollowerResultMaxReached)
				return nil
			}
			var orderIdx uint32
			for i := range followers {
				if followers[i].OrderIdx >= orderIdx {
					orderIdx = followers[i].OrderIdx + 1
				}
			}
			if err := orm.AddIslandFollowerTx(context.Background(), tx, client.Commander.CommanderID, shipID, orderIdx); err != nil {
				return err
			}
		case islandFollowerOpRemove:
			if existingIdx < 0 {
				response.Result = proto.Uint32(islandFollowerResultSuccess)
				return nil
			}
			if err := orm.RemoveIslandFollowerTx(context.Background(), tx, client.Commander.CommanderID, shipID); err != nil {
				return err
			}
		}

		response.Result = proto.Uint32(islandFollowerResultSuccess)
		return nil
	})
	if err != nil {
		return client.SendMessage(21631, response)
	}

	return client.SendMessage(21631, response)
}

func loadIslandMaxFollowerCount() (uint32, error) {
	entry, err := orm.GetConfigEntry(islandFollowerConfigCategory, "max_follower_cnt")
	if err == nil {
		var cfg islandFollowerSetConfigEntry
		if unmarshalErr := json.Unmarshal(entry.Data, &cfg); unmarshalErr == nil {
			return cfg.KeyValueInt, nil
		}
	}

	entries, err := orm.ListConfigEntries(islandFollowerConfigCategory)
	if err != nil {
		return 0, err
	}
	for i := range entries {
		mapped := map[string]islandFollowerSetConfigEntry{}
		if unmarshalErr := json.Unmarshal(entries[i].Data, &mapped); unmarshalErr == nil {
			if cfg, ok := mapped["max_follower_cnt"]; ok {
				return cfg.KeyValueInt, nil
			}
		}
	}

	return 0, nil
}
