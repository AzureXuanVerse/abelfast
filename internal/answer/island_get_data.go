package answer

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	islandSetCategory            = "ShareCfg/island_set.json"
	islandSetCategoryLC          = "sharecfgdata/island_set.json"
	islandSetInitialScene        = "initial_scene"
	islandSetInitialVisitorScene = "initial_visitor_scene"
)

func IslandGetData(buffer *[]byte, client *connection.Client) (int, int, error) {
	var payload protobuf.CS_21200
	if err := proto.Unmarshal(*buffer, &payload); err != nil {
		return 0, 21201, err
	}

	if client.Commander == nil {
		return 0, 21201, fmt.Errorf("missing commander")
	}

	targetIslandID := payload.GetIslandId()
	if targetIslandID == 0 {
		targetIslandID = client.Commander.CommanderID
	}

	isSelf := targetIslandID == client.Commander.CommanderID
	snapshot, err := orm.GetIslandSnapshot(targetIslandID)
	if err != nil && !db.IsNotFound(err) {
		return 0, 21201, err
	}
	if snapshot == nil {
		snapshot = defaultIslandSnapshot(targetIslandID)
	}

	publicData := buildIslandPublicData(targetIslandID, snapshot)
	island := &protobuf.PB_ISLAND{PublicData: publicData}
	if isSelf {
		privateData, err := buildIslandPrivateData(targetIslandID, snapshot)
		if err != nil {
			return 0, 21201, err
		}
		island.PrivateData = privateData
	}

	mapID := snapshot.MapID
	if mapID == 0 {
		mapID = islandDefaultMapID(isSelf)
	}
	response := &protobuf.SC_21201{
		Island: island,
		PlayerPosition: &protobuf.PB_PLAYER_POS_RECORD{
			MapId:    proto.Uint32(mapID),
			Position: newIslandVector3(snapshot.PositionX, snapshot.PositionY, snapshot.PositionZ),
			Rotation: newIslandVector3(snapshot.RotationX, snapshot.RotationY, snapshot.RotationZ),
		},
	}
	return client.SendMessage(21201, response)
}

func buildIslandPublicData(ownerID uint32, snapshot *orm.IslandSnapshot) *protobuf.PB_ISLAND_PUBLIC {
	techState, err := orm.GetIslandTechnologyState(ownerID)
	if err != nil {
		techState = orm.NewIslandTechnologyState(ownerID)
	}
	sort.Slice(techState.UnlockedTechIDs, func(i, j int) bool { return techState.UnlockedTechIDs[i] < techState.UnlockedTechIDs[j] })
	sort.Slice(techState.AbilityIDs, func(i, j int) bool { return techState.AbilityIDs[i] < techState.AbilityIDs[j] })

	repeatFinish := make([]*protobuf.PB_REPEAT_FINISH, 0, len(techState.FinishCounts))
	for techID, count := range techState.FinishCounts {
		repeatFinish = append(repeatFinish, &protobuf.PB_REPEAT_FINISH{Id: proto.Uint32(techID), Num: proto.Uint32(count)})
	}
	sort.Slice(repeatFinish, func(i, j int) bool { return repeatFinish[i].GetId() < repeatFinish[j].GetId() })

	placedData := &protobuf.PB_PLACEMENT_DATA{PlacedList: []*protobuf.PB_FURNITURE_DATA{}, FloorData: []uint32{}, TileData: []uint32{}}
	if placement, err := orm.GetIslandAgoraPlacement(ownerID); err == nil && len(placement.PlacedData) > 0 {
		decoded := &protobuf.PB_PLACEMENT_DATA{}
		if err := proto.Unmarshal(placement.PlacedData, decoded); err == nil {
			placedData = decoded
		}
	}

	return &protobuf.PB_ISLAND_PUBLIC{
		Id:                 proto.Uint32(ownerID),
		Level:              proto.Uint32(maxUint32(snapshot.Level, 1)),
		Exp:                proto.Uint32(snapshot.Exp),
		StorageLevel:       proto.Uint32(maxUint32(snapshot.StorageLevel, 1)),
		Name:               proto.String(defaultIslandName(snapshot.Name, ownerID)),
		Tech:               &protobuf.PB_ISLAND_TECH{FinishList: techState.UnlockedTechIDs, RepeatFinishList: repeatFinish},
		Prosperity:         proto.Uint32(snapshot.Prosperity),
		AbilityList:        techState.AbilityIDs,
		ProsperityRewarded: []uint32{},
		ShipSys:            &protobuf.PB_ISLAND_SHIP_SYS{InviteList: []uint32{}, ShipList: []*protobuf.PB_ISLAND_SHIP{}, HadDress: []*protobuf.PB_ISLAND_DRESS_NUM{}, WearList: []*protobuf.PB_ISLAND_SHIP_WEAR{}, SkinList: []*protobuf.PB_ISLAND_SHIP_SKIN{}},
		AgoraLevel:         proto.Uint32(maxUint32(snapshot.AgoraLevel, 1)),
		PlacedData:         placedData,
		FlagList:           []uint32{},
		TreeGiftTimestamp:  proto.Uint32(0),
		TreeGiftCount:      proto.Uint32(0),
		TreeGiftInvited:    []uint32{},
		TreeGiftVisitor:    []uint32{},
		TaskInfo:           &protobuf.PB_ISLAND_TASK{TaskIdListFinish: []uint32{}, TaskList: []*protobuf.PB_TASK{}, FocusId: proto.Uint32(0), TaskListRandom: []*protobuf.PB_TASK_RANDOM{}, WeekDailyTaskNum: proto.Uint32(0)},
		TradeSys:           &protobuf.PB_ISLAND_TRADE_SYS{TodayEvent: proto.Uint32(0), TodayTrade: proto.Uint32(0), Effect: []*protobuf.PB_EVENT_EFFECT{}, TodayNum: []*protobuf.PB_TRADE_NUM{}, TradeList: []*protobuf.PB_ISLAND_TRADE{}, PresellList: []*protobuf.PB_TRADE_PRESELL{}},
		BuildList:          []*protobuf.PB_ISLAND_BUILD{},
	}
}

