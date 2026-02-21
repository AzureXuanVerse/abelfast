package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	shipyardTestBlueprintID = uint32(70001)
	shipyardTestShipID      = uint32(700011)
)

func setupShipyardBlueprintTest(t *testing.T) *connection.Client {
	t.Helper()
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.CommanderShipyardBlueprint{})
	clearTable(t, &orm.CommanderShipyardState{})
	clearTable(t, &orm.CommanderTask{})
	seedShipyardConfig(t)
	seedShipyardShipTemplate(t)
	execAnswerTestSQLT(t, `
INSERT INTO commander_tasks (commander_id, task_id, progress, accept_time, submit_time)
VALUES ($1, 9001, 1, 1, 1), ($1, 9002, 1, 1, 1), ($1, 9003, 0, 1, 0)
ON CONFLICT (commander_id, task_id) DO UPDATE SET progress = EXCLUDED.progress, submit_time = EXCLUDED.submit_time
`, int64(client.Commander.CommanderID))
	if err := client.Commander.AddItem(40100, 10); err != nil {
		t.Fatalf("seed speedup item: %v", err)
	}
	if err := client.Commander.AddItem(42011, 20); err != nil {
		t.Fatalf("seed strengthen item: %v", err)
	}
	if err := client.Commander.AddItem(40125, 1); err != nil {
		t.Fatalf("seed unlock item: %v", err)
	}
	if err := client.Commander.AddResource(1, 100000); err != nil {
		t.Fatalf("seed gold: %v", err)
	}
	return client
}

func seedShipyardConfig(t *testing.T) {
	t.Helper()
	seedConfigEntry(t, "ShareCfg/ship_data_blueprint.json", "70001", `{"id":70001,"blueprint_version":1,"unlock_task_open_condition":[9001],"unlock_task":[[9002,0],[0,0],[0,0],[9003,0]],"strengthen_effect":[9101],"fate_strengthen":[9102],"strengthen_item":42011,"gain_item_id":[40125],"is_pursuing":1,"price":100}`)
	seedConfigEntry(t, "ShareCfg/ship_strengthen_blueprint.json", "9101", `{"id":9101,"lv":1,"need_exp":10,"need_lv":1}`)
	seedConfigEntry(t, "ShareCfg/ship_strengthen_blueprint.json", "9102", `{"id":9102,"lv":2,"need_exp":10,"need_lv":1}`)
	seedConfigEntry(t, "ShareCfg/task_data_template.json", "9001", `{"id":9001,"target_num":1}`)
	seedConfigEntry(t, "ShareCfg/task_data_template.json", "9002", `{"id":9002,"target_num":1}`)
	seedConfigEntry(t, "ShareCfg/task_data_template.json", "9003", `{"id":9003,"target_num":100}`)
	seedConfigEntry(t, "ShareCfg/gameset.json", "technology_catchup_itemid", `{"description":[[40100,5]]}`)
	seedConfigEntry(t, "ShareCfg/gameset.json", "blueprint_pursue_discount_ssr", `{"description":[0,50,100]}`)
	seedConfigEntry(t, "ShareCfg/gameset.json", "blueprint_pursue_discount_ur", `{"description":[0,50,100]}`)
	seedConfigEntry(t, "sharecfgdata/item_data_statistics.json", "42011", `{"id":42011,"usage_arg":[10]}`)
}

func seedShipyardShipTemplate(t *testing.T) {
	t.Helper()
	execAnswerTestSQLT(t, `
INSERT INTO ships (template_id, name, english_name, rarity_id, star, type, nationality, build_time)
VALUES ($1, 'Blueprint Test', 'Blueprint Test', 5, 6, 2, 1, 0)
ON CONFLICT (template_id) DO NOTHING
`, int64(shipyardTestShipID))
}

