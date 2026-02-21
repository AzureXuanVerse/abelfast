package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

func seedMetaShipForTests(t *testing.T, clientID uint32, ownedID uint32, templateID uint32, level uint32) {
	t.Helper()
	execAnswerTestSQLT(t, "INSERT INTO ships (template_id, name, english_name, rarity_id, star, type, nationality, build_time) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) ON CONFLICT (template_id) DO NOTHING", int64(templateID), "Meta Ship", "Meta Ship", int64(3), int64(1), int64(1), int64(1), int64(1))
	execAnswerTestSQLT(t, "INSERT INTO owned_ships (owner_id, ship_id, id, level, max_level, exp, energy, create_time, change_name_timestamp) VALUES ($1, $2, $3, $4, $5, 0, 150, NOW(), NOW())", int64(clientID), int64(templateID), int64(ownedID), int64(level), int64(70))
}

func TestMetaCharacterRepairSuccess(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.Ship{})
	execAnswerTestSQLT(t, "TRUNCATE TABLE owned_ship_meta_repairs RESTART IDENTITY CASCADE")

	seedMetaShipForTests(t, client.Commander.CommanderID, 7001, 9701011, 10)
	if err := client.Commander.Load(); err != nil {
		t.Fatalf("reload commander: %v", err)
	}
	seedCommanderItem(t, client, 21111, 10)

	seedConfigEntry(t, "ShareCfg/ship_strengthen_meta.json", "970101", `{"id":970101,"ship_id":9701011,"type":3,"repair_torpedo":[15201],"repair_total_exp":5000}`)
	seedConfigEntry(t, "ShareCfg/ship_meta_repair.json", "15201", `{"id":15201,"item_id":21111,"item_num":4,"repair_exp":100}`)

	payload := protobuf.CS_63301{ShipId: proto.Uint32(7001), RepairId: proto.Uint32(15201)}
	buffer, _ := proto.Marshal(&payload)
	if _, _, err := MetaCharacterRepair(&buffer, client); err != nil {
		t.Fatalf("meta repair failed: %v", err)
	}

	var response protobuf.SC_63302
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}

	remaining := queryAnswerTestInt64(t, "SELECT count FROM commander_items WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(21111))
	if remaining != 6 {
		t.Fatalf("expected item count 6, got %d", remaining)
	}
	ship := client.Commander.OwnedShipsMap[7001]
	protoShip := orm.ToProtoOwnedShip(*ship, nil, nil)
	if len(protoShip.GetMetaRepairList()) != 1 || protoShip.GetMetaRepairList()[0] != 15201 {
		t.Fatalf("expected persisted meta repair list, got %v", protoShip.GetMetaRepairList())
	}
}

func TestMetaCharActiveEnergySuccess(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.OwnedResource{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.Ship{})
	execAnswerTestSQLT(t, "TRUNCATE TABLE owned_ship_meta_repairs RESTART IDENTITY CASCADE")

	seedMetaShipForTests(t, client.Commander.CommanderID, 7002, 9701011, 30)
	if err := client.Commander.Load(); err != nil {
		t.Fatalf("reload commander: %v", err)
	}
	seedCommanderItem(t, client, 21015, 1)
	seedCommanderResource(t, client, 1, 1000)

	seedConfigEntry(t, "ShareCfg/ship_meta_breakout.json", "9701011", `{"id":9701011,"breakout_id":9701012,"gold":500,"item1":21015,"item1_num":1,"item2":0,"item2_num":0,"level":10,"repair":0}`)
	seedConfigEntry(t, "ShareCfg/ship_strengthen_meta.json", "970101", `{"id":970101,"ship_id":9701011,"type":3,"repair_total_exp":5000}`)
	execAnswerTestSQLT(t, "INSERT INTO ships (template_id, name, english_name, rarity_id, star, type, nationality, build_time) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", int64(9701012), "Meta Ship B", "Meta Ship B", int64(3), int64(1), int64(1), int64(1), int64(1))
	seedConfigEntry(t, "sharecfgdata/ship_data_template.json", "9701012", `{"id":9701012,"group_type":970101,"max_level":80,"buff_list_display":[]}`)
	if _, err := orm.GetShipMetaBreakoutConfig(9701011); err != nil {
		t.Fatalf("missing breakout config: %v", err)
	}
	if _, err := orm.GetShipStrengthenMetaConfig(970101); err != nil {
		t.Fatalf("missing strengthen config: %v", err)
	}
	if !client.Commander.HasEnoughGold(500) || !client.Commander.HasEnoughItem(21015, 1) {
		t.Fatalf("expected sufficient resources/items before activation")
	}

	payload := protobuf.CS_63303{ShipId: proto.Uint32(7002)}
	buffer, _ := proto.Marshal(&payload)
	if _, _, err := MetaCharActiveEnergy(&buffer, client); err != nil {
		t.Fatalf("meta active energy failed: %v", err)
	}
	var response protobuf.SC_63304
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	newTemplate := queryAnswerTestInt64(t, "SELECT ship_id FROM owned_ships WHERE owner_id = $1 AND id = $2", int64(client.Commander.CommanderID), int64(7002))
	if newTemplate != 9701012 {
		t.Fatalf("expected breakout template 9701012, got %d", newTemplate)
	}
}