func buildIslandPrivateData(ownerID uint32, snapshot *orm.IslandSnapshot) (*protobuf.PB_ISLAND_PRIVATE, error) {
	dressStates, err := orm.ListIslandCommanderDressStates(ownerID)
	if err != nil {
		return nil, err
	}
	hadDress := make([]*protobuf.PB_ISLAND_DRESS_USER, 0, len(dressStates))
	for _, state := range dressStates {
		hadDress = append(hadDress, &protobuf.PB_ISLAND_DRESS_USER{
			Id:        proto.Uint32(state.DressID),
			State:     proto.Uint32(state.State),
			Color:     proto.Uint32(state.Color),
			ColorList: state.ColorList,
		})
	}

	season := &protobuf.PB_ISLAND_SEASON{Id: proto.Uint32(0), Pt: proto.Uint32(0), FetchList: []uint32{}, CountList: []*protobuf.KVDATA{}}
	if seasonState, err := orm.GetIslandSeason(ownerID); err == nil {
		season.Pt = proto.Uint32(seasonState.PT)
	}

	achievementSys := &protobuf.PB_ISLAND_ACHIEVEMENT_SYS{AchieveList: []*protobuf.PB_ISLAND_ACHIEVENT{}, FinishList: []uint32{}}
	achievementState, err := orm.GetIslandAchievementState(ownerID)
	if err != nil && !db.IsNotFound(err) {
		return nil, err
	}
	if achievementState != nil {
		achievementSys.AchieveList = buildIslandAchievementEventList(achievementState.ProgressEntries)
		achievementSys.FinishList = append([]uint32(nil), achievementState.FinishList...)
	}

	socialState, err := orm.GetCommanderIslandSocialState(ownerID)
	if err != nil && !db.IsNotFound(err) {
		return nil, err
	}
	whiteList := []uint32{}
	blackList := []uint32{}
	if socialState != nil {
		whiteList = append(whiteList, socialState.WhiteList...)
		blackList = append(blackList, socialState.BlackList...)
	}

	bookConds, err := orm.ListIslandBookConds(ownerID)
	if err != nil {
		return nil, err
	}
	bookCondByType := make(map[uint32][]uint32)
	for _, cond := range bookConds {
		bookCondByType[cond.Type] = append(bookCondByType[cond.Type], cond.UnlockID)
	}
	viewBookConds := make([]*protobuf.PB_BOOK_COND, 0, len(bookCondByType))
	for condType, unlockIDs := range bookCondByType {
		sort.Slice(unlockIDs, func(i, j int) bool { return unlockIDs[i] < unlockIDs[j] })
		viewBookConds = append(viewBookConds, &protobuf.PB_BOOK_COND{Type: proto.Uint32(condType), UnlockIds: unlockIDs})
	}
	sort.Slice(viewBookConds, func(i, j int) bool { return viewBookConds[i].GetType() < viewBookConds[j].GetType() })

	return &protobuf.PB_ISLAND_PRIVATE{
		OpenFlag:              proto.Uint32(snapshot.OpenFlag),
		WhiteList:             whiteList,
		BlackList:             blackList,
		VisitorHistory:        []*protobuf.PB_VISITOR{},
		ItemList:              []*protobuf.PB_ISLAND_ITEM{},
		ItemListCache:         []*protobuf.PB_ISLAND_ITEM{},
		FurnitureList:         []*protobuf.PB_FURNITURE{},
		ShopList:              buildIslandPrivateShopList(ownerID),
		OrderSystem:           &protobuf.PB_ISLAND_ORDER_SYSTEM{Favor: proto.Uint32(0), GetFavor_: []uint32{}, DailySelect: proto.Uint32(0), DailySlotNum: proto.Uint32(0), TimeSlotNum: proto.Uint32(0), SlotList: []*protobuf.PB_ISLAND_ORDER_SLOT{}, ShipSlotList: []*protobuf.PB_ISLAND_ORDER_SHIP_SLOT{}, SpeedList: []*protobuf.PB_SPEED_USE{}, ShipRefresh: proto.Uint32(0), AppointList: []*protobuf.PB_SHIP_ORDER_APPOINT{}, ActGroup: []*protobuf.PB_FINISH_ACT_GROUP{}},
		InviteCode:            proto.String(snapshot.InviteCode),
		DailyTimestamp:        proto.Uint32(snapshot.DailyTimestamp),
		DailyList:             []*protobuf.KVDATA{},
		SeasonReviewList:      []*protobuf.PB_ISLAND_SEASON_REVIEW{},
		Season:                season,
		CollectSys:            &protobuf.PB_ISLAND_COLLECT_SYS{CollectItem: []*protobuf.PB_ISLAND_COLLECT_ITEM{}, FinishList: []uint32{}},
		FormulaNum:            []*protobuf.PB_USE_FORMULA{},
		UserDress:             &protobuf.PB_ISLAND_USER_DRESS_SYS{CurDress: []*protobuf.PB_ISLAND_CUR_DRESS{}, HadDress: hadDress, CapList: []*protobuf.PB_CAP_STATE{}},
		AchievementSys:        achievementSys,
		GlobalBuff:            &protobuf.PB_ISLAND_GLOBAL_BUFF{ForeverList: []uint32{}, LimitList: []*protobuf.PB_ISLAND_BUFF{}},
		SpeedTickets:          []*protobuf.PB_SPEEDUP_TICKET{},
		ActionList:            []uint32{},
		ActionFeedbackNpcList: []uint32{},
		FlagList:              []*protobuf.PB_SET_FLAG{},
		ViewBook:              &protobuf.PB_VIEW_BOOK{CondList: viewBookConds, BookList: []uint32{}, BookAwards: []uint32{}, BookCollects: []*protobuf.PB_BOOK_COLLECT{}, ItemList: []*protobuf.PB_ISLAND_ITEM{}},
		FollowShips:           snapshot.FollowShips,
		ImageList:             []*protobuf.PB_CARD_IMAGE{},
		FishSys:               buildIslandFishSys(ownerID),
	}, nil
}

