package answer

import (
	"math"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestIslandGetDataSelfIncludesPrivateFollowShips(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandSnapshot{})
	clearTable(t, &orm.IslandTechnologyState{})
	clearTable(t, &orm.IslandCommanderDressState{})
	clearTable(t, &orm.IslandShopState{})

	if err := orm.UpsertIslandSnapshot(&orm.IslandSnapshot{CommanderID: client.Commander.CommanderID, Name: "Home", Level: 3, StorageLevel: 2, FollowShips: []uint32{1001, 1002}}); err != nil {
		t.Fatalf("seed island snapshot: %v", err)
	}

	payload := &protobuf.CS_21200{IslandId: proto.Uint32(client.Commander.CommanderID)}
	buffer, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := IslandGetData(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_21201
	decodePacketAt(t, client, 0, 21201, &response)
	if response.GetIsland().GetPrivateData() == nil {
		t.Fatalf("expected private data")
	}
	if len(response.GetIsland().GetPrivateData().GetFollowShips()) != 2 {
		t.Fatalf("expected persisted follow ships, got %+v", response.GetIsland().GetPrivateData().GetFollowShips())
	}
	if response.GetPlayerPosition().GetMapId() == 0 {
		t.Fatalf("expected non-zero fallback map id")
	}
}

func TestIslandSignInGiftClaimSignInAndClaim(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandSignInState{})

	seedConfigEntry(t, islandSetCategory, "daily_gift_drop_num", `{"key_value_int":6}`)
	seedConfigEntry(t, islandSetCategory, "daily_gift_get_max", `{"key_value_int":3}`)
	seedConfigEntry(t, islandSetCategory, "daily_gift", `{"key_value_int":20001}`)

	signInPayload := &protobuf.CS_21310{IslandId: proto.Uint32(0), Pos: proto.Uint32(0)}
	buffer, _ := proto.Marshal(signInPayload)
	client.Buffer.Reset()
	if _, _, err := IslandSignInGiftClaim(&buffer, client); err != nil {
		t.Fatalf("sign-in failed: %v", err)
	}
	var signInResponse protobuf.SC_21311
	decodePacketAt(t, client, 0, 21311, &signInResponse)
	if signInResponse.GetResult() != 0 {
		t.Fatalf("expected sign-in success")
	}

	claimPayload := &protobuf.CS_21310{IslandId: proto.Uint32(client.Commander.CommanderID), Pos: proto.Uint32(1)}
	buffer, _ = proto.Marshal(claimPayload)
	client.Buffer.Reset()
	if _, _, err := IslandSignInGiftClaim(&buffer, client); err != nil {
		t.Fatalf("claim failed: %v", err)
	}
	var claimResponse protobuf.SC_21311
	decodePacketAt(t, client, 0, 21311, &claimResponse)
	if claimResponse.GetResult() != 0 || len(claimResponse.GetDropList()) != 1 {
		t.Fatalf("expected claim success with one drop, got result=%d drops=%d", claimResponse.GetResult(), len(claimResponse.GetDropList()))
	}
}