func TestMetaCharacterUnlockShipIdempotent(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.Ship{})

	execAnswerTestSQLT(t, "INSERT INTO ships (template_id, name, english_name, rarity_id, star, type, nationality, build_time) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", int64(9702011), "Unlock Ship", "Unlock Ship", int64(3), int64(1), int64(1), int64(1), int64(1))
	seedConfigEntry(t, "ShareCfg/ship_strengthen_meta.json", "970201", `{"id":970201,"ship_id":9702011,"type":1}`)
	seedConfigEntry(t, "sharecfgdata/ship_data_template.json", "9702011", `{"id":9702011,"group_type":970201,"max_level":70,"buff_list_display":[]}`)

	payload := protobuf.CS_63305{MetaId: proto.Uint32(970201)}
	buffer, _ := proto.Marshal(&payload)
	if _, _, err := MetaCharacterUnlockShip(&buffer, client); err != nil {
		t.Fatalf("first unlock failed: %v", err)
	}
	var first protobuf.SC_63306
	decodeResponse(t, client, &first)
	if first.GetResult() != 0 || first.GetShip() == nil {
		t.Fatalf("expected successful unlock with ship")
	}
	firstShipID := first.GetShip().GetId()

	client.Buffer.Reset()
	if _, _, err := MetaCharacterUnlockShip(&buffer, client); err != nil {
		t.Fatalf("second unlock failed: %v", err)
	}
	var second protobuf.SC_63306
	decodeResponse(t, client, &second)
	if second.GetResult() != 0 || second.GetShip() == nil {
		t.Fatalf("expected idempotent success with ship")
	}
	if second.GetShip().GetId() != firstShipID {
		t.Fatalf("expected same owned ship id, got %d and %d", firstShipID, second.GetShip().GetId())
	}
	count := queryAnswerTestInt64(t, "SELECT COUNT(*) FROM owned_ships WHERE owner_id = $1 AND ship_id = $2", int64(client.Commander.CommanderID), int64(9702011))
	if count != 1 {
		t.Fatalf("expected single owned meta ship, got %d", count)
	}
}

func TestMetaCharacterRepairLegacySuccess(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.Ship{})
	execAnswerTestSQLT(t, "TRUNCATE TABLE owned_ship_meta_repairs RESTART IDENTITY CASCADE")

	seedMetaShipForTests(t, client.Commander.CommanderID, 7401, 9701011, 10)
	if err := client.Commander.Load(); err != nil {
		t.Fatalf("reload commander: %v", err)
	}
	seedCommanderItem(t, client, 21111, 10)

	seedConfigEntry(t, "ShareCfg/ship_strengthen_meta.json", "970101", `{"id":970101,"ship_id":9701011,"type":3,"repair_torpedo":[15201],"repair_total_exp":5000}`)
	seedConfigEntry(t, "ShareCfg/ship_meta_repair.json", "15201", `{"id":15201,"item_id":21111,"item_num":4,"repair_exp":100}`)

	buffer := protowire.AppendTag(nil, 1, protowire.VarintType)
	buffer = protowire.AppendVarint(buffer, uint64(7401))
	buffer = protowire.AppendTag(buffer, 2, protowire.VarintType)
	buffer = protowire.AppendVarint(buffer, uint64(15201))
	if _, _, err := MetaCharacterRepairLegacy(&buffer, client); err != nil {
		t.Fatalf("legacy meta repair failed: %v", err)
	}

	var response protobuf.SC_70002
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}

	remaining := queryAnswerTestInt64(t, "SELECT count FROM commander_items WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(21111))
	if remaining != 6 {
		t.Fatalf("expected item count 6, got %d", remaining)
	}
}

