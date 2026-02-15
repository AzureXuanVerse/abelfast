package answer

import (
	"fmt"
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestStartLearnTacticsSuccess(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.CommanderSkillClass{})
	clearTable(t, &orm.CommanderShipSkill{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.Ship{})
	clearTable(t, &orm.ConfigEntry{})

	seedTacticsShip(t, client, 901, 200001)
	seedCommanderItem(t, client, 16001, 2)
	seedTacticsConfig(t, 200001, 501, 1, 3)

	payload := protobuf.CS_22201{RoomId: proto.Uint32(1), ShipId: proto.Uint32(901), SkillPos: proto.Uint32(1), ItemId: proto.Uint32(16001)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := StartLearnTactics(&buffer, client); err != nil {
		t.Fatalf("start learn tactics failed: %v", err)
	}

	var response protobuf.SC_22202
	decodeResponse(t, client, &response)
	if response.GetResult() != lessonResultOK {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if response.GetClassInfo() == nil {
		t.Fatalf("expected class info in response")
	}
	if response.GetClassInfo().GetShipId() != 901 || response.GetClassInfo().GetRoomId() != 1 || response.GetClassInfo().GetSkillPos() != 1 {
		t.Fatalf("unexpected class info payload")
	}
	if response.GetClassInfo().GetExp() != 150 {
		t.Fatalf("expected lesson exp 150, got %d", response.GetClassInfo().GetExp())
	}

	itemCount := queryAnswerTestInt64(t, "SELECT count FROM commander_items WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(16001))
	if itemCount != 1 {
		t.Fatalf("expected item count 1, got %d", itemCount)
	}

	classCount := queryAnswerTestInt64(t, "SELECT COUNT(*) FROM commander_skill_classes WHERE commander_id = $1 AND room_id = $2", int64(client.Commander.CommanderID), int64(1))
	if classCount != 1 {
		t.Fatalf("expected one class row, got %d", classCount)
	}
}

func TestStartLearnTacticsFailsWhenSkillAtMaxLevel(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.CommanderSkillClass{})
	clearTable(t, &orm.CommanderShipSkill{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.Ship{})
	clearTable(t, &orm.ConfigEntry{})

	seedTacticsShip(t, client, 902, 200002)
	seedCommanderItem(t, client, 16001, 1)
	seedTacticsConfig(t, 200002, 502, 2, 2)
	execAnswerTestSQLT(t, "INSERT INTO commander_ship_skills (commander_id, ship_id, skill_pos, skill_id, level, exp) VALUES ($1, $2, $3, $4, $5, $6)", int64(client.Commander.CommanderID), int64(902), int64(1), int64(502), int64(2), int64(0))

	payload := protobuf.CS_22201{RoomId: proto.Uint32(1), ShipId: proto.Uint32(902), SkillPos: proto.Uint32(1), ItemId: proto.Uint32(16001)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := StartLearnTactics(&buffer, client); err != nil {
		t.Fatalf("start learn tactics failed: %v", err)
	}

	var response protobuf.SC_22202
	decodeResponse(t, client, &response)
	if response.GetResult() != lessonResultFailed {
		t.Fatalf("expected failure result, got %d", response.GetResult())
	}

	itemCount := queryAnswerTestInt64(t, "SELECT count FROM commander_items WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(16001))
	if itemCount != 1 {
		t.Fatalf("expected unchanged item count, got %d", itemCount)
	}
}

func TestCancelLearnTacticsSuccess(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.CommanderSkillClass{})
	clearTable(t, &orm.CommanderShipSkill{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.Ship{})
	clearTable(t, &orm.ConfigEntry{})

	seedTacticsShip(t, client, 903, 200003)
	seedTacticsConfig(t, 200003, 503, 1, 3)
	now := uint32(time.Now().UTC().Unix())
	execAnswerTestSQLT(t, "INSERT INTO commander_skill_classes (commander_id, room_id, ship_id, skill_pos, skill_id, start_time, finish_time, exp) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", int64(client.Commander.CommanderID), int64(1), int64(903), int64(1), int64(503), int64(now-500), int64(now-1), int64(200))

	payload := protobuf.CS_22203{RoomId: proto.Uint32(1), Type: proto.Uint32(skillCancelTypeManual)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := CancelLearnTactics(&buffer, client); err != nil {
		t.Fatalf("cancel learn tactics failed: %v", err)
	}

	var response protobuf.SC_22204
	decodeResponse(t, client, &response)
	if response.GetResult() != lessonResultOK {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if response.GetExp() != 200 {
		t.Fatalf("expected granted exp 200, got %d", response.GetExp())
	}

	classCount := queryAnswerTestInt64(t, "SELECT COUNT(*) FROM commander_skill_classes WHERE commander_id = $1 AND room_id = $2", int64(client.Commander.CommanderID), int64(1))
	if classCount != 0 {
		t.Fatalf("expected class row deleted, got %d", classCount)
	}
	skillLevel := queryAnswerTestInt64(t, "SELECT level FROM commander_ship_skills WHERE commander_id = $1 AND ship_id = $2 AND skill_pos = $3", int64(client.Commander.CommanderID), int64(903), int64(1))
	if skillLevel != 3 {
		t.Fatalf("expected level 3, got %d", skillLevel)
	}
}

func TestCancelLearnTacticsFailsForUnknownRoom(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.CommanderSkillClass{})

	payload := protobuf.CS_22203{RoomId: proto.Uint32(999), Type: proto.Uint32(skillCancelTypeAuto)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := CancelLearnTactics(&buffer, client); err != nil {
		t.Fatalf("cancel learn tactics failed: %v", err)
	}

	var response protobuf.SC_22204
	decodeResponse(t, client, &response)
	if response.GetResult() != lessonResultFailed {
		t.Fatalf("expected failure result, got %d", response.GetResult())
	}
}

func TestResourcesInfoIncludesSkillClassList(t *testing.T) {
	client := setupConfigTest(t)
	clearTable(t, &orm.CommanderSkillClass{})
	clearTable(t, &orm.CommanderTacticsQuickFinish{})
	seedConfigEntry(t, "ShareCfg/oilfield_template.json", "1", `{"level":1,"time":0}`)
	seedConfigEntry(t, "ShareCfg/class_upgrade_template.json", "1", `{"level":1,"time":0}`)
	seedConfigEntry(t, "ShareCfg/navalacademy_data_template.json", "1", `{"id":1}`)
	execAnswerTestSQLT(t, "INSERT INTO ships (template_id, name, english_name, rarity_id, star, type, nationality, build_time) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", int64(300001), "Ship", "Ship", int64(2), int64(1), int64(1), int64(1), int64(1))
	execAnswerTestSQLT(t, "INSERT INTO owned_ships (owner_id, ship_id, id) VALUES ($1, $2, $3)", int64(client.Commander.CommanderID), int64(300001), int64(9901))
	execAnswerTestSQLT(t, "INSERT INTO commander_skill_classes (commander_id, room_id, ship_id, skill_pos, skill_id, start_time, finish_time, exp) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", int64(client.Commander.CommanderID), int64(1), int64(9901), int64(1), int64(777), int64(10), int64(20), int64(30))

	buffer := []byte{}
	if _, _, err := ResourcesInfo(&buffer, client); err != nil {
		t.Fatalf("resources info failed: %v", err)
	}

	var response protobuf.SC_22001
	decodeResponse(t, client, &response)
	if len(response.GetSkillClassList()) != 1 {
		t.Fatalf("expected one active class, got %d", len(response.GetSkillClassList()))
	}
	if response.GetSkillClassList()[0].GetShipId() != 9901 {
		t.Fatalf("unexpected class ship id %d", response.GetSkillClassList()[0].GetShipId())
	}
	if response.GetDailyFinishBuffCnt() != 0 {
		t.Fatalf("expected daily finish count 0, got %d", response.GetDailyFinishBuffCnt())
	}
}

func TestQuickFinishLearnTacticsSuccess(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.CommanderSkillClass{})
	clearTable(t, &orm.CommanderShipSkill{})
	clearTable(t, &orm.CommanderBuff{})
	clearTable(t, &orm.CommanderTacticsQuickFinish{})
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.Ship{})

	seedTacticsShip(t, client, 904, 200004)
	seedTacticsConfig(t, 200004, 504, 1, 3)
	seedQuickFinishBuffConfig(t, 7001, 2)
	seedCommanderBuff(t, client.Commander.CommanderID, 7001, time.Now().UTC().Add(time.Hour))

	now := uint32(time.Now().UTC().Unix())
	execAnswerTestSQLT(t, "INSERT INTO commander_skill_classes (commander_id, room_id, ship_id, skill_pos, skill_id, start_time, finish_time, exp) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", int64(client.Commander.CommanderID), int64(1), int64(904), int64(1), int64(504), int64(now-600), int64(now+600), int64(180))

	payload := protobuf.CS_22014{Roomid: proto.Uint32(1)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := QuickFinishLearnTactics(&buffer, client); err != nil {
		t.Fatalf("quick finish learn tactics failed: %v", err)
	}

	var response protobuf.SC_22015
	decodeResponse(t, client, &response)
	if response.GetResult() != lessonQuickFinishResultOK {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}

	classCount := queryAnswerTestInt64(t, "SELECT COUNT(*) FROM commander_skill_classes WHERE commander_id = $1 AND room_id = $2", int64(client.Commander.CommanderID), int64(1))
	if classCount != 0 {
		t.Fatalf("expected class row deleted, got %d", classCount)
	}
	skillLevel := queryAnswerTestInt64(t, "SELECT level FROM commander_ship_skills WHERE commander_id = $1 AND ship_id = $2 AND skill_pos = $3", int64(client.Commander.CommanderID), int64(904), int64(1))
	if skillLevel != 2 {
		t.Fatalf("expected skill level 2 after quick finish, got %d", skillLevel)
	}
	usedCount := queryAnswerTestInt64(t, "SELECT used_count FROM commander_tactics_quick_finishes WHERE commander_id = $1", int64(client.Commander.CommanderID))
	if usedCount != 1 {
		t.Fatalf("expected used count 1, got %d", usedCount)
	}

	resourcesBuffer := []byte{}
	if _, _, err := ResourcesInfo(&resourcesBuffer, client); err != nil {
		t.Fatalf("resources info failed: %v", err)
	}
	offset := 0
	var quickFinishResponse protobuf.SC_22015
	offset = decodePacketAt(t, client, offset, 22015, &quickFinishResponse)
	if quickFinishResponse.GetResult() != lessonQuickFinishResultOK {
		t.Fatalf("expected result 0, got %d", quickFinishResponse.GetResult())
	}
	var resources protobuf.SC_22001
	_ = decodePacketAt(t, client, offset, 22001, &resources)
	if resources.GetDailyFinishBuffCnt() != 1 {
		t.Fatalf("expected daily finish buff count 1, got %d", resources.GetDailyFinishBuffCnt())
	}
}

func TestQuickFinishLearnTacticsFailsWithoutSession(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.CommanderSkillClass{})
	clearTable(t, &orm.CommanderBuff{})
	clearTable(t, &orm.CommanderTacticsQuickFinish{})
	clearTable(t, &orm.ConfigEntry{})

	seedQuickFinishBuffConfig(t, 7002, 2)
	seedCommanderBuff(t, client.Commander.CommanderID, 7002, time.Now().UTC().Add(time.Hour))

	payload := protobuf.CS_22014{Roomid: proto.Uint32(99)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := QuickFinishLearnTactics(&buffer, client); err != nil {
		t.Fatalf("quick finish learn tactics failed: %v", err)
	}

	var response protobuf.SC_22015
	decodeResponse(t, client, &response)
	if response.GetResult() != lessonQuickFinishResultSessionNotFound {
		t.Fatalf("expected session not found result, got %d", response.GetResult())
	}
}

func TestQuickFinishLearnTacticsFailsForOtherCommanderSession(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.CommanderSkillClass{})
	clearTable(t, &orm.CommanderBuff{})
	clearTable(t, &orm.CommanderTacticsQuickFinish{})
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.Ship{})

	seedTacticsShip(t, client, 907, 200007)
	seedQuickFinishBuffConfig(t, 7005, 2)
	seedCommanderBuff(t, client.Commander.CommanderID, 7005, time.Now().UTC().Add(time.Hour))

	otherCommanderID := client.Commander.CommanderID + 100
	execAnswerTestSQLT(t, "INSERT INTO commanders (commander_id, account_id, name) VALUES ($1, $2, $3)", int64(otherCommanderID), int64(otherCommanderID), "Other")
	execAnswerTestSQLT(t, "INSERT INTO owned_ships (owner_id, ship_id, id) VALUES ($1, $2, $3)", int64(otherCommanderID), int64(200007), int64(1907))
	now := uint32(time.Now().UTC().Unix())
	execAnswerTestSQLT(t, "INSERT INTO commander_skill_classes (commander_id, room_id, ship_id, skill_pos, skill_id, start_time, finish_time, exp) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", int64(otherCommanderID), int64(1), int64(1907), int64(1), int64(507), int64(now-600), int64(now+600), int64(120))

	payload := protobuf.CS_22014{Roomid: proto.Uint32(1)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := QuickFinishLearnTactics(&buffer, client); err != nil {
		t.Fatalf("quick finish learn tactics failed: %v", err)
	}

	var response protobuf.SC_22015
	decodeResponse(t, client, &response)
	if response.GetResult() != lessonQuickFinishResultSessionNotFound {
		t.Fatalf("expected session not found result, got %d", response.GetResult())
	}

	otherCount := queryAnswerTestInt64(t, "SELECT COUNT(*) FROM commander_skill_classes WHERE commander_id = $1 AND room_id = $2", int64(otherCommanderID), int64(1))
	if otherCount != 1 {
		t.Fatalf("expected other commander class unchanged, got %d", otherCount)
	}
}

func TestQuickFinishLearnTacticsFailsWhenAllowanceExhausted(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.CommanderSkillClass{})
	clearTable(t, &orm.CommanderShipSkill{})
	clearTable(t, &orm.CommanderBuff{})
	clearTable(t, &orm.CommanderTacticsQuickFinish{})
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.Ship{})

	seedTacticsShip(t, client, 905, 200005)
	seedTacticsConfig(t, 200005, 505, 1, 3)
	seedQuickFinishBuffConfig(t, 7003, 1)
	seedCommanderBuff(t, client.Commander.CommanderID, 7003, time.Now().UTC().Add(time.Hour))

	now := uint32(time.Now().UTC().Unix())
	execAnswerTestSQLT(t, "INSERT INTO commander_skill_classes (commander_id, room_id, ship_id, skill_pos, skill_id, start_time, finish_time, exp) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", int64(client.Commander.CommanderID), int64(1), int64(905), int64(1), int64(505), int64(now-600), int64(now+600), int64(120))
	execAnswerTestSQLT(t, "INSERT INTO commander_tactics_quick_finishes (commander_id, used_count, reset_day) VALUES ($1, $2, $3)", int64(client.Commander.CommanderID), int64(1), int64(dayKeyFromTime(time.Now().UTC())))

	payload := protobuf.CS_22014{Roomid: proto.Uint32(1)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := QuickFinishLearnTactics(&buffer, client); err != nil {
		t.Fatalf("quick finish learn tactics failed: %v", err)
	}

	var response protobuf.SC_22015
	decodeResponse(t, client, &response)
	if response.GetResult() != lessonQuickFinishResultAllowanceExceeded {
		t.Fatalf("expected allowance exceeded result, got %d", response.GetResult())
	}

	classCount := queryAnswerTestInt64(t, "SELECT COUNT(*) FROM commander_skill_classes WHERE commander_id = $1 AND room_id = $2", int64(client.Commander.CommanderID), int64(1))
	if classCount != 1 {
		t.Fatalf("expected class row to remain, got %d", classCount)
	}
}

func TestQuickFinishLearnTacticsIdempotent(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.CommanderSkillClass{})
	clearTable(t, &orm.CommanderShipSkill{})
	clearTable(t, &orm.CommanderBuff{})
	clearTable(t, &orm.CommanderTacticsQuickFinish{})
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.Ship{})

	seedTacticsShip(t, client, 906, 200006)
	seedTacticsConfig(t, 200006, 506, 1, 3)
	seedQuickFinishBuffConfig(t, 7004, 2)
	seedCommanderBuff(t, client.Commander.CommanderID, 7004, time.Now().UTC().Add(time.Hour))

	now := uint32(time.Now().UTC().Unix())
	execAnswerTestSQLT(t, "INSERT INTO commander_skill_classes (commander_id, room_id, ship_id, skill_pos, skill_id, start_time, finish_time, exp) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", int64(client.Commander.CommanderID), int64(1), int64(906), int64(1), int64(506), int64(now-600), int64(now+600), int64(120))

	payload := protobuf.CS_22014{Roomid: proto.Uint32(1)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := QuickFinishLearnTactics(&buffer, client); err != nil {
		t.Fatalf("first quick finish failed: %v", err)
	}
	if _, _, err := QuickFinishLearnTactics(&buffer, client); err != nil {
		t.Fatalf("second quick finish failed: %v", err)
	}

	offset := 0
	var first protobuf.SC_22015
	offset = decodePacketAt(t, client, offset, 22015, &first)
	var second protobuf.SC_22015
	_ = decodePacketAt(t, client, offset, 22015, &second)

	if first.GetResult() != lessonQuickFinishResultOK {
		t.Fatalf("expected first result ok, got %d", first.GetResult())
	}
	if second.GetResult() != lessonQuickFinishResultSessionNotFound {
		t.Fatalf("expected second result session not found, got %d", second.GetResult())
	}

	usedCount := queryAnswerTestInt64(t, "SELECT used_count FROM commander_tactics_quick_finishes WHERE commander_id = $1", int64(client.Commander.CommanderID))
	if usedCount != 1 {
		t.Fatalf("expected one quick finish usage, got %d", usedCount)
	}
	skillLevel := queryAnswerTestInt64(t, "SELECT level FROM commander_ship_skills WHERE commander_id = $1 AND ship_id = $2 AND skill_pos = $3", int64(client.Commander.CommanderID), int64(906), int64(1))
	if skillLevel != 2 {
		t.Fatalf("expected skill level 2, got %d", skillLevel)
	}
}

func seedTacticsShip(t *testing.T, client *connection.Client, ownedShipID uint32, templateID uint32) {
	t.Helper()
	execAnswerTestSQLT(t, "INSERT INTO ships (template_id, name, english_name, rarity_id, star, type, nationality, build_time) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", int64(templateID), "Tactics Ship", "Tactics Ship", int64(3), int64(1), int64(1), int64(1), int64(1))
	execAnswerTestSQLT(t, "INSERT INTO owned_ships (owner_id, ship_id, id) VALUES ($1, $2, $3)", int64(client.Commander.CommanderID), int64(templateID), int64(ownedShipID))
	ship := orm.OwnedShip{OwnerID: client.Commander.CommanderID, ShipID: templateID, ID: ownedShipID}
	client.Commander.Ships = append(client.Commander.Ships, ship)
	client.Commander.OwnedShipsMap[ownedShipID] = &client.Commander.Ships[len(client.Commander.Ships)-1]
}

func seedTacticsConfig(t *testing.T, templateID uint32, skillID uint32, skillType uint32, maxLevel uint32) {
	t.Helper()
	seedConfigEntry(t, "ShareCfg/navalacademy_data_template.json", "1", `{"id":1}`)
	seedConfigEntry(t, shipTemplateCategory, fmt.Sprintf("%d", templateID), fmt.Sprintf(`{"id":%d,"buff_list_display":[%d]}`, templateID, skillID))
	seedConfigEntry(t, itemConfigCategory, "16001", `{"id":16001,"type":10,"usage":"usage_book","usage_arg":[3600,100,1,50]}`)
	seedConfigEntry(t, skillTemplateCategory, fmt.Sprintf("%d", skillID), fmt.Sprintf(`{"id":%d,"type":%d,"max_level":%d}`, skillID, skillType, maxLevel))
}

func seedQuickFinishBuffConfig(t *testing.T, buffID uint32, effect uint32) {
	t.Helper()
	seedConfigEntry(t, "ShareCfg/benefit_buff_template.json", fmt.Sprintf("%d", buffID), fmt.Sprintf(`{"id":%d,"benefit_type":"skill_learn_time","benefit_effect":"%d"}`, buffID, effect))
}

func seedCommanderBuff(t *testing.T, commanderID uint32, buffID uint32, expiresAt time.Time) {
	t.Helper()
	execAnswerTestSQLT(t, "INSERT INTO commander_buffs (commander_id, buff_id, expires_at) VALUES ($1, $2, $3)", int64(commanderID), int64(buffID), expiresAt)
}

func dayKeyFromTime(now time.Time) uint32 {
	year, month, day := now.UTC().Date()
	return uint32(year*10000 + int(month)*100 + day)
}