func TestShipyardStartStopResumeFlow(t *testing.T) {
	client := setupShipyardBlueprintTest(t)

	startPayload := protobuf.CS_63200{BlueprintId: proto.Uint32(shipyardTestBlueprintID)}
	startBuffer, _ := proto.Marshal(&startPayload)
	client.Buffer.Reset()
	if _, _, err := StartShipBlueprintDevelopment(&startBuffer, client); err != nil {
		t.Fatalf("start handler failed: %v", err)
	}
	var startResponse protobuf.SC_63201
	decodeResponse(t, client, &startResponse)
	if startResponse.GetResult() != 0 || startResponse.GetTime() == 0 {
		t.Fatalf("expected successful start with time, got result=%d time=%d", startResponse.GetResult(), startResponse.GetTime())
	}

	stopPayload := protobuf.CS_63206{BlueprintId: proto.Uint32(shipyardTestBlueprintID)}
	stopBuffer, _ := proto.Marshal(&stopPayload)
	client.Buffer.Reset()
	if _, _, err := StopShipBlueprint(&stopBuffer, client); err != nil {
		t.Fatalf("stop handler failed: %v", err)
	}
	var stopResponse protobuf.SC_63207
	decodeResponse(t, client, &stopResponse)
	if stopResponse.GetResult() != 0 {
		t.Fatalf("expected stop success")
	}

	resumePayload := protobuf.CS_63208{BlueprintId: proto.Uint32(shipyardTestBlueprintID)}
	resumeBuffer, _ := proto.Marshal(&resumePayload)
	client.Buffer.Reset()
	if _, _, err := ResumeShipBlueprint(&resumeBuffer, client); err != nil {
		t.Fatalf("resume handler failed: %v", err)
	}
	var resumeResponse protobuf.SC_63209
	decodeResponse(t, client, &resumeResponse)
	if resumeResponse.GetResult() != 0 {
		t.Fatalf("expected resume success")
	}
}

func TestShipyardSpeedupAndPursueAndMod(t *testing.T) {
	client := setupShipyardBlueprintTest(t)

	speedupPayload := protobuf.CS_63210{Blueprintid: proto.Uint32(shipyardTestBlueprintID), Itemid: proto.Uint32(40100), Number: proto.Uint32(2), TaskId: proto.Uint32(9003)}
	speedupBuffer, _ := proto.Marshal(&speedupPayload)
	client.Buffer.Reset()
	if _, _, err := UseTechSpeedupItem(&speedupBuffer, client); err != nil {
		t.Fatalf("speedup handler failed: %v", err)
	}
	var speedupResponse protobuf.SC_63211
	decodeResponse(t, client, &speedupResponse)
	if speedupResponse.GetResult() != 0 {
		t.Fatalf("expected speedup success")
	}
}

func TestShipyardModAndPursue(t *testing.T) {
	client := setupShipyardBlueprintTest(t)
	ship, err := client.Commander.AddShip(shipyardTestShipID)
	if err != nil {
		t.Fatalf("seed owned ship failed: %v", err)
	}
	execAnswerTestSQLT(t, `
INSERT INTO commander_shipyard_blueprints (commander_id, blueprint_id, ship_id, start_time, blue_print_level, exp, start_duration)
VALUES ($1, $2, $3, 0, 0, 0, 0)
ON CONFLICT (commander_id, blueprint_id)
DO UPDATE SET ship_id = EXCLUDED.ship_id, blue_print_level = 0, exp = 0
`, int64(client.Commander.CommanderID), int64(shipyardTestBlueprintID), int64(ship.ID))

	unlockPayload := protobuf.CS_63214{Group: proto.Uint32(shipyardTestBlueprintID), Itemid: proto.Uint32(40125)}
	unlockBuffer, _ := proto.Marshal(&unlockPayload)
	client.Buffer.Reset()
	if _, _, err := ItemUnlockShipBlueprint(&unlockBuffer, client); err != nil {
		t.Fatalf("unlock handler failed: %v", err)
	}
	var unlockResponse protobuf.SC_63215
	decodeResponse(t, client, &unlockResponse)
	if unlockResponse.GetResult() == 0 {
		t.Fatalf("expected unlock to fail once blueprint already linked to a ship")
	}

	modPayload := protobuf.CS_63204{ShipId: proto.Uint32(ship.ID), Count: proto.Uint32(1)}
	modBuffer, _ := proto.Marshal(&modPayload)
	client.Buffer.Reset()
	if _, _, err := ModShipBlueprint(&modBuffer, client); err != nil {
		t.Fatalf("mod handler failed: %v", err)
	}
	var modResponse protobuf.SC_63205
	decodeResponse(t, client, &modResponse)
	if modResponse.GetResult() != 0 {
		t.Fatalf("expected mod success")
	}

	pursuePayload := protobuf.CS_63212{ShipId: proto.Uint32(ship.ID), Count: proto.Uint32(2)}
	pursueBuffer, _ := proto.Marshal(&pursuePayload)
	client.Buffer.Reset()
	if _, _, err := PursueShipBlueprint(&pursueBuffer, client); err != nil {
		t.Fatalf("pursue handler failed: %v", err)
	}
	var pursueResponse protobuf.SC_63213
	decodeResponse(t, client, &pursueResponse)
	if pursueResponse.GetResult() != 0 {
		t.Fatalf("expected pursue success")
	}
}