func TestMetaCharActiveEnergyLegacySuccess(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.OwnedResource{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.Ship{})
	execAnswerTestSQLT(t, "TRUNCATE TABLE owned_ship_meta_repairs RESTART IDENTITY CASCADE")

	seedMetaShipForTests(t, client.Commander.CommanderID, 7402, 9701011, 30)
	if err := client.Commander.Load(); err != nil {
		t.Fatalf("reload commander: %v", err)
	}
	seedCommanderItem(t, client, 21015, 1)
	seedCommanderResource(t, client, 1, 1000)

	seedConfigEntry(t, "ShareCfg/ship_meta_breakout.json", "9701011", `{"id":9701011,"breakout_id":9701012,"gold":500,"item1":21015,"item1_num":1,"item2":0,"item2_num":0,"level":10,"repair":0}`)
	seedConfigEntry(t, "ShareCfg/ship_strengthen_meta.json", "970101", `{"id":970101,"ship_id":9701011,"type":3,"repair_total_exp":5000}`)
	execAnswerTestSQLT(t, "INSERT INTO ships (template_id, name, english_name, rarity_id, star, type, nationality, build_time) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", int64(9701012), "Meta Ship B", "Meta Ship B", int64(3), int64(1), int64(1), int64(1), int64(1))
	seedConfigEntry(t, "sharecfgdata/ship_data_template.json", "9701012", `{"id":9701012,"group_type":970101,"max_level":80,"buff_list_display":[]}`)

	payload := protobuf.CS_70003{Id: proto.Uint32(7402)}
	buffer, _ := proto.Marshal(&payload)
	if _, _, err := MetaCharActiveEnergyLegacy(&buffer, client); err != nil {
		t.Fatalf("legacy meta active energy failed: %v", err)
	}

	var response protobuf.SC_70004
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
}

func TestMetaCharacterUnlockShipLegacyIdempotent(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.Ship{})

	execAnswerTestSQLT(t, "INSERT INTO ships (template_id, name, english_name, rarity_id, star, type, nationality, build_time) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", int64(9702011), "Unlock Ship", "Unlock Ship", int64(3), int64(1), int64(1), int64(1), int64(1))
	seedConfigEntry(t, "ShareCfg/ship_strengthen_meta.json", "970201", `{"id":970201,"ship_id":9702011,"type":1}`)
	seedConfigEntry(t, "sharecfgdata/ship_data_template.json", "9702011", `{"id":9702011,"group_type":970201,"max_level":70,"buff_list_display":[]}`)

	payload := protobuf.CS_70005{Id: proto.Uint32(970201)}
	buffer, _ := proto.Marshal(&payload)
	if _, _, err := MetaCharacterUnlockShipLegacy(&buffer, client); err != nil {
		t.Fatalf("first legacy unlock failed: %v", err)
	}
	var first protobuf.SC_70006
	decodeResponse(t, client, &first)
	if first.GetResult() != 0 || first.GetShip() == nil {
		t.Fatalf("expected successful unlock with ship")
	}
	firstShipID := first.GetShip().GetId()

	client.Buffer.Reset()
	if _, _, err := MetaCharacterUnlockShipLegacy(&buffer, client); err != nil {
		t.Fatalf("second legacy unlock failed: %v", err)
	}
	var second protobuf.SC_70006
	decodeResponse(t, client, &second)
	if second.GetResult() != 0 || second.GetShip() == nil {
		t.Fatalf("expected idempotent success with ship")
	}
	if second.GetShip().GetId() != firstShipID {
		t.Fatalf("expected same owned ship id, got %d and %d", firstShipID, second.GetShip().GetId())
	}
}