func TestIslandUnlockAndFinishTech(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandSnapshot{})
	clearTable(t, &orm.IslandTechnologyState{})

	seedConfigEntry(t, islandTechCategory, "1", `{"id":1,"formula_id":101,"island_level":1,"sys_unlock":[],"tech_repeat":2}`)
	seedConfigEntry(t, islandFormulaCategory, "101", `{"id":101,"unlock_type":7,"drop_list":[[2,20001,2]]}`)
	if err := orm.UpsertIslandSnapshot(&orm.IslandSnapshot{CommanderID: client.Commander.CommanderID, Level: 2, StorageLevel: 1}); err != nil {
		t.Fatalf("seed island snapshot: %v", err)
	}

	unlockPayload := &protobuf.CS_21520{TechId: proto.Uint32(1)}
	buffer, _ := proto.Marshal(unlockPayload)
	client.Buffer.Reset()
	if _, _, err := IslandUnlockTech(&buffer, client); err != nil {
		t.Fatalf("unlock handler failed: %v", err)
	}
	var unlockResponse protobuf.SC_21521
	decodePacketAt(t, client, 0, 21521, &unlockResponse)
	if unlockResponse.GetResult() != 0 {
		t.Fatalf("expected unlock success")
	}

	finishPayload := &protobuf.CS_21522{TechId: proto.Uint32(1)}
	buffer, _ = proto.Marshal(finishPayload)
	client.Buffer.Reset()
	if _, _, err := IslandFinishTechImmediate(&buffer, client); err != nil {
		t.Fatalf("finish handler failed: %v", err)
	}
	var finishResponse protobuf.SC_21523
	decodePacketAt(t, client, 0, 21523, &finishResponse)
	if finishResponse.GetResult() != 0 {
		t.Fatalf("expected immediate finish success")
	}
}

func TestIslandUnlockTechUsesDefaultLevelWhenSnapshotMissing(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandSnapshot{})
	clearTable(t, &orm.IslandTechnologyState{})

	seedConfigEntry(t, islandTechCategory, "1", `{"id":1,"formula_id":101,"island_level":1,"sys_unlock":[],"tech_repeat":1}`)
	seedConfigEntry(t, islandTechCategory, "2", `{"id":2,"formula_id":102,"island_level":2,"sys_unlock":[],"tech_repeat":1}`)
	seedConfigEntry(t, islandFormulaCategory, "101", `{"id":101,"drop_list":[[2,20001,1]]}`)
	seedConfigEntry(t, islandFormulaCategory, "102", `{"id":102,"drop_list":[[2,20001,1]]}`)

	successPayload := &protobuf.CS_21520{TechId: proto.Uint32(1)}
	buffer, _ := proto.Marshal(successPayload)
	client.Buffer.Reset()
	if _, _, err := IslandUnlockTech(&buffer, client); err != nil {
		t.Fatalf("unlock level-1 tech failed: %v", err)
	}
	var successResponse protobuf.SC_21521
	decodePacketAt(t, client, 0, 21521, &successResponse)
	if successResponse.GetResult() != 0 {
		t.Fatalf("expected level-1 unlock success without snapshot")
	}

	failPayload := &protobuf.CS_21520{TechId: proto.Uint32(2)}
	buffer, _ = proto.Marshal(failPayload)
	client.Buffer.Reset()
	if _, _, err := IslandUnlockTech(&buffer, client); err != nil {
		t.Fatalf("unlock level-2 tech failed: %v", err)
	}
	var failResponse protobuf.SC_21521
	decodePacketAt(t, client, 0, 21521, &failResponse)
	if failResponse.GetResult() == 0 {
		t.Fatalf("expected level-2 unlock to fail without snapshot")
	}
}

func TestIslandFinishTechImmediateAppliesAllDropTypes(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandSnapshot{})
	clearTable(t, &orm.IslandTechnologyState{})

	seedConfigEntry(t, islandTechCategory, "5", `{"id":5,"formula_id":105,"island_level":1,"sys_unlock":[],"tech_repeat":1}`)
	seedConfigEntry(t, islandFormulaCategory, "105", `{"id":105,"drop_list":[[1,1,33],[2,20001,2]]}`)
	if err := orm.UpsertIslandSnapshot(&orm.IslandSnapshot{CommanderID: client.Commander.CommanderID, Level: 1, StorageLevel: 1}); err != nil {
		t.Fatalf("seed island snapshot: %v", err)
	}

	unlockPayload := &protobuf.CS_21520{TechId: proto.Uint32(5)}
	buffer, _ := proto.Marshal(unlockPayload)
	client.Buffer.Reset()
	if _, _, err := IslandUnlockTech(&buffer, client); err != nil {
		t.Fatalf("unlock tech failed: %v", err)
	}

	goldBefore := client.Commander.GetResourceCount(1)
	itemBefore := client.Commander.GetItemCount(20001)

	finishPayload := &protobuf.CS_21522{TechId: proto.Uint32(5)}
	buffer, _ = proto.Marshal(finishPayload)
	client.Buffer.Reset()
	if _, _, err := IslandFinishTechImmediate(&buffer, client); err != nil {
		t.Fatalf("finish tech failed: %v", err)
	}

	var response protobuf.SC_21523
	decodePacketAt(t, client, 0, 21523, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected finish success")
	}
	if len(response.GetDropList()) != 2 {
		t.Fatalf("expected two configured drops, got %d", len(response.GetDropList()))
	}
	if client.Commander.GetResourceCount(1) != goldBefore+33 {
		t.Fatalf("expected gold increase by 33, got %d", client.Commander.GetResourceCount(1)-goldBefore)
	}
	if client.Commander.GetItemCount(20001) != itemBefore+2 {
		t.Fatalf("expected item increase by 2, got %d", client.Commander.GetItemCount(20001)-itemBefore)
	}
}

