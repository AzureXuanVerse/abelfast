package answer

import (
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func IslandShopGetData(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21016
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21017, err
	}

	response := &protobuf.SC_21017{Result: proto.Uint32(1)}
	if client.Commander == nil || payload.GetShopId() == 0 {
		return client.SendMessage(21017, response)
	}

	shopTemplate, ok := loadIslandShopTemplate(payload.GetShopId())
	if !ok {
		return client.SendMessage(21017, response)
	}
	normalTemplate, ok := loadIslandShopNormalTemplate(payload.GetShopId())
	if !ok {
		return client.SendMessage(21017, response)
	}

	state, err := orm.GetIslandShopState(client.Commander.CommanderID, payload.GetShopId())
	if err != nil {
		if !db.IsNotFound(err) {
			return client.SendMessage(21017, response)
		}
		now := uint32(time.Now().UTC().Unix())
		state = &orm.IslandShopState{
			CommanderID:  client.Commander.CommanderID,
			ShopID:       payload.GetShopId(),
			ExistTime:    now + normalTemplate.ExistTime,
			RefreshTime:  now + normalTemplate.RefreshTime,
			RefreshCount: 0,
			Goods:        make([]orm.IslandShopGoodsState, 0, len(shopTemplate.GoodsID)),
		}
		for _, goodsID := range shopTemplate.GoodsID {
			state.Goods = append(state.Goods, orm.IslandShopGoodsState{ID: goodsID, Num: 0})
		}
		if err := orm.UpsertIslandShopState(state); err != nil {
			return client.SendMessage(21017, response)
		}
	}

	response.Result = proto.Uint32(0)
	response.ShopInfo = buildPBShop(state)
	return client.SendMessage(21017, response)
}
