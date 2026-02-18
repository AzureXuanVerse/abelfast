package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func resetIslandShipPacketTables(t *testing.T, commanderID uint32) {
	t.Helper()
	execAnswerTestSQLT(t, "DELETE FROM island_ship_invites WHERE commander_id = $1", int64(commanderID))
	execAnswerTestSQLT(t, "DELETE FROM island_role_dresses WHERE commander_id = $1", int64(commanderID))
	execAnswerTestSQLT(t, "DELETE FROM island_ship_dresses WHERE commander_id = $1", int64(commanderID))
	execAnswerTestSQLT(t, "DELETE FROM island_ship_skins WHERE commander_id = $1", int64(commanderID))
	execAnswerTestSQLT(t, "DELETE FROM island_commander_dress_profiles WHERE commander_id = $1", int64(commanderID))
	execAnswerTestSQLT(t, "DELETE FROM island_npc_feedback_states WHERE commander_id = $1", int64(commanderID))
	clearTable(t, &orm.IslandInventory{})
	clearTable(t, &orm.IslandShip{})
	clearTable(t, &orm.ConfigEntry{})
}

func TestIslandShipAttrLimitUnlockSuccess(t *testing.T) {
	client := setupHandlerCommander(t)
	resetIslandShipPacketTables(t, client.Commander.CommanderID)

	if err := orm.UpsertIslandShip(&orm.IslandShip{CommanderID: client.Commander.CommanderID, ShipID: 1001, Level: 1, BreakLv: 1, SkillLv: 1, ExtraAttrs: []orm.IslandShipAttr{}, Buffs: []orm.IslandShipBuff{}, CanFollow: true}); err != nil {
		t.Fatalf("seed ship: %v", err)
	}
	execAnswerTestSQLT(t, "INSERT INTO island_inventories (commander_id, item_id, count) VALUES ($1, $2, $3)", int64(client.Commander.CommanderID), int64(100000), int64(1))

	payload := &protobuf.CS_21603{ShipId: proto.Uint32(1001)}
	buffer, _ := proto.Marshal(payload)
	client.Buffer.Reset()
	if _, _, err := HandleIslandShipAttrLimitUnlock(&buffer, client); err != nil {
		t.Fatalf("unlock handler failed: %v", err)
	}
	var response protobuf.SC_21604
	decodePacketAt(t, client, 0, 21604, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success result")
	}
	ship, err := orm.GetIslandShip(client.Commander.CommanderID, 1001)
	if err != nil {
		t.Fatalf("load ship: %v", err)
	}
	if ship.UpLimitState != 1 {
		t.Fatalf("expected up_limit_state=1")
	}
}

func TestIslandShipAttrUpgradeAndExpBook(t *testing.T) {
	client := setupHandlerCommander(t)
	resetIslandShipPacketTables(t, client.Commander.CommanderID)

	if err := orm.UpsertIslandShip(&orm.IslandShip{CommanderID: client.Commander.CommanderID, ShipID: 1002, Level: 1, BreakLv: 1, SkillLv: 1, ExtraAttrs: []orm.IslandShipAttr{}, Buffs: []orm.IslandShipBuff{}, CanFollow: true}); err != nil {
		t.Fatalf("seed ship: %v", err)
	}
	seedConfigEntry(t, islandCharaTemplateCategory, "1002", `{"id":1002,"att_item":[[[9001,1]]],"extra_max":[[50,100]]}`)
	seedConfigEntry(t, islandItemTemplateCategory, "9001", `{"id":9001,"usage":"usage_attr","usage_arg":[5]}`)
	execAnswerTestSQLT(t, "INSERT INTO island_inventories (commander_id, item_id, count) VALUES ($1, $2, $3)", int64(client.Commander.CommanderID), int64(9001), int64(2))

	attrPayload := &protobuf.CS_21605{ShipId: proto.Uint32(1002), Type: proto.Uint32(1), ItemList: []*protobuf.PB_ISLAND_ITEM{{Id: proto.Uint32(9001), Num: proto.Uint32(2)}}}
	attrBuffer, _ := proto.Marshal(attrPayload)
	client.Buffer.Reset()
	if _, _, err := HandleIslandShipAttrUpgrade(&attrBuffer, client); err != nil {
		t.Fatalf("attr upgrade failed: %v", err)
	}
	var attrResponse protobuf.SC_21606
	decodePacketAt(t, client, 0, 21606, &attrResponse)
	if attrResponse.GetResult() != 0 {
		t.Fatalf("expected attr upgrade success")
	}

	seedConfigEntry(t, islandItemTemplateCategory, "9101", `{"id":9101,"usage":"usage_expbook","usage_arg":[120]}`)
	seedConfigEntry(t, islandCharaLevelCategory, "1", `{"id":1,"level_up_exp":100}`)
	seedConfigEntry(t, islandCharaLevelCategory, "2", `{"id":2,"level_up_exp":200}`)
	execAnswerTestSQLT(t, "INSERT INTO island_inventories (commander_id, item_id, count) VALUES ($1, $2, $3)", int64(client.Commander.CommanderID), int64(9101), int64(1))

	expPayload := &protobuf.CS_21607{ShipId: proto.Uint32(1002), ItemList: []*protobuf.PB_ISLAND_ITEM{{Id: proto.Uint32(9101), Num: proto.Uint32(1)}}}
	expBuffer, _ := proto.Marshal(expPayload)
	client.Buffer.Reset()
	if _, _, err := HandleIslandUseShipExpBook(&expBuffer, client); err != nil {
		t.Fatalf("exp book failed: %v", err)
	}
	var expResponse protobuf.SC_21608
	decodePacketAt(t, client, 0, 21608, &expResponse)
	if expResponse.GetResult() != 0 || expResponse.GetAddExp() == 0 {
		t.Fatalf("expected exp success with applied exp")
	}
}