func TestIslandShopRefreshAndDressRead(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandShopState{})
	clearTable(t, &orm.IslandCommanderDressState{})

	seedConfigEntry(t, islandShopTemplateCategory, "10", `{"id":10,"goods_id":[101,102]}`)
	seedConfigEntry(t, islandShopNormalCategory, "10", `{"id":10,"refresh_set":3,"refresh_player":[2,20001,1],"refresh_free":1,"refresh_time":120,"exist_time":3600}`)

	refreshPayload := &protobuf.CS_21020{ShopId: proto.Uint32(10)}
	buffer, _ := proto.Marshal(refreshPayload)
	client.Buffer.Reset()
	if _, _, err := IslandShopPlayerRefresh(&buffer, client); err != nil {
		t.Fatalf("shop refresh failed: %v", err)
	}
	var refreshResponse protobuf.SC_21021
	decodePacketAt(t, client, 0, 21021, &refreshResponse)
	if refreshResponse.GetResult() != 0 || refreshResponse.GetShopInfo() == nil {
		t.Fatalf("expected shop refresh success")
	}

	dressPayload := &protobuf.CS_21621{DressId: []uint32{5001, 5001, 5002}}
	buffer, _ = proto.Marshal(dressPayload)
	client.Buffer.Reset()
	if _, _, err := IslandSetCommanderDressRead(&buffer, client); err != nil {
		t.Fatalf("dress read failed: %v", err)
	}
	var dressResponse protobuf.SC_21622
	decodePacketAt(t, client, 0, 21622, &dressResponse)
	if dressResponse.GetResult() != 0 {
		t.Fatalf("expected dress read success")
	}

	states, err := orm.ListIslandCommanderDressStates(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("list dress states: %v", err)
	}
	if len(states) != 2 {
		t.Fatalf("expected deduplicated dress rows, got %d", len(states))
	}
}

func TestIslandGoFishingSuccess(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})

	seedConfigEntry(t, islandFishPointCategory, "300", `{"id":300}`)
	seedConfigEntry(t, islandFishCategory, "9001", `{"id":9001,"min_weight":10,"max_weight":10,"gold_state":1}`)

	payload := &protobuf.CS_21060{IslandId: proto.Uint32(client.Commander.CommanderID), PointId: proto.Uint32(300)}
	buffer, _ := proto.Marshal(payload)
	client.Buffer.Reset()
	if _, _, err := IslandGoFishing(&buffer, client); err != nil {
		t.Fatalf("go fishing failed: %v", err)
	}
	var response protobuf.SC_21061
	decodePacketAt(t, client, 0, 21061, &response)
	if response.GetResult() != 0 || response.GetFishId() == 0 || response.GetWeight() == 0 {
		t.Fatalf("expected fishing success payload, got %+v", response)
	}
}

