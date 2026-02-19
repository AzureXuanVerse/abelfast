package answer_test

import (
	"os"
	"testing"

	"github.com/ggmolly/belfast/internal/answer"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func setupDorm3dPacketClient(t *testing.T, commanderID uint32) *connection.Client {
	t.Helper()
	os.Setenv("MODE", "test")
	orm.InitDatabase()
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.Dorm3dApartment{})
	commander := createDorm3dCommander(t, commanderID)
	return &connection.Client{Commander: commander}
}

func seedDorm3dFavorConfig(t *testing.T, category string, key string, payload string) {
	t.Helper()
	if err := orm.UpsertConfigEntry(category, key, []byte(payload)); err != nil {
		t.Fatalf("failed to seed config %s/%s: %v", category, key, err)
	}
}

func TestDorm3dTriggerFavorSuccessPersists(t *testing.T) {
	client := setupDorm3dPacketClient(t, 9200)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_set.json", "daily_vigor_max", `{"key":"daily_vigor_max","key_value_int":3}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_set.json", "favor_level", `{"key":"favor_level","key_value_int":2}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_dorm_template.json", "7001", `{"id":7001}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_favor_trigger.json", "9001", `{"id":9001,"is_daily_max":1,"is_repeat":1,"num":40}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_favor.json", "7001_2", `{"id":1002,"char_id":7001,"level":2,"favor_exp":200,"levelup_item":[]}`)

	apartment := orm.NewDorm3dApartment(client.Commander.CommanderID)
	apartment.Ships = orm.Dorm3dShipList{{ShipGroup: 7001, FavorLv: 1, FavorExp: 10, DailyFavor: 2, RegularTrigger: []uint32{}}}
	if err := orm.CreateDorm3dApartment(&apartment); err != nil {
		t.Fatalf("failed to seed apartment: %v", err)
	}

	payload := &protobuf.CS_28003{ShipGroup: proto.Uint32(7001), TriggerId: proto.Uint32(9001)}
	buffer, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	if _, _, err := answer.Dorm3dTriggerFavor(&buffer, client); err != nil {
		t.Fatalf("Dorm3dTriggerFavor failed: %v", err)
	}

	response := &protobuf.SC_28004{}
	decodeTestPacket(t, client, 28004, response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}

	updated, err := orm.GetDorm3dApartment(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("failed to load apartment: %v", err)
	}
	if updated.DailyVigorMax != 1 {
		t.Fatalf("expected vigor usage 1, got %d", updated.DailyVigorMax)
	}
	ship := updated.Ships[0]
	if ship.FavorExp != 50 {
		t.Fatalf("expected favor exp 50, got %d", ship.FavorExp)
	}
	if ship.DailyFavor != 42 {
		t.Fatalf("expected daily favor 42, got %d", ship.DailyFavor)
	}
	if len(ship.RegularTrigger) != 1 || ship.RegularTrigger[0] != 9001 {
		t.Fatalf("expected regular trigger 9001, got %v", ship.RegularTrigger)
	}
}

