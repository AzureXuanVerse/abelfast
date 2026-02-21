package answer

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
)

const (
	guildStoreConfigCategory     = "ShareCfg/guild_store.json"
	guildShopGoodsTypeFixed      = uint32(1)
	guildShopGoodsTypeSelectable = uint32(2)
	guildShopCoinResourceID      = uint32(8)

	guildShopPurchaseResultOK           = uint32(0)
	guildShopPurchaseResultInvalid      = uint32(1)
	guildShopPurchaseResultInsufficient = uint32(2)
	guildShopPurchaseResultStock        = uint32(3)
	guildShopPurchaseResultUnsupported  = uint32(4)
	guildShopPurchaseResultDBError      = uint32(5)
)

type guildStorePurchaseEntry struct {
	ID        uint32   `json:"id"`
	Price     uint32   `json:"price"`
	Goods     []uint32 `json:"goods"`
	GoodsType uint32   `json:"goods_type"`
	Num       uint32   `json:"num"`
	Type      uint32   `json:"type"`
}

func GuildShopPurchase(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_60035
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 60036, err
	}

	response := protobuf.SC_60036{Result: proto.Uint32(guildShopPurchaseResultInvalid)}
	goodsID := payload.GetGoodsid()
	index := payload.GetIndex()
	if goodsID == 0 || index == 0 {
		return client.SendMessage(60036, &response)
	}

	config, ok, err := loadGuildStorePurchaseEntry(goodsID)
	if err != nil {
		return 0, 60036, err
	}
	if !ok || len(config.Goods) == 0 || config.Num == 0 {
		return client.SendMessage(60036, &response)
	}

	rewards, totalUnits, valid := normalizeGuildShopSelection(config, payload.GetSelected())
	if !valid {
		return client.SendMessage(60036, &response)
	}

	totalCost := config.Price * totalUnits
	dropType, ok := mapGuildShopDropType(config.Type)
	if !ok {
		response.Result = proto.Uint32(guildShopPurchaseResultUnsupported)
		return client.SendMessage(60036, &response)
	}

	errInvalid := errors.New("invalid")
	errInsufficient := errors.New("insufficient")
	errStock := errors.New("stock")
	errUnsupported := errors.New("unsupported")

	ctx := context.Background()
	commanderID := client.Commander.CommanderID
	err = orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		var rowGoodsID uint32
		var rowCount uint32
		if err := tx.QueryRow(ctx, `
SELECT goods_id, count
FROM guild_shop_goods
WHERE commander_id = $1 AND "index" = $2
`, int64(commanderID), int64(index)).Scan(&rowGoodsID, &rowCount); err != nil {
			err = db.MapNotFound(err)
			if db.IsNotFound(err) {
				return errInvalid
			}
			return err
		}
		if rowGoodsID != goodsID {
			return errInvalid
		}
		if rowCount < totalUnits {
			return errStock
		}
		if !client.Commander.HasEnoughResource(guildShopCoinResourceID, totalCost) {
			return errInsufficient
		}
		if err := client.Commander.ConsumeResourceTx(ctx, tx, guildShopCoinResourceID, totalCost); err != nil {
			return errInsufficient
		}

		result, err := tx.Exec(ctx, `
UPDATE guild_shop_goods
SET count = count - $4
WHERE commander_id = $1 AND "index" = $2 AND goods_id = $3 AND count >= $4
`, int64(commanderID), int64(index), int64(goodsID), int64(totalUnits))
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return errStock
		}

		dropList := make([]*protobuf.DROPINFO, 0, len(rewards))
		for id, units := range rewards {
			rewardAmount := config.Num * units
			switch dropType {
			case consts.DROP_TYPE_ITEM:
				if err := client.Commander.AddItemTx(ctx, tx, id, rewardAmount); err != nil {
					return err
				}
			case consts.DROP_TYPE_SHIP:
				for i := uint32(0); i < rewardAmount; i++ {
					if _, err := client.Commander.AddShipTx(ctx, tx, id); err != nil {
						return err
					}
				}
			default:
				return errUnsupported
			}
			dropList = append(dropList, &protobuf.DROPINFO{
				Type:   proto.Uint32(dropType),
				Id:     proto.Uint32(id),
				Number: proto.Uint32(rewardAmount),
			})
		}
		response.DropList = dropList
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, errInvalid):
			response.Result = proto.Uint32(guildShopPurchaseResultInvalid)
		case errors.Is(err, errInsufficient):
			response.Result = proto.Uint32(guildShopPurchaseResultInsufficient)
		case errors.Is(err, errStock):
			response.Result = proto.Uint32(guildShopPurchaseResultStock)
		case errors.Is(err, errUnsupported):
			response.Result = proto.Uint32(guildShopPurchaseResultUnsupported)
		default:
			response.Result = proto.Uint32(guildShopPurchaseResultDBError)
		}
		response.DropList = nil
		_ = client.Commander.Load()
		return client.SendMessage(60036, &response)
	}

	response.Result = proto.Uint32(guildShopPurchaseResultOK)
	return client.SendMessage(60036, &response)
}