func TestIslandFishingIntnConcurrent(t *testing.T) {
	islandFishingRNGMu.Lock()
	previousRNG := islandFishingRNG
	islandFishingRNG = rand.New(rand.NewSource(1))
	islandFishingRNGMu.Unlock()
	t.Cleanup(func() {
		islandFishingRNGMu.Lock()
		islandFishingRNG = previousRNG
		islandFishingRNGMu.Unlock()
	})

	var wg sync.WaitGroup
	failures := make(chan struct{}, 1)

	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				value := islandFishingIntn(7)
				if value >= 0 && value < 7 {
					continue
				}
				select {
				case failures <- struct{}{}:
				default:
				}
				return
			}
		}()
	}

	wg.Wait()
	if len(failures) > 0 {
		t.Fatalf("expected islandFishingIntn to stay within bounds under concurrency")
	}
}

func TestIslandGoFishingStoresRollForFinalize(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})

	seedConfigEntry(t, islandFishPointCategory, "300", `{"id":300}`)
	seedConfigEntry(t, islandFishCategory, "9001", `{"id":9001,"min_weight":10,"max_weight":10,"gold_state":1}`)

	islandFishingStateMu.Lock()
	islandFishingState = map[string]islandFishingRoll{}
	islandFishingStateMu.Unlock()
	previousNow := islandFishingNow
	now := time.Unix(1_700_000_000, 0).UTC()
	islandFishingNow = func() time.Time { return now }
	t.Cleanup(func() {
		islandFishingNow = previousNow
	})

	payload := &protobuf.CS_21060{IslandId: proto.Uint32(client.Commander.CommanderID), PointId: proto.Uint32(300)}
	buffer, _ := proto.Marshal(payload)
	client.Buffer.Reset()
	if _, _, err := IslandGoFishing(&buffer, client); err != nil {
		t.Fatalf("go fishing failed: %v", err)
	}

	key := islandFishingKey(client.Commander.CommanderID, client.Commander.CommanderID, 300)
	islandFishingStateMu.Lock()
	roll, ok := islandFishingState[key]
	islandFishingStateMu.Unlock()
	if !ok {
		t.Fatalf("expected fishing roll state to be cached")
	}
	if roll.ExpiresAt.Sub(now) != 5*time.Minute {
		t.Fatalf("expected 5 minute expiry window")
	}
}

func TestIslandFishingResultSuccessPersistsFishWeight(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandFishingState{})

	seedConfigEntry(t, islandFishPointCategory, "300", `{"id":300}`)
	seedConfigEntry(t, islandFishCategory, "9001", `{"id":9001,"min_weight":10,"max_weight":10,"gold_state":1}`)

	startPayload := &protobuf.CS_21060{IslandId: proto.Uint32(client.Commander.CommanderID), PointId: proto.Uint32(300)}
	startBuffer, _ := proto.Marshal(startPayload)
	client.Buffer.Reset()
	if _, _, err := IslandGoFishing(&startBuffer, client); err != nil {
		t.Fatalf("go fishing failed: %v", err)
	}

	finalizePayload := &protobuf.CS_21062{
		IslandId:  proto.Uint32(client.Commander.CommanderID),
		PointId:   proto.Uint32(300),
		EndResult: proto.Uint32(islandFishingEndResultSuccess),
	}
	finalizeBuffer, _ := proto.Marshal(finalizePayload)
	client.Buffer.Reset()
	if _, _, err := IslandFishingResult(&finalizeBuffer, client); err != nil {
		t.Fatalf("fishing finalize failed: %v", err)
	}
	var response protobuf.SC_21063
	decodePacketAt(t, client, 0, 21063, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected finalize success, got %d", response.GetResult())
	}

	state, err := orm.GetIslandFishingState(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load fishing state: %v", err)
	}
	if state.BaitID != 0 || state.FishRod != 0 {
		t.Fatalf("unexpected bait state mutation: %+v", state)
	}
	if len(state.FishWeights) != 1 || state.FishWeights[0].FishID != 9001 || state.FishWeights[0].MinWeight != 10 || state.FishWeights[0].MaxWeight != 10 {
		t.Fatalf("expected persisted fish weight, got %+v", state.FishWeights)
	}
}