func TestDorm3dTriggerFavorFailsWhenOneTimeAlreadyUsed(t *testing.T) {
	client := setupDorm3dPacketClient(t, 9201)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_set.json", "daily_vigor_max", `{"key":"daily_vigor_max","key_value_int":3}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_set.json", "favor_level", `{"key":"favor_level","key_value_int":2}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_favor_trigger.json", "9002", `{"id":9002,"is_daily_max":0,"is_repeat":0,"num":60}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_favor.json", "7002_2", `{"id":1002,"char_id":7002,"level":2,"favor_exp":200,"levelup_item":[]}`)

	apartment := orm.NewDorm3dApartment(client.Commander.CommanderID)
	apartment.Ships = orm.Dorm3dShipList{{ShipGroup: 7002, FavorLv: 1, FavorExp: 10, RegularTrigger: []uint32{9002}}}
	if err := orm.CreateDorm3dApartment(&apartment); err != nil {
		t.Fatalf("failed to seed apartment: %v", err)
	}

	payload := &protobuf.CS_28003{ShipGroup: proto.Uint32(7002), TriggerId: proto.Uint32(9002)}
	buffer, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	if _, _, err := answer.Dorm3dTriggerFavor(&buffer, client); err != nil {
		t.Fatalf("Dorm3dTriggerFavor failed: %v", err)
	}

	response := &protobuf.SC_28004{}
	decodeTestPacket(t, client, 28004, response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result")
	}

	updated, err := orm.GetDorm3dApartment(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("failed to load apartment: %v", err)
	}
	if updated.Ships[0].FavorExp != 10 {
		t.Fatalf("expected unchanged favor exp, got %d", updated.Ships[0].FavorExp)
	}
	if len(updated.Ships[0].RegularTrigger) != 1 {
		t.Fatalf("expected unchanged trigger list, got %v", updated.Ships[0].RegularTrigger)
	}
}

