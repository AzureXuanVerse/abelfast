package answer

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	islandShopTemplateCategory   = "ShareCfg/island_shop_template.json"
	islandShopTemplateCategoryLC = "sharecfgdata/island_shop_template.json"
	islandShopNormalCategory     = "ShareCfg/island_shop_normal_template.json"
	islandShopNormalCategoryLC   = "sharecfgdata/island_shop_normal_template.json"
)

type islandShopTemplate struct {
	ID      uint32   `json:"id"`
	GoodsID []uint32 `json:"goods_id"`
}

type islandShopNormalTemplate struct {
	ID            uint32   `json:"id"`
	RefreshSet    uint32   `json:"refresh_set"`
	RefreshPlayer []uint32 `json:"refresh_player"`
	RefreshFree   uint32   `json:"refresh_free"`
	RefreshTime   uint32   `json:"refresh_time"`
	ExistTime     uint32   `json:"exist_time"`
}

func IslandShopPlayerRefresh(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21020
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21021, err
	}

	response := &protobuf.SC_21021{Result: proto.Uint32(1)}
	if client.Commander == nil {
		return client.SendMessage(21021, response)
	}

	shopID := payload.GetShopId()
	if shopID == 0 {
		return client.SendMessage(21021, response)
	}

	shopTemplate, ok := loadIslandShopTemplate(shopID)
	if !ok {
		return client.SendMessage(21021, response)
	}
	normalTemplate, ok := loadIslandShopNormalTemplate(shopID)
	if !ok {
		return client.SendMessage(21021, response)
	}

	state, err := orm.GetIslandShopState(client.Commander.CommanderID, shopID)
	if err != nil {
		if !db.IsNotFound(err) {
			return client.SendMessage(21021, response)
		}
		state = &orm.IslandShopState{CommanderID: client.Commander.CommanderID, ShopID: shopID, Goods: []orm.IslandShopGoodsState{}}
	}

	if normalTemplate.RefreshSet > 0 && state.RefreshCount >= normalTemplate.RefreshSet {
		return client.SendMessage(21021, response)
	}

	if err := ensureCommanderLoaded(client, "Island/ShopRefresh"); err != nil {
		return client.SendMessage(21021, response)
	}

	requiresCost := !(normalTemplate.RefreshFree == 1 && state.RefreshCount == 0)
	if requiresCost && len(normalTemplate.RefreshPlayer) >= 3 {
		dropType := normalTemplate.RefreshPlayer[0]
		dropID := normalTemplate.RefreshPlayer[1]
		dropCount := normalTemplate.RefreshPlayer[2]
		var consumeErr error
		switch dropType {
		case consts.DROP_TYPE_RESOURCE:
			consumeErr = client.Commander.ConsumeResource(dropID, dropCount)
		case consts.DROP_TYPE_ITEM:
			consumeErr = client.Commander.ConsumeItem(dropID, dropCount)
		default:
			consumeErr = nil
		}
		if consumeErr != nil {
			return client.SendMessage(21021, response)
		}
	}

	nowUnix := uint32(time.Now().UTC().Unix())
	state.RefreshCount++
	state.ExistTime = nowUnix + normalTemplate.ExistTime
	state.RefreshTime = nowUnix + normalTemplate.RefreshTime
	state.Goods = make([]orm.IslandShopGoodsState, 0, len(shopTemplate.GoodsID))
	for _, goodsID := range shopTemplate.GoodsID {
		state.Goods = append(state.Goods, orm.IslandShopGoodsState{ID: goodsID, Num: 0})
	}

	if err := orm.UpsertIslandShopState(state); err != nil {
		return client.SendMessage(21021, response)
	}

	goods := make([]*protobuf.PB_GOODS, 0, len(state.Goods))
	for _, item := range state.Goods {
		goods = append(goods, &protobuf.PB_GOODS{Id: proto.Uint32(item.ID), Num: proto.Uint32(item.Num)})
	}

	response.Result = proto.Uint32(0)
	response.ShopInfo = &protobuf.PB_SHOP{
		Id:           proto.Uint32(shopID),
		ExistTime:    proto.Uint32(state.ExistTime),
		RefreshTime:  proto.Uint32(state.RefreshTime),
		GoodsList:    goods,
		RefreshCount: proto.Uint32(state.RefreshCount),
	}
	return client.SendMessage(21021, response)
}

func loadIslandShopTemplate(shopID uint32) (*islandShopTemplate, bool) {
	if template, ok := loadIslandShopTemplateFromCategory(islandShopTemplateCategory, shopID); ok {
		return template, true
	}
	return loadIslandShopTemplateFromCategory(islandShopTemplateCategoryLC, shopID)
}

func loadIslandShopTemplateFromCategory(category string, shopID uint32) (*islandShopTemplate, bool) {
	entry, err := orm.GetConfigEntry(category, strconv.FormatUint(uint64(shopID), 10))
	if err == nil {
		var template islandShopTemplate
		if json.Unmarshal(entry.Data, &template) == nil {
			if template.ID == 0 {
				template.ID = shopID
			}
			return &template, true
		}
	}
	entries, err := orm.ListConfigEntries(category)
	if err != nil {
		return nil, false
	}
	for _, row := range entries {
		var template islandShopTemplate
		if json.Unmarshal(row.Data, &template) == nil {
			if template.ID == shopID {
				return &template, true
			}
		}
	}
	return nil, false
}

func loadIslandShopNormalTemplate(shopID uint32) (*islandShopNormalTemplate, bool) {
	if template, ok := loadIslandShopNormalTemplateFromCategory(islandShopNormalCategory, shopID); ok {
		return template, true
	}
	return loadIslandShopNormalTemplateFromCategory(islandShopNormalCategoryLC, shopID)
}

func loadIslandShopNormalTemplateFromCategory(category string, shopID uint32) (*islandShopNormalTemplate, bool) {
	entry, err := orm.GetConfigEntry(category, strconv.FormatUint(uint64(shopID), 10))
	if err == nil {
		var template islandShopNormalTemplate
		if json.Unmarshal(entry.Data, &template) == nil {
			if template.ID == 0 {
				template.ID = shopID
			}
			return &template, true
		}
	}
	entries, err := orm.ListConfigEntries(category)
	if err != nil {
		return nil, false
	}
	for _, row := range entries {
		var template islandShopNormalTemplate
		if json.Unmarshal(row.Data, &template) == nil {
			if template.ID == shopID {
				return &template, true
			}
		}
	}
	return nil, false
}