func TestIslandFishingResultFailsWhenNoActiveRoll(t *testing.T) {
	client := setupHandlerCommander(t)

	islandFishingStateMu.Lock()
	islandFishingState = map[string]islandFishingRoll{}
	islandFishingStateMu.Unlock()

	payload := &protobuf.CS_21062{
		IslandId:  proto.Uint32(client.Commander.CommanderID),
		PointId:   proto.Uint32(300),
		EndResult: proto.Uint32(islandFishingEndResultSuccess),
	}
	buffer, _ := proto.Marshal(payload)
	client.Buffer.Reset()
	if _, _, err := IslandFishingResult(&buffer, client); err != nil {
		t.Fatalf("fishing finalize failed: %v", err)
	}
	var response protobuf.SC_21063
	decodePacketAt(t, client, 0, 21063, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected finalize failure without active roll")
	}
}

func TestIslandExchangeLureSuccess(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandInventory{})
	clearTable(t, &orm.IslandFishingState{})

	seedConfigEntry(t, islandFishingBaitCategory, "1001", `{"id":1001,"fish_rod":7,"cost":[[41,5001,2]]}`)
	execAnswerTestSQLT(t, "INSERT INTO island_inventories (commander_id, item_id, count) VALUES ($1, $2, $3)", int64(client.Commander.CommanderID), int64(5001), int64(5))

	payload := &protobuf.CS_21064{BaitId: proto.Uint32(1001)}
	buffer, _ := proto.Marshal(payload)
	client.Buffer.Reset()
	if _, _, err := IslandExchangeLure(&buffer, client); err != nil {
		t.Fatalf("exchange lure failed: %v", err)
	}

	var response protobuf.SC_21065
	decodePacketAt(t, client, 0, 21065, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected lure exchange success, got %d", response.GetResult())
	}

	remaining := queryAnswerTestInt64(t, "SELECT count FROM island_inventories WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(5001))
	if remaining != 3 {
		t.Fatalf("expected lure cost consumption, got %d", remaining)
	}

	state, err := orm.GetIslandFishingState(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load fishing state: %v", err)
	}
	if state.BaitID != 1001 || state.FishRod != 7 {
		t.Fatalf("expected persisted bait/rod state, got %+v", state)
	}
}