func TestDorm3dGiveGiftSuccessPersists(t *testing.T) {
	client := setupDorm3dPacketClient(t, 9202)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_set.json", "daily_vigor_max", `{"key":"daily_vigor_max","key_value_int":3}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_set.json", "favor_level", `{"key":"favor_level","key_value_int":5}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_dorm_template.json", "7003", `{"id":7003}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_favor_trigger.json", "9101", `{"id":9101,"is_daily_max":0,"is_repeat":1,"num":40}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_favor.json", "7003_5", `{"id":1005,"char_id":7003,"level":5,"favor_exp":9999,"levelup_item":[]}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_gift.json", "5001", `{"id":5001,"ship_group_id":0,"favor_trigger_id":9101}`)

	apartment := orm.NewDorm3dApartment(client.Commander.CommanderID)
	apartment.Gifts = orm.Dorm3dGiftList{{GiftID: 5001, Number: 3, UsedNumber: 0}}
	apartment.Ships = orm.Dorm3dShipList{{ShipGroup: 7003, FavorLv: 1, FavorExp: 20, RegularTrigger: []uint32{}}}
	if err := orm.CreateDorm3dApartment(&apartment); err != nil {
		t.Fatalf("failed to seed apartment: %v", err)
	}

	payload := &protobuf.CS_28009{
		ShipGroup: proto.Uint32(7003),
		Gifts:     []*protobuf.APARTMENT_GIVE_GIFT{{GiftId: proto.Uint32(5001), Number: proto.Uint32(2)}},
	}
	buffer, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	if _, _, err := answer.HandleDorm3dGiveGift(&buffer, client); err != nil {
		t.Fatalf("HandleDorm3dGiveGift failed: %v", err)
	}

	response := &protobuf.SC_28010{}
	decodeTestPacket(t, client, 28010, response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}

	updated, err := orm.GetDorm3dApartment(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("failed to load apartment: %v", err)
	}
	if updated.Gifts[0].Number != 1 || updated.Gifts[0].UsedNumber != 2 {
		t.Fatalf("expected updated gift counters, got %+v", updated.Gifts[0])
	}
	ship := updated.Ships[0]
	if ship.FavorExp != 100 {
		t.Fatalf("expected favor exp 100, got %d", ship.FavorExp)
	}
	if len(ship.RegularTrigger) != 2 || ship.RegularTrigger[0] != 9101 || ship.RegularTrigger[1] != 9101 {
		t.Fatalf("expected repeated trigger history, got %v", ship.RegularTrigger)
	}
}

func TestDorm3dGiveGiftFailsDedicatedGiftAlreadyUsed(t *testing.T) {
	client := setupDorm3dPacketClient(t, 9203)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_set.json", "daily_vigor_max", `{"key":"daily_vigor_max","key_value_int":3}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_set.json", "favor_level", `{"key":"favor_level","key_value_int":5}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_favor.json", "7004_5", `{"id":1005,"char_id":7004,"level":5,"favor_exp":9999,"levelup_item":[]}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_gift.json", "5002", `{"id":5002,"ship_group_id":7004,"favor_trigger_id":0}`)

	apartment := orm.NewDorm3dApartment(client.Commander.CommanderID)
	apartment.Gifts = orm.Dorm3dGiftList{{GiftID: 5002, Number: 1, UsedNumber: 1}}
	apartment.Ships = orm.Dorm3dShipList{{ShipGroup: 7004, FavorLv: 1, FavorExp: 20, RegularTrigger: []uint32{}}}
	if err := orm.CreateDorm3dApartment(&apartment); err != nil {
		t.Fatalf("failed to seed apartment: %v", err)
	}

	payload := &protobuf.CS_28009{
		ShipGroup: proto.Uint32(7004),
		Gifts:     []*protobuf.APARTMENT_GIVE_GIFT{{GiftId: proto.Uint32(5002), Number: proto.Uint32(1)}},
	}
	buffer, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	if _, _, err := answer.HandleDorm3dGiveGift(&buffer, client); err != nil {
		t.Fatalf("HandleDorm3dGiveGift failed: %v", err)
	}

	response := &protobuf.SC_28010{}
	decodeTestPacket(t, client, 28010, response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result")
	}

	updated, err := orm.GetDorm3dApartment(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("failed to load apartment: %v", err)
	}
	if updated.Gifts[0].Number != 1 || updated.Gifts[0].UsedNumber != 1 {
		t.Fatalf("expected unchanged gift counters, got %+v", updated.Gifts[0])
	}
}

func TestDorm3dApartmentLevelUpSuccessReturnsDrops(t *testing.T) {
	client := setupDorm3dPacketClient(t, 9204)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_set.json", "favor_level", `{"key":"favor_level","key_value_int":3}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_dorm_template.json", "7005", `{"id":7005}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_favor.json", "7005_2", `{"id":2002,"char_id":7005,"level":2,"favor_exp":40,"levelup_item":[[2,1001,3],[1,1,5]]}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_favor.json", "7005_3", `{"id":2003,"char_id":7005,"level":3,"favor_exp":200,"levelup_item":[]}`)

	apartment := orm.NewDorm3dApartment(client.Commander.CommanderID)
	apartment.Ships = orm.Dorm3dShipList{{ShipGroup: 7005, FavorLv: 1, FavorExp: 50}}
	if err := orm.CreateDorm3dApartment(&apartment); err != nil {
		t.Fatalf("failed to seed apartment: %v", err)
	}
	beforeGold := uint32(0)
	if res, ok := client.Commander.OwnedResourcesMap[1]; ok {
		beforeGold = res.Amount
	}
	beforeItem := uint32(0)
	if item, ok := client.Commander.CommanderItemsMap[1001]; ok {
		beforeItem = item.Count
	}

	payload := &protobuf.CS_28005{ShipGroup: proto.Uint32(7005)}
	buffer, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	if _, _, err := answer.Dorm3dApartmentLevelUp(&buffer, client); err != nil {
		t.Fatalf("Dorm3dApartmentLevelUp failed: %v", err)
	}

	response := &protobuf.SC_28006{}
	decodeTestPacket(t, client, 28006, response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if len(response.GetDropList()) != 2 {
		t.Fatalf("expected 2 drops, got %d", len(response.GetDropList()))
	}

	updated, err := orm.GetDorm3dApartment(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("failed to load apartment: %v", err)
	}
	ship := updated.Ships[0]
	if ship.FavorLv != 2 || ship.FavorExp != 10 {
		t.Fatalf("expected level 2 with exp 10, got level=%d exp=%d", ship.FavorLv, ship.FavorExp)
	}
	afterGold := uint32(0)
	if res, ok := client.Commander.OwnedResourcesMap[1]; ok {
		afterGold = res.Amount
	}
	if afterGold != beforeGold+5 {
		t.Fatalf("expected +5 resource reward, got %d", afterGold)
	}
	afterItem := uint32(0)
	if item, ok := client.Commander.CommanderItemsMap[1001]; ok {
		afterItem = item.Count
	}
	if afterItem != beforeItem+3 {
		t.Fatalf("expected +3 item reward, got %d", afterItem)
	}
}

func TestDorm3dGiveGiftFailsDedicatedGiftBulkRequest(t *testing.T) {
	client := setupDorm3dPacketClient(t, 9207)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_set.json", "daily_vigor_max", `{"key":"daily_vigor_max","key_value_int":3}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_set.json", "favor_level", `{"key":"favor_level","key_value_int":5}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_favor.json", "7007_5", `{"id":1005,"char_id":7007,"level":5,"favor_exp":9999,"levelup_item":[]}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_gift.json", "5007", `{"id":5007,"ship_group_id":7007,"favor_trigger_id":0}`)

	apartment := orm.NewDorm3dApartment(client.Commander.CommanderID)
	apartment.Gifts = orm.Dorm3dGiftList{{GiftID: 5007, Number: 2, UsedNumber: 0}}
	apartment.Ships = orm.Dorm3dShipList{{ShipGroup: 7007, FavorLv: 1, FavorExp: 20, RegularTrigger: []uint32{}}}
	if err := orm.CreateDorm3dApartment(&apartment); err != nil {
		t.Fatalf("failed to seed apartment: %v", err)
	}

	payload := &protobuf.CS_28009{
		ShipGroup: proto.Uint32(7007),
		Gifts:     []*protobuf.APARTMENT_GIVE_GIFT{{GiftId: proto.Uint32(5007), Number: proto.Uint32(2)}},
	}
	buffer, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	if _, _, err := answer.HandleDorm3dGiveGift(&buffer, client); err != nil {
		t.Fatalf("HandleDorm3dGiveGift failed: %v", err)
	}

	response := &protobuf.SC_28010{}
	decodeTestPacket(t, client, 28010, response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result for dedicated bulk gift")
	}

	updated, err := orm.GetDorm3dApartment(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("failed to load apartment: %v", err)
	}
	if updated.Gifts[0].Number != 2 || updated.Gifts[0].UsedNumber != 0 {
		t.Fatalf("expected unchanged gift counters, got %+v", updated.Gifts[0])
	}
}

func TestDorm3dApartmentLevelUpFailsWhenInsufficientFavor(t *testing.T) {
	client := setupDorm3dPacketClient(t, 9205)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_set.json", "favor_level", `{"key":"favor_level","key_value_int":3}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_favor.json", "7006_2", `{"id":2002,"char_id":7006,"level":2,"favor_exp":40,"levelup_item":[]}`)
	seedDorm3dFavorConfig(t, "ShareCfg/dorm3d_favor.json", "7006_3", `{"id":2003,"char_id":7006,"level":3,"favor_exp":200,"levelup_item":[]}`)

	apartment := orm.NewDorm3dApartment(client.Commander.CommanderID)
	apartment.Ships = orm.Dorm3dShipList{{ShipGroup: 7006, FavorLv: 1, FavorExp: 20}}
	if err := orm.CreateDorm3dApartment(&apartment); err != nil {
		t.Fatalf("failed to seed apartment: %v", err)
	}

	payload := &protobuf.CS_28005{ShipGroup: proto.Uint32(7006)}
	buffer, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	if _, _, err := answer.Dorm3dApartmentLevelUp(&buffer, client); err != nil {
		t.Fatalf("Dorm3dApartmentLevelUp failed: %v", err)
	}

	response := &protobuf.SC_28006{}
	decodeTestPacket(t, client, 28006, response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result")
	}

	updated, err := orm.GetDorm3dApartment(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("failed to load apartment: %v", err)
	}
	ship := updated.Ships[0]
	if ship.FavorLv != 1 || ship.FavorExp != 20 {
		t.Fatalf("expected unchanged level and exp, got level=%d exp=%d", ship.FavorLv, ship.FavorExp)
	}
}