func normalizeGuildShopSelection(config *guildStorePurchaseEntry, selected []*protobuf.GUILD_SHOP_INFO) (map[uint32]uint32, uint32, bool) {
	if config.GoodsType != guildShopGoodsTypeFixed && config.GoodsType != guildShopGoodsTypeSelectable {
		return nil, 0, false
	}

	rewards := make(map[uint32]uint32)
	totalUnits := uint32(0)

	if len(selected) == 0 {
		if config.GoodsType != guildShopGoodsTypeFixed {
			return nil, 0, false
		}
		for _, id := range config.Goods {
			if id == 0 {
				return nil, 0, false
			}
			rewards[id] += 1
		}
		if len(rewards) == 0 {
			return nil, 0, false
		}
		return rewards, 1, true
	}

	if config.GoodsType == guildShopGoodsTypeFixed {
		return nil, 0, false
	}

	for _, pick := range selected {
		id := pick.GetId()
		count := pick.GetCount()
		if id == 0 || count == 0 {
			return nil, 0, false
		}
		if !containsUint32(config.Goods, id) {
			return nil, 0, false
		}
		rewards[id] += count
		totalUnits += count
	}

	if totalUnits == 0 {
		return nil, 0, false
	}
	return rewards, totalUnits, true
}

func mapGuildShopDropType(configType uint32) (uint32, bool) {
	switch configType {
	case consts.DROP_TYPE_ITEM:
		return consts.DROP_TYPE_ITEM, true
	case consts.DROP_TYPE_SHIP:
		return consts.DROP_TYPE_SHIP, true
	default:
		return 0, false
	}
}

func loadGuildStorePurchaseEntry(goodsID uint32) (*guildStorePurchaseEntry, bool, error) {
	entry, err := orm.GetConfigEntry(guildStoreConfigCategory, strconv.FormatUint(uint64(goodsID), 10))
	if err != nil && !db.IsNotFound(err) {
		return nil, false, err
	}
	if err == nil {
		var config guildStorePurchaseEntry
		if err := json.Unmarshal(entry.Data, &config); err != nil {
			return nil, false, err
		}
		if config.ID == goodsID {
			return &config, true, nil
		}
	}

	entries, err := orm.ListConfigEntries(guildStoreConfigCategory)
	if err != nil {
		return nil, false, err
	}
	for _, configEntry := range entries {
		var config guildStorePurchaseEntry
		if err := json.Unmarshal(configEntry.Data, &config); err != nil {
			var list []guildStorePurchaseEntry
			if err := json.Unmarshal(configEntry.Data, &list); err != nil {
				continue
			}
			for i := range list {
				if list[i].ID == goodsID {
					return &list[i], true, nil
				}
			}
			continue
		}
		if config.ID == goodsID {
			return &config, true, nil
		}
	}

	return nil, false, nil
}
