package answer

import (
	"encoding/json"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func TestCommanderBuildBoxStartAndClaim(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.CommanderBox{})
	clearTable(t, &orm.CommanderMeow{})
	clearTable(t, &orm.ConfigEntry{})

	if err := client.Commander.SetItem(20011, 1); err != nil {
		t.Fatalf("set material item: %v", err)
	}
	if err := orm.UpsertConfigEntry("ShareCfg/commander_data_create_material.json", "1", json.RawMessage(`{"id":1,"use_item":20011,"number_1":1}`)); err != nil {
		t.Fatalf("seed create material: %v", err)
	}
	if err := orm.UpsertConfigEntry("ShareCfg/commander_data_template.json", "10011", json.RawMessage(`{"id":10011,"rarity":5,"group_type":1001,"exp":200,"exp_cost":20}`)); err != nil {
		t.Fatalf("seed template: %v", err)
	}

	start := protobuf.CS_25002{Boxid: proto.Uint32(1)}
	startBuf, _ := proto.Marshal(&start)
	if _, _, err := CommanderBuildBoxStart(&startBuf, client); err != nil {
		t.Fatalf("start box failed: %v", err)
	}

	var startResp protobuf.SC_25003
	decodePacketAt(t, client, 0, 25003, &startResp)
	if startResp.GetResult() != 0 {
		t.Fatalf("expected start result 0, got %d", startResp.GetResult())
	}
	if startResp.GetBox().GetPoolId() != 1 {
		t.Fatalf("expected pool 1, got %d", startResp.GetBox().GetPoolId())
	}

	execAnswerTestSQLT(t, "UPDATE commander_boxes SET finish_time = $3 WHERE commander_id = $1 AND box_id = $2", int64(client.Commander.CommanderID), int64(1), int64(time.Now().Unix()-1))
	client.Buffer.Reset()

	restart := protobuf.CS_25002{Boxid: proto.Uint32(1)}
	restartBuf, _ := proto.Marshal(&restart)
	if _, _, err := CommanderBuildBoxStart(&restartBuf, client); err != nil {
		t.Fatalf("restart completed box failed: %v", err)
	}
	var restartResp protobuf.SC_25003
	decodePacketAt(t, client, 0, 25003, &restartResp)
	if restartResp.GetResult() == 0 {
		t.Fatalf("expected restart to fail until completed box is claimed")
	}
	client.Buffer.Reset()

	claim := protobuf.CS_25004{Boxid: proto.Uint32(1)}
	claimBuf, _ := proto.Marshal(&claim)
	if _, _, err := CommanderClaimBox(&claimBuf, client); err != nil {
		t.Fatalf("claim box failed: %v", err)
	}

	var claimResp protobuf.SC_25005
	decodePacketAt(t, client, 0, 25005, &claimResp)
	if claimResp.GetResult() != 0 {
		t.Fatalf("expected claim result 0, got %d", claimResp.GetResult())
	}
	if claimResp.GetCommander().GetId() == 0 {
		t.Fatalf("expected claimed commander id")
	}
}

