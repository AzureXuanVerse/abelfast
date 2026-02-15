package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestUseAddShipExpItemSuccess(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.Ship{})

	seedShipExpBookConfig(t)
	seedShipLevelConfigForItemUse(t)
	seedShipForItemUse(t, client, 9001, 200001, 1, 50, 80, 0, 3)
	seedCommanderItem(t, client, 16501, 2)
	seedCommanderItem(t, client, 16502, 2)

	payload := protobuf.CS_22011{
		ShipId: proto.Uint32(9001),
		Books: []*protobuf.ITEM_INFO{
			{Id: proto.Uint32(16501), Num: proto.Uint32(1)},
			{Id: proto.Uint32(16502), Num: proto.Uint32(2)},
		},
	}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := UseAddShipExpItem(&buffer, client); err != nil {
		t.Fatalf("use add ship exp item failed: %v", err)
	}

	var response protobuf.SC_22012
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}

	level := queryAnswerTestInt64(t, "SELECT level FROM owned_ships WHERE owner_id = $1 AND id = $2", int64(client.Commander.CommanderID), int64(9001))
	if level != 2 {
		t.Fatalf("expected level 2, got %d", level)
	}
	exp := queryAnswerTestInt64(t, "SELECT exp FROM owned_ships WHERE owner_id = $1 AND id = $2", int64(client.Commander.CommanderID), int64(9001))
	if exp != 140 {
		t.Fatalf("expected exp 140, got %d", exp)
	}
	item16501 := queryAnswerTestInt64(t, "SELECT count FROM commander_items WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(16501))
	if item16501 != 1 {
		t.Fatalf("expected item 16501 count 1, got %d", item16501)
	}
	item16502 := queryAnswerTestInt64(t, "SELECT count FROM commander_items WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(16502))
	if item16502 != 0 {
		t.Fatalf("expected item 16502 count 0, got %d", item16502)
	}
}

func TestUseAddShipExpItemFailsWhenItemsInsufficient(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.Ship{})

	seedShipExpBookConfig(t)
	seedShipLevelConfigForItemUse(t)
	seedShipForItemUse(t, client, 9002, 200002, 1, 50, 10, 0, 3)
	seedCommanderItem(t, client, 16501, 1)

	payload := protobuf.CS_22011{
		ShipId: proto.Uint32(9002),
		Books: []*protobuf.ITEM_INFO{
			{Id: proto.Uint32(16501), Num: proto.Uint32(2)},
		},
	}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := UseAddShipExpItem(&buffer, client); err != nil {
		t.Fatalf("use add ship exp item failed: %v", err)
	}

	var response protobuf.SC_22012
	decodeResponse(t, client, &response)
	if response.GetResult() != 1 {
		t.Fatalf("expected failure result, got %d", response.GetResult())
	}

	level := queryAnswerTestInt64(t, "SELECT level FROM owned_ships WHERE owner_id = $1 AND id = $2", int64(client.Commander.CommanderID), int64(9002))
	if level != 1 {
		t.Fatalf("expected level unchanged, got %d", level)
	}
	exp := queryAnswerTestInt64(t, "SELECT exp FROM owned_ships WHERE owner_id = $1 AND id = $2", int64(client.Commander.CommanderID), int64(9002))
	if exp != 10 {
		t.Fatalf("expected exp unchanged, got %d", exp)
	}
}

func TestUseAddShipExpItemFailsForInvalidShip(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderItem{})

	seedShipExpBookConfig(t)
	seedCommanderItem(t, client, 16501, 1)

	payload := protobuf.CS_22011{
		ShipId: proto.Uint32(999999),
		Books:  []*protobuf.ITEM_INFO{{Id: proto.Uint32(16501), Num: proto.Uint32(1)}},
	}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := UseAddShipExpItem(&buffer, client); err != nil {
		t.Fatalf("use add ship exp item failed: %v", err)
	}

	var response protobuf.SC_22012
	decodeResponse(t, client, &response)
	if response.GetResult() != 1 {
		t.Fatalf("expected failure result, got %d", response.GetResult())
	}
}