func buildIslandAchievementEventList(entries []orm.IslandAchievementProgressEntry) []*protobuf.PB_ISLAND_ACHIEVENT {
	if len(entries) == 0 {
		return []*protobuf.PB_ISLAND_ACHIEVENT{}
	}
	events := make([]*protobuf.PB_ISLAND_ACHIEVENT, 0, len(entries))
	for _, entry := range entries {
		events = append(events, &protobuf.PB_ISLAND_ACHIEVENT{
			EventType: proto.Uint32(entry.EventType),
			EventArg:  proto.Uint32(entry.EventArg),
			Value:     proto.Uint32(entry.Value),
		})
	}
	return events
}

func buildIslandFishSys(ownerID uint32) *protobuf.PB_FISH_SYS {
	state, err := orm.GetIslandFishingState(ownerID)
	if err != nil {
		return &protobuf.PB_FISH_SYS{OldBait: proto.Uint32(0), FishRod: proto.Uint32(0), FishWeight: []*protobuf.PB_FISH_WEIGHT{}}
	}
	weights := make([]*protobuf.PB_FISH_WEIGHT, 0, len(state.FishWeights))
	for i := range state.FishWeights {
		weights = append(weights, &protobuf.PB_FISH_WEIGHT{
			FishId:    proto.Uint32(state.FishWeights[i].FishID),
			MinWeight: proto.Uint32(state.FishWeights[i].MinWeight),
			MaxWeight: proto.Uint32(state.FishWeights[i].MaxWeight),
			GoldState: proto.Uint32(state.FishWeights[i].GoldState),
		})
	}
	return &protobuf.PB_FISH_SYS{
		OldBait:    proto.Uint32(state.BaitID),
		FishRod:    proto.Uint32(state.FishRod),
		FishWeight: weights,
	}
}