func TestIslandInviteShipAndSkillUpgrade(t *testing.T) {
	client := setupHandlerCommander(t)
	resetIslandShipPacketTables(t, client.Commander.CommanderID)

	seedConfigEntry(t, islandCharaTemplateCategory, "1003", `{"id":1003,"power":12,"skill_id":701,"skill_unlock":1}`)
	seedConfigEntry(t, islandCharaSkillCategory, "701", `{"id":701,"material":[[[9201,1]],[[9201,2]]],"skill_effect":[[1],[2],[3]]}`)
	if err := orm.AddIslandShipInvite(client.Commander.CommanderID, 1003); err != nil {
		t.Fatalf("seed invite: %v", err)
	}

	invitePayload := &protobuf.CS_21609{ShipId: proto.Uint32(1003)}
	inviteBuffer, _ := proto.Marshal(invitePayload)
	client.Buffer.Reset()
	if _, _, err := HandleIslandInviteShip(&inviteBuffer, client); err != nil {
		t.Fatalf("invite failed: %v", err)
	}
	var inviteResponse protobuf.SC_21610
	decodePacketAt(t, client, 0, 21610, &inviteResponse)
	if inviteResponse.GetResult() != 0 || inviteResponse.GetShip().GetId() != 1003 {
		t.Fatalf("expected invite success with ship payload")
	}

	execAnswerTestSQLT(t, "INSERT INTO island_inventories (commander_id, item_id, count) VALUES ($1, $2, $3)", int64(client.Commander.CommanderID), int64(9201), int64(2))
	skillPayload := &protobuf.CS_21611{ShipId: proto.Uint32(1003)}
	skillBuffer, _ := proto.Marshal(skillPayload)
	client.Buffer.Reset()
	if _, _, err := HandleIslandShipSkillUpgrade(&skillBuffer, client); err != nil {
		t.Fatalf("skill upgrade failed: %v", err)
	}
	var skillResponse protobuf.SC_21612
	decodePacketAt(t, client, 0, 21612, &skillResponse)
	if skillResponse.GetResult() != 0 {
		t.Fatalf("expected skill upgrade success")
	}
}

