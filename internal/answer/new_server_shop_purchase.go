package answer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func NewServerShopPurchase(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_26043
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 26044, err
	}

	response := &protobuf.SC_26044{Result: proto.Uint32(newServerShopResultFailed), DropList: []*protobuf.DROPINFO{}}
	if client.Commander == nil || payload.GetActId() == 0 || payload.GetGoodsid() == 0 {
		return client.SendMessage(26044, response)
	}

	activity, active, err := loadNewServerShopActivity(payload.GetActId(), time.Now().UTC())
	if err != nil {
		return 0, 26044, err
	}
	if !active {
		return client.SendMessage(26044, response)
	}

	entry, ok := activity.GoodsByID[payload.GetGoodsid()]
	if !ok {
		return client.SendMessage(26044, response)
	}

	err = orm.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.GetNewServerShopStateTx(context.Background(), tx, client.Commander.CommanderID, payload.GetActId())
		if err != nil {
			if !db.IsNotFound(err) {
				return err
			}
			state = defaultNewServerShopState(client.Commander.CommanderID, payload.GetActId(), activity.Goods)
			if err := orm.UpsertNewServerShopStateTx(context.Background(), tx, state); err != nil {
				return err
			}
			state, err = orm.GetNewServerShopStateTx(context.Background(), tx, client.Commander.CommanderID, payload.GetActId())
			if err != nil {
				return err
			}
		}

		if normalizeNewServerShopState(state, activity.Goods) {
			if err := orm.UpsertNewServerShopStateTx(context.Background(), tx, state); err != nil {
				return err
			}
		}

		goodsIndex := -1
		for i := range state.Goods {
			if state.Goods[i].ID == entry.ID {
				goodsIndex = i
				break
			}
		}
		if goodsIndex < 0 {
			return fmt.Errorf("missing goods state")
		}

		selectedByItem := make(map[uint32]uint32)
		for _, selected := range payload.GetSelected() {
			if selected == nil || selected.GetItemid() == 0 || selected.GetCount() == 0 {
				continue
			}
			selectedByItem[selected.GetItemid()] += selected.GetCount()
		}

		purchaseCount := uint32(1)
		drops := make([]*protobuf.DROPINFO, 0)

		switch entry.GoodsType {
		case newServerShopGoodsTypeFixed:
			if len(selectedByItem) > 0 || len(entry.Goods) == 0 {
				return fmt.Errorf("invalid fixed purchase")
			}
			drops = append(drops, newDropInfo(entry.Type, entry.Goods[0], entry.Num))
		default:
			if len(selectedByItem) == 0 {
				return fmt.Errorf("missing selected goods")
			}
			allowed := make(map[uint32]struct{}, len(entry.Goods))
			for _, itemID := range entry.Goods {
				allowed[itemID] = struct{}{}
			}

			boughtSet := make(map[uint32]struct{}, len(state.Goods[goodsIndex].BoughtRecord))
			for _, itemID := range state.Goods[goodsIndex].BoughtRecord {
				boughtSet[itemID] = struct{}{}
			}

			itemIDs := make([]uint32, 0, len(selectedByItem))
			for itemID, count := range selectedByItem {
				if _, ok := allowed[itemID]; !ok {
					return fmt.Errorf("selected goods not allowed")
				}
				if entry.GoodsType == newServerShopGoodsTypeSelectable {
					if count != 1 {
						return fmt.Errorf("invalid selectable count")
					}
					if _, seen := boughtSet[itemID]; seen {
						return fmt.Errorf("already bought")
					}
				}
				purchaseCount += count
				itemIDs = append(itemIDs, itemID)
				drops = append(drops, newDropInfo(entry.Type, itemID, entry.Num*count))
			}
			purchaseCount--

			if entry.GoodsType == newServerShopGoodsTypeSelectable {
				state.Goods[goodsIndex].BoughtRecord = append(state.Goods[goodsIndex].BoughtRecord, itemIDs...)
				state.Goods[goodsIndex].BoughtRecord = sortedUniqueUint32(state.Goods[goodsIndex].BoughtRecord)
			}
		}

		if purchaseCount == 0 || state.Goods[goodsIndex].Count < purchaseCount {
			return fmt.Errorf("purchase limit exceeded")
		}

		if err := consumeNewServerShopCostTx(context.Background(), tx, client, entry, purchaseCount); err != nil {
			return err
		}
		if err := applyNewServerShopDropsTx(context.Background(), tx, client, drops); err != nil {
			return err
		}

		state.Goods[goodsIndex].Count -= purchaseCount
		if err := orm.UpsertNewServerShopStateTx(context.Background(), tx, state); err != nil {
			return err
		}

		response.Result = proto.Uint32(newServerShopResultOK)
		response.DropList = mergeDropList(drops)
		return nil
	})
	if err != nil {
		message := strings.ToLower(err.Error())
		switch {
		case strings.Contains(message, "insufficient"):
			response.Result = proto.Uint32(newServerShopResultInsufficient)
		case strings.Contains(message, "limit") || strings.Contains(message, "selected") || strings.Contains(message, "already bought"):
			response.Result = proto.Uint32(newServerShopResultLimit)
		default:
			response.Result = proto.Uint32(newServerShopResultUnsupported)
		}
		response.DropList = nil
		_ = client.Commander.Load()
	}

	return client.SendMessage(26044, response)
}
