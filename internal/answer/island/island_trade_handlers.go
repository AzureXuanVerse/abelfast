package island

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/logger"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

const (
	islandTradePurchaseType = uint32(1)
	islandTradeSellType     = uint32(2)

	islandTradeResultOK       = uint32(0)
	islandTradeResultInvalid  = uint32(1)
	islandTradeResultLack     = uint32(2)
	islandTradeResultLimit    = uint32(3)
	islandTradeResultPersist  = uint32(5)
	islandTradeDefaultLevel   = uint32(1)
	islandTradeGoldItemID     = uint32(1)
	islandTradePearlItemID    = uint32(9900)
	islandTradeConfigLimitKey = "treasure_week_limit"
	islandTradeConfigBuyKey   = "treasure_price_buy"
	islandTradeConfigInitKey  = "treasure_price_initial"
)

type islandTradeConfigValue struct {
	KeyValueInt     uint32          `json:"key_value_int"`
	KeyValue        []uint32        `json:"key_value"`
	KeyValueVarchar json.RawMessage `json:"key_value_varchar"`
}

type islandTradeSettings struct {
	BuyLimit      uint32
	SellLimit     uint32
	BuyPercentage uint32
	InitialPrice  uint32
}

func IslandTradeOp(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21240
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21241, err
	}

	response := &protobuf.SC_21241{Result: proto.Uint32(islandTradeResultInvalid), DropList: []*protobuf.DROPINFO{}}
	if payload.GetIslandId() == 0 || payload.GetNum() == 0 {
		return client.SendMessage(21241, response)
	}
	if payload.GetType() != islandTradePurchaseType && payload.GetType() != islandTradeSellType {
		return client.SendMessage(21241, response)
	}

	settings, err := loadIslandTradeSettings()
	if err != nil {
		response.Result = proto.Uint32(islandTradeResultPersist)
		return client.SendMessage(21241, response)
	}

	err = db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		state, err := orm.GetIslandTreasureStateForUpdateTx(context.Background(), tx, client.Commander.CommanderID)
		if err != nil {
			response.Result = proto.Uint32(islandTradeResultPersist)
			return err
		}

		now := time.Now().UTC()
		todayTimestamp := currentDayStartUnix(now)
		todayPrice := currentIslandTreasurePrice(state, settings.InitialPrice)
		if todayPrice == 0 {
			response.Result = proto.Uint32(islandTradeResultInvalid)
			return nil
		}

		switch payload.GetType() {
		case islandTradePurchaseType:
			if state.WeekBuyNum+payload.GetNum() > settings.BuyLimit {
				response.Result = proto.Uint32(islandTradeResultLimit)
				return nil
			}
			unitCost := uint32(math.Floor(float64(todayPrice) * float64(settings.BuyPercentage) / 100.0))
			totalCost, ok := mulUint32Checked(unitCost, payload.GetNum())
			if !ok {
				response.Result = proto.Uint32(islandTradeResultInvalid)
				return nil
			}
			if err := orm.ConsumeIslandInventoryCheckedTx(context.Background(), tx, client.Commander.CommanderID, islandTradeGoldItemID, totalCost); err != nil {
				if db.IsNotFound(err) {
					response.Result = proto.Uint32(islandTradeResultLack)
					return nil
				}
				response.Result = proto.Uint32(islandTradeResultPersist)
				return err
			}
			if err := orm.AddIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, islandTradePearlItemID, payload.GetNum()); err != nil {
				response.Result = proto.Uint32(islandTradeResultPersist)
				return err
			}
			state.WeekBuyNum += payload.GetNum()
			response.DropList = []*protobuf.DROPINFO{newDropInfo(consts.DROP_TYPE_ISLAND_ITEM, islandTradePearlItemID, payload.GetNum())}
			response.Result = proto.Uint32(islandTradeResultOK)
		case islandTradeSellType:
			if payload.GetIslandId() != client.Commander.CommanderID {
				if state.SellCount(payload.GetIslandId())+payload.GetNum() > settings.SellLimit {
					response.Result = proto.Uint32(islandTradeResultLimit)
					return nil
				}
			}
			if err := orm.ConsumeIslandInventoryCheckedTx(context.Background(), tx, client.Commander.CommanderID, islandTradePearlItemID, payload.GetNum()); err != nil {
				if db.IsNotFound(err) {
					response.Result = proto.Uint32(islandTradeResultLack)
					return nil
				}
				response.Result = proto.Uint32(islandTradeResultPersist)
				return err
			}
			payout, ok := mulUint32Checked(todayPrice, payload.GetNum())
			if !ok {
				response.Result = proto.Uint32(islandTradeResultInvalid)
				return nil
			}
			if err := orm.AddIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, islandTradeGoldItemID, payout); err != nil {
				response.Result = proto.Uint32(islandTradeResultPersist)
				return err
			}
			if payload.GetIslandId() != client.Commander.CommanderID {
				state.AddSellCount(payload.GetIslandId(), payload.GetNum())
			}
			response.DropList = []*protobuf.DROPINFO{newDropInfo(consts.DROP_TYPE_ISLAND_ITEM, islandTradeGoldItemID, payout)}
			response.Result = proto.Uint32(islandTradeResultOK)
		}

		if response.GetResult() == islandTradeResultOK {
			state.UpsertPrice(todayTimestamp, todayPrice)
			if err := orm.UpsertIslandTreasureStateTx(context.Background(), tx, state); err != nil {
				response.Result = proto.Uint32(islandTradeResultPersist)
				return err
			}
		}

		return nil
	})
	if err != nil {
		return client.SendMessage(21241, response)
	}

	return client.SendMessage(21241, response)
}