func TestIslandExchangeItemBatchSuccessAndRollback(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandInventory{})

	seedConfigEntry(t, islandExchangeCategory, "1", `{"id":1,"origin_item":[41,7001,2],"target_item":[41,8001],"target_num":3}`)
	seedConfigEntry(t, islandExchangeCategory, "2", `{"id":2,"origin_item":[2,20001,1],"target_item":[2,20002],"target_num":2}`)
	seedConfigEntry(t, islandExchangeCategory, "3", `{"id":3,"origin_item":[41,7001,7],"target_item":[41,8003],"target_num":1}`)
	execAnswerTestSQLT(t, "INSERT INTO island_inventories (commander_id, item_id, count) VALUES ($1, $2, $3)", int64(client.Commander.CommanderID), int64(7001), int64(10))
	seedHandlerCommanderItem(t, client, 20001, 5)

	payload := &protobuf.CS_21066{Makes: []*protobuf.PB_ISLAND_MAKE{{MakeId: proto.Uint32(1), Num: proto.Uint32(2)}, {MakeId: proto.Uint32(2), Num: proto.Uint32(1)}}}
	buffer, _ := proto.Marshal(payload)
	client.Buffer.Reset()
	if _, _, err := IslandExchangeItem(&buffer, client); err != nil {
		t.Fatalf("exchange item failed: %v", err)
	}

	var response protobuf.SC_21067
	decodePacketAt(t, client, 0, 21067, &response)
	if response.GetResult() != 0 || len(response.GetDropList()) != 2 {
		t.Fatalf("expected batch exchange success with merged drops, got result=%d drops=%d", response.GetResult(), len(response.GetDropList()))
	}

	remainingSource := queryAnswerTestInt64(t, "SELECT count FROM island_inventories WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(7001))
	if remainingSource != 6 {
		t.Fatalf("expected source island item consumption, got %d", remainingSource)
	}
	producedIslandItem := queryAnswerTestInt64(t, "SELECT count FROM island_inventories WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(8001))
	if producedIslandItem != 6 {
		t.Fatalf("expected produced island item count 6, got %d", producedIslandItem)
	}
	if client.Commander.GetItemCount(20001) != 4 || client.Commander.GetItemCount(20002) != 2 {
		t.Fatalf("expected commander item mutation for exchange template")
	}

	rollbackPayload := &protobuf.CS_21066{Makes: []*protobuf.PB_ISLAND_MAKE{{MakeId: proto.Uint32(1), Num: proto.Uint32(4)}}}
	rollbackBuffer, _ := proto.Marshal(rollbackPayload)
	client.Buffer.Reset()
	if _, _, err := IslandExchangeItem(&rollbackBuffer, client); err != nil {
		t.Fatalf("exchange rollback request failed: %v", err)
	}
	var rollbackResponse protobuf.SC_21067
	decodePacketAt(t, client, 0, 21067, &rollbackResponse)
	if rollbackResponse.GetResult() != islandFishingResultLack {
		t.Fatalf("expected insufficient result, got %d", rollbackResponse.GetResult())
	}
	postRollbackSource := queryAnswerTestInt64(t, "SELECT count FROM island_inventories WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(7001))
	if postRollbackSource != 6 {
		t.Fatalf("expected atomic rollback on insufficient source, got %d", postRollbackSource)
	}

	partialPayload := &protobuf.CS_21066{Makes: []*protobuf.PB_ISLAND_MAKE{{MakeId: proto.Uint32(1), Num: proto.Uint32(1)}, {MakeId: proto.Uint32(3), Num: proto.Uint32(1)}}}
	partialBuffer, _ := proto.Marshal(partialPayload)
	client.Buffer.Reset()
	if _, _, err := IslandExchangeItem(&partialBuffer, client); err != nil {
		t.Fatalf("exchange partial rollback request failed: %v", err)
	}
	var partialResponse protobuf.SC_21067
	decodePacketAt(t, client, 0, 21067, &partialResponse)
	if partialResponse.GetResult() != islandFishingResultLack {
		t.Fatalf("expected insufficient result for partial batch, got %d", partialResponse.GetResult())
	}
	postPartialSource := queryAnswerTestInt64(t, "SELECT count FROM island_inventories WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(7001))
	if postPartialSource != 6 {
		t.Fatalf("expected atomic rollback when later batch entry fails, got %d", postPartialSource)
	}

	overflowPayload := &protobuf.CS_21066{Makes: []*protobuf.PB_ISLAND_MAKE{{MakeId: proto.Uint32(1), Num: proto.Uint32(math.MaxUint32)}}}
	overflowBuffer, _ := proto.Marshal(overflowPayload)
	client.Buffer.Reset()
	if _, _, err := IslandExchangeItem(&overflowBuffer, client); err != nil {
		t.Fatalf("exchange overflow request failed: %v", err)
	}
	var overflowResponse protobuf.SC_21067
	decodePacketAt(t, client, 0, 21067, &overflowResponse)
	if overflowResponse.GetResult() != islandFishingResultInvalid {
		t.Fatalf("expected invalid result for overflowed batch size, got %d", overflowResponse.GetResult())
	}
	postOverflowSource := queryAnswerTestInt64(t, "SELECT count FROM island_inventories WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(7001))
	if postOverflowSource != 6 {
		t.Fatalf("expected no mutation for overflowed batch size, got %d", postOverflowSource)
	}
}