func buildIslandPrivateShopList(ownerID uint32) []*protobuf.PB_SHOP {
	shopStates, err := orm.ListIslandShopStates(ownerID)
	if err != nil {
		return []*protobuf.PB_SHOP{}
	}
	shops := make([]*protobuf.PB_SHOP, 0, len(shopStates))
	for _, shopState := range shopStates {
		goods := make([]*protobuf.PB_GOODS, 0, len(shopState.Goods))
		for _, entry := range shopState.Goods {
			goods = append(goods, &protobuf.PB_GOODS{Id: proto.Uint32(entry.ID), Num: proto.Uint32(entry.Num)})
		}
		shops = append(shops, &protobuf.PB_SHOP{
			Id:           proto.Uint32(shopState.ShopID),
			ExistTime:    proto.Uint32(shopState.ExistTime),
			RefreshTime:  proto.Uint32(shopState.RefreshTime),
			GoodsList:    goods,
			RefreshCount: proto.Uint32(shopState.RefreshCount),
		})
	}
	return shops
}

func defaultIslandSnapshot(ownerID uint32) *orm.IslandSnapshot {
	return &orm.IslandSnapshot{
		CommanderID:    ownerID,
		Name:           "",
		Level:          1,
		Exp:            0,
		StorageLevel:   1,
		Prosperity:     0,
		AgoraLevel:     1,
		MapID:          0,
		PositionX:      0,
		PositionY:      0,
		PositionZ:      0,
		RotationX:      0,
		RotationY:      0,
		RotationZ:      0,
		OpenFlag:       0,
		InviteCode:     "",
		DailyTimestamp: uint32(time.Now().UTC().Unix()),
		FollowShips:    []uint32{},
	}
}

func defaultIslandName(name string, ownerID uint32) string {
	if name != "" {
		return name
	}
	return fmt.Sprintf("Island %d", ownerID)
}

func newIslandVector3(x float32, y float32, z float32) *protobuf.PB_VECTOR3 {
	return &protobuf.PB_VECTOR3{X: proto.Float32(x), Y: proto.Float32(y), Z: proto.Float32(z)}
}

func islandDefaultMapID(isSelf bool) uint32 {
	if isSelf {
		return loadIslandSetInt(islandSetInitialScene, 1001)
	}
	return loadIslandSetInt(islandSetInitialVisitorScene, loadIslandSetInt(islandSetInitialScene, 1001))
}

func loadIslandSetInt(key string, fallback uint32) uint32 {
	if value, ok := loadIslandSetIntFromCategory(islandSetCategory, key); ok {
		return value
	}
	if value, ok := loadIslandSetIntFromCategory(islandSetCategoryLC, key); ok {
		return value
	}
	return fallback
}

func loadIslandSetIntFromCategory(category string, key string) (uint32, bool) {
	entry, err := orm.GetConfigEntry(category, key)
	if err == nil {
		if value, ok := parseIslandSetValue(entry.Data); ok {
			return value, true
		}
	}
	entries, err := orm.ListConfigEntries(category)
	if err != nil {
		return 0, false
	}
	for _, row := range entries {
		if row.Key != key {
			continue
		}
		if value, ok := parseIslandSetValue(row.Data); ok {
			return value, true
		}
	}
	return 0, false
}

func parseIslandSetValue(raw json.RawMessage) (uint32, bool) {
	var direct struct {
		KeyValueInt uint32 `json:"key_value_int"`
	}
	if err := json.Unmarshal(raw, &direct); err == nil {
		if direct.KeyValueInt > 0 {
			return direct.KeyValueInt, true
		}
	}

	var list []struct {
		ID          any    `json:"id"`
		KeyValueInt uint32 `json:"key_value_int"`
	}
	if err := json.Unmarshal(raw, &list); err == nil {
		for _, row := range list {
			if row.KeyValueInt > 0 {
				return row.KeyValueInt, true
			}
		}
	}
	return 0, false
}

func maxUint32(value uint32, floor uint32) uint32 {
	if value < floor {
		return floor
	}
	return value
}

func parseUint32Key(value string) uint32 {
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(parsed)
}