func TestMetaTacticsUnlockAndLevelUpFlow(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.Ship{})
	execAnswerTestSQLT(t, "TRUNCATE TABLE commander_meta_tactics_states RESTART IDENTITY CASCADE")
	execAnswerTestSQLT(t, "TRUNCATE TABLE commander_meta_tactics_skill_states RESTART IDENTITY CASCADE")
	execAnswerTestSQLT(t, "TRUNCATE TABLE commander_meta_tactics_task_progress RESTART IDENTITY CASCADE")

	seedMetaShipForTests(t, client.Commander.CommanderID, 7101, 9701011, 20)
	if err := client.Commander.Load(); err != nil {
		t.Fatalf("reload commander: %v", err)
	}
	seedCommanderItem(t, client, 16003, 5)
	seedCommanderItem(t, client, 16031, 2)

	seedConfigEntry(t, "sharecfgdata/ship_data_template.json", "9701011", `{"id":9701011,"group_type":970101,"max_level":70,"buff_list_display":[800040]}`)
	seedConfigEntry(t, "ShareCfg/ship_meta_skilltask.json", "1", `{"id":1,"level":1,"need_exp":100,"skill_ID":800040,"skill_levelup_task":[],"skill_unlock":[[2,16003,5]]}`)
	seedConfigEntry(t, "ShareCfg/ship_meta_skilltask.json", "2", `{"id":2,"level":2,"need_exp":200,"skill_ID":800040,"skill_levelup_task":[],"skill_unlock":[]}`)
	seedConfigEntry(t, "sharecfgdata/skill_data_template.json", "800040", `{"id":800040,"max_level":10}`)
	seedConfigEntry(t, "sharecfgdata/item_data_statistics.json", "16031", `{"id":16031,"type":25,"usage_arg":"100"}`)

	unlockPayload := protobuf.CS_63311{ShipId: proto.Uint32(7101), SkillId: proto.Uint32(800040), Index: proto.Uint32(2)}
	unlockBuffer, _ := proto.Marshal(&unlockPayload)
	if _, _, err := MetaCharacterTacticsUnlockCommandResponse(&unlockBuffer, client); err != nil {
		t.Fatalf("unlock failed: %v", err)
	}
	var unlockResponse protobuf.SC_63312
	decodeResponse(t, client, &unlockResponse)
	if unlockResponse.GetResult() != 0 {
		t.Fatalf("expected unlock success")
	}

	client.Buffer.Reset()
	booksPayload := protobuf.CS_63319{ShipId: proto.Uint32(7101), SkillId: proto.Uint32(800040), Books: []*protobuf.ITEM_INFO{{Id: proto.Uint32(16031), Num: proto.Uint32(1)}}}
	booksBuffer, _ := proto.Marshal(&booksPayload)
	if _, _, err := MetaQuickTacticsUseBooks(&booksBuffer, client); err != nil {
		t.Fatalf("quick tactics failed: %v", err)
	}
	var quickResponse protobuf.SC_63320
	decodeResponse(t, client, &quickResponse)
	if quickResponse.GetRet() != 0 {
		t.Fatalf("expected quick tactics success")
	}

	execAnswerTestSQLT(t, "UPDATE commander_meta_tactics_skill_states SET level = 1, exp = 100 WHERE commander_id = $1 AND ship_id = $2 AND skill_id = $3", int64(client.Commander.CommanderID), int64(7101), int64(800040))
	levelPayload := protobuf.CS_63309{ShipId: proto.Uint32(7101), SkillId: proto.Uint32(800040)}
	client.Buffer.Reset()
	levelBuffer, _ := proto.Marshal(&levelPayload)
	if _, _, err := MetaCharacterTacticsLevelUpCommandResponse(&levelBuffer, client); err != nil {
		t.Fatalf("level up failed: %v", err)
	}
	var levelResponse protobuf.SC_63310
	decodeResponse(t, client, &levelResponse)
	if levelResponse.GetResult() != 0 {
		t.Fatalf("expected level up success")
	}

	currentLevel := queryAnswerTestInt64(t, "SELECT level FROM commander_meta_tactics_skill_states WHERE commander_id = $1 AND ship_id = $2 AND skill_id = $3", int64(client.Commander.CommanderID), int64(7101), int64(800040))
	if currentLevel != 2 {
		t.Fatalf("expected level 2, got %d", currentLevel)
	}

	client.Buffer.Reset()
	requestPayload := protobuf.CS_63313{ShipId: proto.Uint32(7101)}
	requestBuffer, _ := proto.Marshal(&requestPayload)
	if _, _, err := MetaCharacterTacticsRequestCommandResponse(&requestBuffer, client); err != nil {
		t.Fatalf("request failed: %v", err)
	}
	var detail protobuf.SC_63314
	decodeResponse(t, client, &detail)
	if detail.GetShipId() != 7101 || detail.GetSkillId() != 800040 {
		t.Fatalf("expected detailed tactics payload for unlocked skill")
	}
}

