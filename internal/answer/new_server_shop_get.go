package answer

import (
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func GetNewServerShop(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_26041
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 26042, err
	}

	response := &protobuf.SC_26042{Result: proto.Uint32(newServerShopResultFailed), StartTime: proto.Uint32(0), StopTime: proto.Uint32(0), Goods: []*protobuf.ACT_GOODS_INFO{}}
	if client.Commander == nil || payload.GetActId() == 0 {
		return client.SendMessage(26042, response)
	}

	activity, active, err := loadNewServerShopActivity(payload.GetActId(), time.Now().UTC())
	if err != nil {
		return 0, 26042, err
	}
	if !active {
		return client.SendMessage(26042, response)
	}

	state, err := orm.GetNewServerShopState(client.Commander.CommanderID, payload.GetActId())
	if err != nil {
		if !db.IsNotFound(err) {
			return 0, 26042, err
		}
		state = defaultNewServerShopState(client.Commander.CommanderID, payload.GetActId(), activity.Goods)
		if err := orm.UpsertNewServerShopState(state); err != nil {
			return 0, 26042, err
		}
	}

	if normalizeNewServerShopState(state, activity.Goods) {
		if err := orm.UpsertNewServerShopState(state); err != nil {
			return 0, 26042, err
		}
	}

	response.Result = proto.Uint32(newServerShopResultOK)
	response.StartTime = proto.Uint32(activity.StartTime)
	response.StopTime = proto.Uint32(activity.StopTime)
	response.Goods = newServerShopResponseGoods(activity, state)
	return client.SendMessage(26042, response)
}