func TestUseAddShipExpItemFailsForInvalidBook(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.Ship{})

	seedShipExpBookConfig(t)
	seedShipLevelConfigForItemUse(t)
	seedShipForItemUse(t, client, 9003, 200003, 1, 50, 0, 0, 3)
	seedCommanderItem(t, client, 77777, 1)
	seedConfigEntry(t, itemConfigCategoryPrimary, "77777", `{"id":77777,"usage_arg":"100","max_num":100}`)

	payload := protobuf.CS_22011{
		ShipId: proto.Uint32(9003),
		Books:  []*protobuf.ITEM_INFO{{Id: proto.Uint32(77777), Num: proto.Uint32(1)}},
	}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := UseAddShipExpItem(&buffer, client); err != nil {
		t.Fatalf("use add ship exp item failed: %v", err)
	}

	var response protobuf.SC_22012
	decodeResponse(t, client, &response)
	if response.GetResult() != 1 {
		t.Fatalf("expected failure result, got %d", response.GetResult())
	}
}

func TestUseAddShipExpItemAppliesSurplusAtMaxLevel(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	initCommanderMaps(client)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.Ship{})

	seedShipExpBookConfig(t)
	seedShipForItemUse(t, client, 9004, 200004, 100, 100, 10, 20, 3)
	seedCommanderItem(t, client, 16501, 1)

	payload := protobuf.CS_22011{
		ShipId: proto.Uint32(9004),
		Books:  []*protobuf.ITEM_INFO{{Id: proto.Uint32(16501), Num: proto.Uint32(1)}},
	}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := UseAddShipExpItem(&buffer, client); err != nil {
		t.Fatalf("use add ship exp item failed: %v", err)
	}

	var response protobuf.SC_22012
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}

	surplusExp := queryAnswerTestInt64(t, "SELECT surplus_exp FROM owned_ships WHERE owner_id = $1 AND id = $2", int64(client.Commander.CommanderID), int64(9004))
	if surplusExp != 80 {
		t.Fatalf("expected surplus exp 80, got %d", surplusExp)
	}
	exp := queryAnswerTestInt64(t, "SELECT exp FROM owned_ships WHERE owner_id = $1 AND id = $2", int64(client.Commander.CommanderID), int64(9004))
	if exp != 10 {
		t.Fatalf("expected exp to remain 10, got %d", exp)
	}
}

func seedShipExpBookConfig(t *testing.T) {
	t.Helper()
	seedConfigEntry(t, gameSetConfig, shipExpBooksGameSetKey, `{"key_value":0,"description":[16501,16502]}`)
	seedConfigEntry(t, itemConfigCategoryPrimary, "16501", `{"id":16501,"usage_arg":"60","max_num":3000}`)
	seedConfigEntry(t, itemConfigCategoryPrimary, "16502", `{"id":16502,"usage_arg":"50","max_num":3000}`)
}

func seedShipLevelConfigForItemUse(t *testing.T) {
	t.Helper()
	seedConfigEntry(t, "ShareCfg/ship_level.json", "1", `{"level":1,"exp":100,"exp_ur":120}`)
	seedConfigEntry(t, "ShareCfg/ship_level.json", "2", `{"level":2,"exp":200,"exp_ur":220}`)
	seedConfigEntry(t, "ShareCfg/ship_level.json", "3", `{"level":3,"exp":400,"exp_ur":440}`)
}

func seedShipForItemUse(t *testing.T, client *connection.Client, ownedShipID uint32, templateID uint32, level uint32, maxLevel uint32, exp uint32, surplusExp uint32, rarityID uint32) {
	t.Helper()
	execAnswerTestSQLT(t, "INSERT INTO ships (template_id, name, english_name, rarity_id, star, type, nationality, build_time) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", int64(templateID), "EXP Ship", "EXP Ship", int64(rarityID), int64(1), int64(1), int64(1), int64(1))
	execAnswerTestSQLT(t, "INSERT INTO owned_ships (owner_id, ship_id, id, level, max_level, exp, surplus_exp, energy, create_time, change_name_timestamp) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())", int64(client.Commander.CommanderID), int64(templateID), int64(ownedShipID), int64(level), int64(maxLevel), int64(exp), int64(surplusExp), int64(150))
	owned := orm.OwnedShip{OwnerID: client.Commander.CommanderID, ShipID: templateID, ID: ownedShipID, Level: level, MaxLevel: maxLevel, Exp: exp, SurplusExp: surplusExp, Energy: 150, Ship: orm.Ship{TemplateID: templateID, RarityID: rarityID}}
	client.Commander.Ships = append(client.Commander.Ships, owned)
	client.Commander.OwnedShipsMap[ownedShipID] = &client.Commander.Ships[len(client.Commander.Ships)-1]
}