func TestMetaQuickTacticsUseBooksRejectsMaxLevel(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.Ship{})
	execAnswerTestSQLT(t, "TRUNCATE TABLE commander_meta_tactics_skill_states RESTART IDENTITY CASCADE")

	seedMetaShipForTests(t, client.Commander.CommanderID, 7201, 9703011, 20)
	if err := client.Commander.Load(); err != nil {
		t.Fatalf("reload commander: %v", err)
	}
	seedCommanderItem(t, client, 16031, 1)
	seedConfigEntry(t, "sharecfgdata/ship_data_template.json", "9703011", `{"id":9703011,"group_type":970301,"max_level":70,"buff_list_display":[800041]}`)
	seedConfigEntry(t, "ShareCfg/ship_meta_skilltask.json", "11", `{"id":11,"level":1,"need_exp":100,"skill_ID":800041,"skill_levelup_task":[],"skill_unlock":[]}`)
	seedConfigEntry(t, "sharecfgdata/skill_data_template.json", "800041", `{"id":800041,"max_level":2}`)
	seedConfigEntry(t, "sharecfgdata/item_data_statistics.json", "16031", `{"id":16031,"type":25,"usage_arg":"100"}`)
	execAnswerTestSQLT(t, `
INSERT INTO commander_meta_tactics_skill_states (commander_id, ship_id, skill_id, skill_pos, level, exp)
VALUES ($1, $2, $3, 1, 2, 0)
ON CONFLICT (commander_id, ship_id, skill_id)
DO UPDATE SET level = 2, exp = 0
`, int64(client.Commander.CommanderID), int64(7201), int64(800041))

	payload := protobuf.CS_63319{ShipId: proto.Uint32(7201), SkillId: proto.Uint32(800041), Books: []*protobuf.ITEM_INFO{{Id: proto.Uint32(16031), Num: proto.Uint32(1)}}}
	buffer, _ := proto.Marshal(&payload)
	if _, _, err := MetaQuickTacticsUseBooks(&buffer, client); err != nil {
		t.Fatalf("quick tactics failed: %v", err)
	}
	var response protobuf.SC_63320
	decodeResponse(t, client, &response)
	if response.GetRet() == 0 {
		t.Fatalf("expected quick tactics to fail for max-level skill")
	}
	remaining := queryAnswerTestInt64(t, "SELECT count FROM commander_items WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(16031))
	if remaining != 1 {
		t.Fatalf("expected books to remain unchanged, got %d", remaining)
	}
}

func TestMetaTacticsRequestPreservesZeroSwitchCount(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.Ship{})
	execAnswerTestSQLT(t, "TRUNCATE TABLE commander_meta_tactics_states RESTART IDENTITY CASCADE")

	seedMetaShipForTests(t, client.Commander.CommanderID, 7301, 9704011, 20)
	if err := client.Commander.Load(); err != nil {
		t.Fatalf("reload commander: %v", err)
	}
	seedConfigEntry(t, "sharecfgdata/ship_data_template.json", "9704011", `{"id":9704011,"group_type":970401,"max_level":70,"buff_list_display":[800042]}`)
	seedConfigEntry(t, "ShareCfg/ship_meta_skilltask.json", "21", `{"id":21,"level":1,"need_exp":100,"skill_ID":800042,"skill_levelup_task":[],"skill_unlock":[]}`)
	execAnswerTestSQLT(t, `
INSERT INTO commander_meta_tactics_states (commander_id, ship_id, current_skill_id, daily_exp, double_exp, switch_cnt)
VALUES ($1, $2, 800042, 0, 0, 0)
ON CONFLICT (commander_id, ship_id)
DO UPDATE SET current_skill_id = 800042, switch_cnt = 0
`, int64(client.Commander.CommanderID), int64(7301))

	payload := protobuf.CS_63313{ShipId: proto.Uint32(7301)}
	buffer, _ := proto.Marshal(&payload)
	if _, _, err := MetaCharacterTacticsRequestCommandResponse(&buffer, client); err != nil {
		t.Fatalf("tactics request failed: %v", err)
	}
	var response protobuf.SC_63314
	decodeResponse(t, client, &response)
	if response.GetSwitchCnt() != 0 {
		t.Fatalf("expected switch count 0, got %d", response.GetSwitchCnt())
	}
}