func IslandGetFriendTradeRank(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21243
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21244, err
	}

	targetIslandID := payload.GetIslandId()
	if targetIslandID == 0 {
		targetIslandID = client.Commander.CommanderID
	}

	level := islandTradeDefaultLevel
	snapshot, err := orm.GetIslandSnapshot(targetIslandID)
	if err == nil && snapshot != nil {
		level = maxUint32(snapshot.Level, islandTradeDefaultLevel)
	}

	settings, settingsErr := loadIslandTradeSettings()
	defaultPrice := uint32(0)
	if settingsErr == nil {
		defaultPrice = settings.InitialPrice
	}
	todayPrice, usedFallback := loadIslandTradePrice(targetIslandID, defaultPrice)
	if usedFallback {
		logger.WithFields(
			"Island/GetFriendTradeRank",
			logger.FieldValue("target_island_id", targetIslandID),
			logger.FieldValue("price_fallback", true),
		).Info("using fallback island trade price")
	}

	response := &protobuf.SC_21244{
		TodayPrice: &protobuf.PB_TRE_HISTORY_PRICE{
			Timestamp: proto.Uint32(uint32(time.Now().UTC().Unix())),
			Price:     proto.Uint32(todayPrice),
		},
		IslandLv: proto.Uint32(level),
	}
	return client.SendMessage(21244, response)
}

func loadIslandTradePrice(targetIslandID uint32, defaultPrice uint32) (uint32, bool) {
	state, err := orm.GetIslandTreasureState(targetIslandID)
	if err != nil || state == nil || len(state.PriceList) == 0 {
		return defaultPrice, true
	}
	return currentIslandTreasurePrice(state, defaultPrice), false
}

func currentIslandTreasurePrice(state *orm.IslandTreasureState, fallback uint32) uint32 {
	if state == nil || len(state.PriceList) == 0 {
		return fallback
	}
	sort.Slice(state.PriceList, func(i int, j int) bool {
		return state.PriceList[i].Timestamp < state.PriceList[j].Timestamp
	})
	price := state.PriceList[len(state.PriceList)-1].Price
	if price == 0 {
		return fallback
	}
	return price
}

func loadIslandTradeSettings() (islandTradeSettings, error) {
	limits, err := loadIslandTradeConfigList(islandTradeConfigLimitKey)
	if err != nil {
		return islandTradeSettings{}, err
	}
	if len(limits) < 2 {
		return islandTradeSettings{}, db.ErrNotFound
	}
	buyRate, err := loadIslandTradeConfigUint(islandTradeConfigBuyKey)
	if err != nil {
		return islandTradeSettings{}, err
	}
	if buyRate == 0 {
		return islandTradeSettings{}, db.ErrNotFound
	}
	initialPrice, err := loadIslandTradeConfigUint(islandTradeConfigInitKey)
	if err != nil {
		return islandTradeSettings{}, err
	}
	return islandTradeSettings{
		BuyLimit:      limits[0],
		SellLimit:     limits[1],
		BuyPercentage: buyRate,
		InitialPrice:  initialPrice,
	}, nil
}

func loadIslandTradeConfigList(key string) ([]uint32, error) {
	entry, err := getIslandTradeConfigEntry(key)
	if err != nil {
		return nil, err
	}
	if len(entry.KeyValue) > 0 {
		return entry.KeyValue, nil
	}
	if len(entry.KeyValueVarchar) > 0 {
		list, ok := decodeIslandTradeConfigList(entry.KeyValueVarchar)
		if ok {
			return list, nil
		}
	}
	return nil, db.ErrNotFound
}

func loadIslandTradeConfigUint(key string) (uint32, error) {
	entry, err := getIslandTradeConfigEntry(key)
	if err != nil {
		return 0, err
	}
	if entry.KeyValueInt != 0 {
		return entry.KeyValueInt, nil
	}
	if len(entry.KeyValue) > 0 {
		return entry.KeyValue[0], nil
	}
	if len(entry.KeyValueVarchar) > 0 {
		list, ok := decodeIslandTradeConfigList(entry.KeyValueVarchar)
		if ok && len(list) > 0 {
			return list[0], nil
		}
	}
	return 0, db.ErrNotFound
}

func getIslandTradeConfigEntry(key string) (*islandTradeConfigValue, error) {
	entry, err := orm.GetConfigEntry(islandSetCategory, key)
	if err != nil {
		entry, err = orm.GetConfigEntry(islandSetCategoryLC, key)
		if err != nil {
			return nil, err
		}
	}
	cfg := &islandTradeConfigValue{}
	if err := json.Unmarshal(entry.Data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func decodeIslandTradeConfigList(raw json.RawMessage) ([]uint32, bool) {
	var direct []uint32
	if err := json.Unmarshal(raw, &direct); err == nil && len(direct) > 0 {
		return direct, true
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return nil, false
	}
	var list []uint32
	if err := json.Unmarshal([]byte(text), &list); err == nil && len(list) > 0 {
		return list, true
	}
	if value, err := strconv.ParseUint(text, 10, 32); err == nil {
		return []uint32{uint32(value)}, true
	}
	return nil, false
}