func TestCommanderFleetEquipAndUpgrade(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.CommanderBox{})
	clearTable(t, &orm.CommanderMeow{})
	clearTable(t, &orm.ConfigEntry{})

	execAnswerTestSQLT(t, `INSERT INTO fleets (game_id, commander_id, name, ship_list, meowfficer_list) VALUES ($1, $2, $3, '[]'::jsonb, '[]'::jsonb)`, int64(1), int64(client.Commander.CommanderID), "Fleet")
	if err := client.Commander.Load(); err != nil {
		t.Fatalf("reload commander: %v", err)
	}

	if err := orm.UpsertConfigEntry("ShareCfg/commander_data_template.json", "10011", json.RawMessage(`{"id":10011,"rarity":5,"group_type":1001,"exp":100,"exp_cost":10}`)); err != nil {
		t.Fatalf("seed template 10011: %v", err)
	}
	if err := orm.UpsertConfigEntry("ShareCfg/commander_data_template.json", "10021", json.RawMessage(`{"id":10021,"rarity":5,"group_type":1001,"exp":200,"exp_cost":15}`)); err != nil {
		t.Fatalf("seed template 10021: %v", err)
	}
	if err := orm.UpsertConfigEntry("ShareCfg/gameset.json", "commander_exp_same_rate", json.RawMessage(`{"key_value":12000}`)); err != nil {
		t.Fatalf("seed same rate: %v", err)
	}
	if err := client.Commander.SetResource(1, 1000); err != nil {
		t.Fatalf("set gold: %v", err)
	}

	execAnswerTestSQLT(t, `INSERT INTO commander_meows (commander_id, template_id, level, exp, is_locked, used_pt) VALUES ($1, $2, 1, 0, 0, 0)`, int64(client.Commander.CommanderID), int64(10011))
	execAnswerTestSQLT(t, `INSERT INTO commander_meows (commander_id, template_id, level, exp, is_locked, used_pt) VALUES ($1, $2, 1, 0, 0, 0)`, int64(client.Commander.CommanderID), int64(10021))

	meows, err := orm.ListCommanderMeows(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("list meows: %v", err)
	}
	if len(meows) < 2 {
		t.Fatalf("expected 2 meows, got %d", len(meows))
	}

	equip := protobuf.CS_25006{Groupid: proto.Uint32(1), Pos: proto.Uint32(1), Commanderid: proto.Uint32(meows[1].ID)}
	equipBuf, _ := proto.Marshal(&equip)
	if _, _, err := CommanderFleetEquip(&equipBuf, client); err != nil {
		t.Fatalf("equip failed: %v", err)
	}
	var equipResp protobuf.SC_25007
	decodePacketAt(t, client, 0, 25007, &equipResp)
	if equipResp.GetResult() != 0 {
		t.Fatalf("expected equip success, got %d", equipResp.GetResult())
	}

	client.Buffer.Reset()
	upgrade := protobuf.CS_25008{Targetid: proto.Uint32(meows[0].ID), Materialid: []uint32{meows[1].ID}}
	upgradeBuf, _ := proto.Marshal(&upgrade)
	if _, _, err := CommanderUpgrade(&upgradeBuf, client); err != nil {
		t.Fatalf("upgrade failed: %v", err)
	}
	var upgradeResp protobuf.SC_25009
	decodePacketAt(t, client, 0, 25009, &upgradeResp)
	if upgradeResp.GetResult() == 0 {
		t.Fatalf("expected upgrade failure while material is equipped")
	}

	client.Buffer.Reset()
	remove := protobuf.CS_25006{Groupid: proto.Uint32(1), Pos: proto.Uint32(1), Commanderid: proto.Uint32(0)}
	removeBuf, _ := proto.Marshal(&remove)
	if _, _, err := CommanderFleetEquip(&removeBuf, client); err != nil {
		t.Fatalf("remove equip failed: %v", err)
	}

	client.Buffer.Reset()
	if _, _, err := CommanderUpgrade(&upgradeBuf, client); err != nil {
		t.Fatalf("upgrade failed after remove: %v", err)
	}
	decodePacketAt(t, client, 0, 25009, &upgradeResp)
	if upgradeResp.GetResult() != 0 {
		t.Fatalf("expected upgrade success, got %d", upgradeResp.GetResult())
	}
}

func TestCommanderQuickFinishAndRefresh(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.CommanderBox{})
	clearTable(t, &orm.ConfigEntry{})

	if err := client.Commander.SetItem(20010, 1); err != nil {
		t.Fatalf("set quick finish item: %v", err)
	}

	now := uint32(time.Now().Unix())
	execAnswerTestSQLT(t, `INSERT INTO commander_boxes (commander_id, box_id, pool_id, begin_time, finish_time) VALUES ($1, $2, $3, $4, $5)`, int64(client.Commander.CommanderID), int64(1), int64(1), int64(now-100), int64(now+600))

	packet := protobuf.CS_25037{ItemCnt: proto.Uint32(1), FinishCnt: proto.Uint32(1), AffectCnt: proto.Uint32(1)}
	buffer, _ := proto.Marshal(&packet)
	if _, _, err := CommanderQuicklyFinishBoxes(&buffer, client); err != nil {
		t.Fatalf("quick finish failed: %v", err)
	}
	var quickResp protobuf.SC_25038
	decodePacketAt(t, client, 0, 25038, &quickResp)
	if quickResp.GetResult() != 0 {
		t.Fatalf("expected quick finish success, got %d", quickResp.GetResult())
	}

	client.Buffer.Reset()
	refresh := protobuf.CS_25034{Type: proto.Uint32(0)}
	refreshBuf, _ := proto.Marshal(&refresh)
	if _, _, err := CommanderRefreshBoxes(&refreshBuf, client); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	var refreshResp protobuf.SC_25035
	decodePacketAt(t, client, 0, 25035, &refreshResp)
	if len(refreshResp.GetBoxList()) == 0 || refreshResp.GetBoxList()[0].GetFinishTime() > uint32(time.Now().Unix()) {
		t.Fatalf("expected finished box in refresh")
	}
}