func TestShipyardFinishBlueprint(t *testing.T) {
	client := setupShipyardBlueprintTest(t)
	execAnswerTestSQLT(t, `
UPDATE commander_tasks
SET progress = 100, submit_time = 1
WHERE commander_id = $1 AND task_id = 9003
`, int64(client.Commander.CommanderID))

	execAnswerTestSQLT(t, `
INSERT INTO commander_shipyard_blueprints (commander_id, blueprint_id, ship_id, start_time, blue_print_level, exp, start_duration)
VALUES ($1, $2, 0, 10, 0, 0, 0)
ON CONFLICT (commander_id, blueprint_id)
DO UPDATE SET start_time = 10, blue_print_level = 0, exp = 0, ship_id = 0
`, int64(client.Commander.CommanderID), int64(shipyardTestBlueprintID))

	payload := protobuf.CS_63202{BlueprintId: proto.Uint32(shipyardTestBlueprintID)}
	buffer, _ := proto.Marshal(&payload)
	client.Buffer.Reset()
	if _, _, err := FinishShipBlueprint(&buffer, client); err != nil {
		t.Fatalf("finish handler failed: %v", err)
	}
	var response protobuf.SC_63203
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 || response.GetShip() == nil {
		t.Fatalf("expected finish success with ship")
	}
}

func TestShipyardResumeRejectsWhenAnotherDevelopmentActive(t *testing.T) {
	client := setupShipyardBlueprintTest(t)

	startPayload := protobuf.CS_63200{BlueprintId: proto.Uint32(shipyardTestBlueprintID)}
	startBuffer, _ := proto.Marshal(&startPayload)
	client.Buffer.Reset()
	if _, _, err := StartShipBlueprintDevelopment(&startBuffer, client); err != nil {
		t.Fatalf("start handler failed: %v", err)
	}

	const pausedBlueprintID = uint32(70002)
	execAnswerTestSQLT(t, `
INSERT INTO commander_shipyard_blueprints (commander_id, blueprint_id, ship_id, start_time, blue_print_level, exp, start_duration)
VALUES ($1, $2, 0, 0, 0, 0, 60)
ON CONFLICT (commander_id, blueprint_id)
DO UPDATE SET ship_id = 0, start_time = 0, start_duration = 60
`, int64(client.Commander.CommanderID), int64(pausedBlueprintID))

	resumePayload := protobuf.CS_63208{BlueprintId: proto.Uint32(pausedBlueprintID)}
	resumeBuffer, _ := proto.Marshal(&resumePayload)
	client.Buffer.Reset()
	if _, _, err := ResumeShipBlueprint(&resumeBuffer, client); err != nil {
		t.Fatalf("resume handler failed: %v", err)
	}
	var resumeResponse protobuf.SC_63209
	decodeResponse(t, client, &resumeResponse)
	if resumeResponse.GetResult() == 0 {
		t.Fatalf("expected resume to fail when another blueprint is active")
	}
}

func TestShipyardDataUsesPersistedRows(t *testing.T) {
	client := setupShipyardBlueprintTest(t)
	execAnswerTestSQLT(t, `
INSERT INTO commander_shipyard_states (commander_id, cold_time, daily_catchup_strengthen, daily_catchup_strengthen_ur)
VALUES ($1, 123, 4, 5)
ON CONFLICT (commander_id)
DO UPDATE SET cold_time = 123, daily_catchup_strengthen = 4, daily_catchup_strengthen_ur = 5
`, int64(client.Commander.CommanderID))
	execAnswerTestSQLT(t, `
INSERT INTO commander_shipyard_blueprints (commander_id, blueprint_id, ship_id, start_time, blue_print_level, exp, start_duration)
VALUES ($1, $2, 0, 10, 1, 2, 3)
ON CONFLICT (commander_id, blueprint_id)
DO UPDATE SET start_time = 10, blue_print_level = 1, exp = 2, start_duration = 3
`, int64(client.Commander.CommanderID), int64(shipyardTestBlueprintID))

	buffer := []byte{}
	client.Buffer.Reset()
	if _, _, err := ShipyardData(&buffer, client); err != nil {
		t.Fatalf("shipyard data failed: %v", err)
	}
	var response protobuf.SC_63100
	decodeResponse(t, client, &response)
	if response.GetColdTime() != 123 || response.GetDailyCatchupStrengthen() != 4 || response.GetDailyCatchupStrengthenUr() != 5 {
		t.Fatalf("unexpected shipyard state values")
	}
	if len(response.GetBlueprintList()) != 1 || response.GetBlueprintList()[0].GetId() != shipyardTestBlueprintID {
		t.Fatalf("expected persisted blueprint list")
	}
}