func TestIslandGiveGiftAndRoleSkinColor(t *testing.T) {
	client := setupHandlerCommander(t)
	resetIslandShipPacketTables(t, client.Commander.CommanderID)

	if err := orm.UpsertIslandShip(&orm.IslandShip{CommanderID: client.Commander.CommanderID, ShipID: 1004, Level: 1, BreakLv: 1, SkillLv: 1, CurSkinID: 2004, ExtraAttrs: []orm.IslandShipAttr{}, Buffs: []orm.IslandShipBuff{}, CanFollow: true}); err != nil {
		t.Fatalf("seed ship: %v", err)
	}
	seedConfigEntry(t, islandCharaTemplateCategory, "1004", `{"id":1004,"favorite_gift":[9301]}`)
	seedConfigEntry(t, islandItemTemplateCategory, "9301", `{"id":9301,"usage":"usage_island_gift","usage_arg":[[5,[401]],[10,[402]]]}`)
	seedConfigEntry(t, islandBuffTemplateCategory, "401", `{"id":401,"duel_type":1,"duel_id":1}`)
	seedConfigEntry(t, islandBuffTemplateCategory, "402", `{"id":402,"duel_type":1,"duel_id":1}`)
	execAnswerTestSQLT(t, "INSERT INTO island_inventories (commander_id, item_id, count) VALUES ($1, $2, $3)", int64(client.Commander.CommanderID), int64(9301), int64(1))

	giftPayload := &protobuf.CS_21613{ShipId: proto.Uint32(1004), GiftId: proto.Uint32(9301)}
	giftBuffer, _ := proto.Marshal(giftPayload)
	client.Buffer.Reset()
	if _, _, err := HandleIslandGiveGift(&giftBuffer, client); err != nil {
		t.Fatalf("give gift failed: %v", err)
	}
	var giftResponse protobuf.SC_21614
	decodePacketAt(t, client, 0, 21614, &giftResponse)
	if giftResponse.GetResult() != 0 {
		t.Fatalf("expected gift success")
	}

	seedConfigEntry(t, islandSkinTemplateCategory, "2004", `{"id":2004,"ship_group":88}`)
	seedConfigEntry(t, islandSkinColorTemplateCategory, "9401", `{"id":9401,"skin_group":88,"cost":[[41,9402,1]]}`)
	execAnswerTestSQLT(t, "INSERT INTO island_ship_skins (commander_id, ship_id, skin_id, color_id, color_list) VALUES ($1, $2, $3, $4, $5::jsonb)", int64(client.Commander.CommanderID), int64(1004), int64(2004), int64(0), `[]`)
	execAnswerTestSQLT(t, "INSERT INTO island_inventories (commander_id, item_id, count) VALUES ($1, $2, $3)", int64(client.Commander.CommanderID), int64(9402), int64(1))

	colorPayload := &protobuf.CS_21619{ShipId: proto.Uint32(1004), ColorId: proto.Uint32(9401)}
	colorBuffer, _ := proto.Marshal(colorPayload)
	client.Buffer.Reset()
	if _, _, err := IslandBuyRoleSkinColor(&colorBuffer, client); err != nil {
		t.Fatalf("buy role skin color failed: %v", err)
	}
	var colorResponse protobuf.SC_21620
	decodePacketAt(t, client, 0, 21620, &colorResponse)
	if colorResponse.GetResult() != 0 {
		t.Fatalf("expected role skin color purchase success")
	}
}

func TestIslandDressReadCommanderDressAndBuyColor(t *testing.T) {
	client := setupHandlerCommander(t)
	resetIslandShipPacketTables(t, client.Commander.CommanderID)

	if err := orm.AddIslandRoleDressNum(client.Commander.CommanderID, 9501, 2); err != nil {
		t.Fatalf("seed role dress num: %v", err)
	}
	readPayload := &protobuf.CS_21624{DressId: []uint32{9501, 9501, 0}}
	readBuffer, _ := proto.Marshal(readPayload)
	client.Buffer.Reset()
	if _, _, err := IslandSetRoleDressRead(&readBuffer, client); err != nil {
		t.Fatalf("role dress read failed: %v", err)
	}
	var readResponse protobuf.SC_21625
	decodePacketAt(t, client, 0, 21625, &readResponse)
	if readResponse.GetResult() != 0 {
		t.Fatalf("expected role dress read success")
	}

	changePayload := &protobuf.CS_21626{IslandId: proto.Uint32(client.Commander.CommanderID), DressList: []*protobuf.PB_ISLAND_CUR_DRESS{{Type: proto.Uint32(1), Id: proto.Uint32(9501)}}, ColorList: []*protobuf.PB_DRESS_COLOR{{Id: proto.Uint32(9501), Color: proto.Uint32(3)}}}
	changeBuffer, _ := proto.Marshal(changePayload)
	client.Buffer.Reset()
	if _, _, err := IslandChangeCommanderDress(&changeBuffer, client); err != nil {
		t.Fatalf("change commander dress failed: %v", err)
	}
	var changeResponse protobuf.SC_21627
	decodePacketAt(t, client, 0, 21627, &changeResponse)
	if changeResponse.GetResult() != 0 {
		t.Fatalf("expected commander dress update with ignored unowned color")
	}

	if _, err := orm.GetIslandCommanderDressState(client.Commander.CommanderID, 9501); err == nil || !db.IsNotFound(err) {
		t.Fatalf("expected no commander dress color state for unowned color, err=%v", err)
	}

	seedConfigEntry(t, islandDressColorTemplateCategory, "9601", `{"id":9601,"belongto_dress":9501,"cost":[[41,9602,1]]}`)
	execAnswerTestSQLT(t, "INSERT INTO island_inventories (commander_id, item_id, count) VALUES ($1, $2, $3)", int64(client.Commander.CommanderID), int64(9602), int64(1))
	buyPayload := &protobuf.CS_21628{Id: proto.Uint32(0), ColorId: proto.Uint32(9601)}
	buyBuffer, _ := proto.Marshal(buyPayload)
	client.Buffer.Reset()
	if _, _, err := IslandBuyDressColor(&buyBuffer, client); err != nil {
		t.Fatalf("buy dress color failed: %v", err)
	}
	var buyResponse protobuf.SC_21629
	decodePacketAt(t, client, 0, 21629, &buyResponse)
	if buyResponse.GetResult() != 0 {
		t.Fatalf("expected dress color purchase success")
	}

	applyPayload := &protobuf.CS_21626{IslandId: proto.Uint32(client.Commander.CommanderID), DressList: []*protobuf.PB_ISLAND_CUR_DRESS{{Type: proto.Uint32(1), Id: proto.Uint32(9501)}}, ColorList: []*protobuf.PB_DRESS_COLOR{{Id: proto.Uint32(9501), Color: proto.Uint32(9601)}}}
	applyBuffer, _ := proto.Marshal(applyPayload)
	client.Buffer.Reset()
	if _, _, err := IslandChangeCommanderDress(&applyBuffer, client); err != nil {
		t.Fatalf("apply commander dress color failed: %v", err)
	}
	decodePacketAt(t, client, 0, 21627, &changeResponse)
	if changeResponse.GetResult() != 0 {
		t.Fatalf("expected commander dress save success after color unlock")
	}

	dressState, err := orm.GetIslandCommanderDressState(client.Commander.CommanderID, 9501)
	if err != nil {
		t.Fatalf("load commander dress state: %v", err)
	}
	if dressState.Color != 9601 || !containsUint32(dressState.ColorList, 9601) {
		t.Fatalf("expected equipped and preserved unlocked color, got %+v", dressState)
	}
}

