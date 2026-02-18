package answer

import (
	"fmt"
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestCommanderReserveBoxSuccess(t *testing.T) {
	client := setupHandlerCommander(t)
	seedConfigEntry(t, "ShareCfg/gameset.json", "commander_get_cost", `{"key_value":0,"description":[100,200,300]}`)
	seedHandlerCommanderResource(t, client, 1, 500)

	payload := protobuf.CS_25018{Type: proto.Uint32(2)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := CommanderReserveBox(&buffer, client); err != nil {
		t.Fatalf("reserve box failed: %v", err)
	}

	var response protobuf.SC_25019
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success result")
	}
	if len(response.GetAwards()) != 2 {
		t.Fatalf("expected two awards, got %d", len(response.GetAwards()))
	}
	if client.Commander.GetResourceCount(1) != 200 {
		t.Fatalf("expected gold to be consumed")
	}
	if client.Commander.DrawCount1 != 2 {
		t.Fatalf("expected reserve usage count to increment")
	}
	if client.Commander.GetItemCount(20001) != 2 {
		t.Fatalf("expected reserve box rewards to persist in inventory")
	}
}

func TestCommanderReserveBoxInsufficientGold(t *testing.T) {
	client := setupHandlerCommander(t)
	seedConfigEntry(t, "ShareCfg/gameset.json", "commander_get_cost", `{"key_value":0,"description":[100]}`)
	seedHandlerCommanderResource(t, client, 1, 50)

	payload := protobuf.CS_25018{Type: proto.Uint32(1)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := CommanderReserveBox(&buffer, client); err != nil {
		t.Fatalf("reserve box failed: %v", err)
	}

	var response protobuf.SC_25019
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected failure result")
	}
	if client.Commander.DrawCount1 != 0 {
		t.Fatalf("expected usage count unchanged")
	}
}

func TestCommanderCatteryAssignStyleOperationAndScene(t *testing.T) {
	client := setupHandlerCommander(t)
	seedShipTemplate(t, 900101, 1, 3, 1, "Commander Seed", 1)
	owned := seedOwnedShip(t, client, 900101)
	seedConfigEntry(t, "ShareCfg/commander_data_template.json", fmt.Sprintf("%d", owned.ID), `{"ability":[1,2]}`)
	seedConfigEntry(t, "ShareCfg/commander_home.json", "1", `{"feed_level":[0,20],"nest_appearance":[1,2]}`)
	seedConfigEntry(t, "ShareCfg/commander_home_style.json", "2", `{"id":2}`)

	assignPayload := protobuf.CS_25030{Slotidx: proto.Uint32(1), CommanderId: proto.Uint32(owned.ID)}
	assignBuffer, err := proto.Marshal(&assignPayload)
	if err != nil {
		t.Fatalf("marshal assign payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := CommanderCatteryAssign(&assignBuffer, client); err != nil {
		t.Fatalf("assign failed: %v", err)
	}
	var assignResponse protobuf.SC_25031
	decodeResponse(t, client, &assignResponse)
	if assignResponse.GetResult() != 0 {
		t.Fatalf("expected assign success")
	}

	assignDuplicatePayload := protobuf.CS_25030{Slotidx: proto.Uint32(2), CommanderId: proto.Uint32(owned.ID)}
	assignDuplicateBuffer, err := proto.Marshal(&assignDuplicatePayload)
	if err != nil {
		t.Fatalf("marshal duplicate assign payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := CommanderCatteryAssign(&assignDuplicateBuffer, client); err != nil {
		t.Fatalf("duplicate assign failed: %v", err)
	}
	var duplicateAssignResponse protobuf.SC_25031
	decodeResponse(t, client, &duplicateAssignResponse)
	if duplicateAssignResponse.GetResult() == 0 {
		t.Fatalf("expected duplicate commander assignment to fail")
	}

	home, slots, err := orm.GetCommanderHome(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load home: %v", err)
	}
	if slots[0].OpFlag != 3 {
		t.Fatalf("expected unsupported play flag to be removed")
	}

	stylePayload := protobuf.CS_25032{Slotidx: proto.Uint32(1), Styleidx: proto.Uint32(2)}
	styleBuffer, err := proto.Marshal(&stylePayload)
	if err != nil {
		t.Fatalf("marshal style payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := CommanderCatteryStyle(&styleBuffer, client); err != nil {
		t.Fatalf("style failed: %v", err)
	}
	var styleResponse protobuf.SC_25033
	decodeResponse(t, client, &styleResponse)
	if styleResponse.GetResult() != 0 {
		t.Fatalf("expected style success")
	}

	execAnswerTestSQLT(t, "UPDATE owned_ships SET exp = $1, level = $2, max_level = $3 WHERE owner_id = $4 AND id = $5", int64(90), int64(1), int64(2), int64(client.Commander.CommanderID), int64(owned.ID))
	if err := client.Commander.Load(); err != nil {
		t.Fatalf("reload commander: %v", err)
	}
	opPayload := protobuf.CS_25028{Type: proto.Uint32(2)}
	opBuffer, err := proto.Marshal(&opPayload)
	if err != nil {
		t.Fatalf("marshal op payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := CommanderCatteryOperation(&opBuffer, client); err != nil {
		t.Fatalf("operation failed: %v", err)
	}
	var opResponse protobuf.SC_25029
	decodeResponse(t, client, &opResponse)
	if opResponse.GetResult() != 0 {
		t.Fatalf("expected operation success")
	}
	if err := client.Commander.Load(); err != nil {
		t.Fatalf("reload commander: %v", err)
	}
	if client.Commander.OwnedShipsMap[owned.ID].Level != 2 {
		t.Fatalf("expected feed operation to apply commander exp")
	}

	execAnswerTestSQLT(t, "UPDATE commander_home_slots SET cache_exp = $1 WHERE commander_id = $2", int64(55), int64(client.Commander.CommanderID))
	openPayload := protobuf.CS_25036{IsOpen: proto.Uint32(0)}
	openBuffer, err := proto.Marshal(&openPayload)
	if err != nil {
		t.Fatalf("marshal open payload: %v", err)
	}
	if _, _, err := CommanderCatterySceneState(&openBuffer, client); err != nil {
		t.Fatalf("open scene failed: %v", err)
	}
	home, slots, err = orm.GetCommanderHome(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load home after open: %v", err)
	}
	if !home.SceneOpen {
		t.Fatalf("expected scene to be open")
	}
	if slots[0].CacheExp != 0 {
		t.Fatalf("expected cache exp to be cleared")
	}

	closePayload := protobuf.CS_25036{IsOpen: proto.Uint32(1)}
	closeBuffer, err := proto.Marshal(&closePayload)
	if err != nil {
		t.Fatalf("marshal close payload: %v", err)
	}
	if _, _, err := CommanderCatterySceneState(&closeBuffer, client); err != nil {
		t.Fatalf("close scene failed: %v", err)
	}
	home, _, err = orm.GetCommanderHome(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load home after close: %v", err)
	}
	if home.SceneOpen {
		t.Fatalf("expected scene to be closed")
	}
}

func TestCommanderBoxesRefreshOrdering(t *testing.T) {
	client := setupHandlerCommander(t)
	boxes, err := orm.EnsureCommanderBoxes(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("ensure commander boxes: %v", err)
	}
	if len(boxes) < 2 {
		t.Fatalf("expected at least two commander boxes")
	}
	execAnswerTestSQLT(t, "UPDATE commander_boxes SET pool_id = $3, begin_time = $4, finish_time = $5 WHERE commander_id = $1 AND box_id = $2", int64(client.Commander.CommanderID), int64(1), int64(2), int64(100), int64(300))
	execAnswerTestSQLT(t, "UPDATE commander_boxes SET pool_id = $3, begin_time = $4, finish_time = $5 WHERE commander_id = $1 AND box_id = $2", int64(client.Commander.CommanderID), int64(2), int64(3), int64(200), int64(600))

	payload := protobuf.CS_25034{Type: proto.Uint32(9)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := CommanderBoxesRefresh(&buffer, client); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	var response protobuf.SC_25035
	decodeResponse(t, client, &response)
	if len(response.GetBoxList()) < 2 {
		t.Fatalf("expected commander boxes in response")
	}
	if response.GetBoxList()[0].GetId() != 1 || response.GetBoxList()[1].GetId() != 2 {
		t.Fatalf("expected deterministic id ordering")
	}
	if response.GetBoxList()[0].GetBeginTime() != 100 {
		t.Fatalf("expected begin_time to be populated from meow box state")
	}
}