func TestIslandChangeDressInvalidWearRollsBackUnload(t *testing.T) {
	client := setupHandlerCommander(t)
	resetIslandShipPacketTables(t, client.Commander.CommanderID)

	if err := orm.UpsertIslandShip(&orm.IslandShip{CommanderID: client.Commander.CommanderID, ShipID: 1005, Level: 1, BreakLv: 1, SkillLv: 1, ExtraAttrs: []orm.IslandShipAttr{}, Buffs: []orm.IslandShipBuff{}, CanFollow: true}); err != nil {
		t.Fatalf("seed ship: %v", err)
	}
	execAnswerTestSQLT(t, "INSERT INTO island_ship_dresses (commander_id, ship_id, dress_id) VALUES ($1, $2, $3)", int64(client.Commander.CommanderID), int64(1005), int64(9701))

	payload := &protobuf.CS_21617{
		ShipId:      proto.Uint32(1005),
		UnloadDress: []uint32{9701},
		Dress_List:  []*protobuf.PB_ISLAND_SHIP_WEAR{{ShipId: proto.Uint32(0), DressId: proto.Uint32(0)}},
		SkinId:      proto.Uint32(0),
		ColorId:     proto.Uint32(0),
	}
	buffer, _ := proto.Marshal(payload)
	client.Buffer.Reset()
	if _, _, err := HandleIslandChangeDress(&buffer, client); err != nil {
		t.Fatalf("change dress failed: %v", err)
	}

	var response protobuf.SC_21618
	decodePacketAt(t, client, 0, 21618, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected invalid wear to fail")
	}

	states, err := orm.ListIslandShipDressStates(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("list ship dress states: %v", err)
	}
	if len(states) != 1 || states[0].ShipID != 1005 || states[0].DressID != 9701 {
		t.Fatalf("expected unload rollback, got %+v", states)
	}
}

func TestIslandNPCActionAwardClaimAndDuplicate(t *testing.T) {
	client := setupHandlerCommander(t)
	resetIslandShipPacketTables(t, client.Commander.CommanderID)

	seedConfigEntry(t, islandStrollNPCCategory, "5001", `{"id":5001,"action_feedback":6001}`)
	seedConfigEntry(t, islandActionFeedbackCategory, "6001", `{"id":6001,"drop_id":7001}`)
	seedConfigEntry(t, islandDropDataTemplateCategory, "7001", `{"id":7001,"drop_list":[[41,9801,2]]}`)
	seedConfigEntry(t, islandSetCategory, "island_feedback_award_times", `{"key_value_int":3}`)

	payload := &protobuf.CS_21702{NpcId: proto.Uint32(5001), ActionFeedbackId: proto.Uint32(6001)}
	buffer, _ := proto.Marshal(payload)
	client.Buffer.Reset()
	if _, _, err := IslandGetNpcActionAward(&buffer, client); err != nil {
		t.Fatalf("npc action award failed: %v", err)
	}
	var response protobuf.SC_21703
	decodePacketAt(t, client, 0, 21703, &response)
	if response.GetResult() != 0 || len(response.GetDropList()) != 1 {
		t.Fatalf("expected first claim success with one drop")
	}

	client.Buffer.Reset()
	if _, _, err := IslandGetNpcActionAward(&buffer, client); err != nil {
		t.Fatalf("npc action award duplicate failed: %v", err)
	}
	var duplicate protobuf.SC_21703
	decodePacketAt(t, client, 0, 21703, &duplicate)
	if duplicate.GetResult() == 0 {
		t.Fatalf("expected duplicate claim failure")
	}
}
