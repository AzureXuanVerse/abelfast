package answer

import (
	"context"
	"os"
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func setupActivityTaskTestClient(t *testing.T) *connection.Client {
	t.Helper()
	os.Setenv("MODE", "test")
	orm.InitDatabase()

	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderActivityTask{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.CommanderMiscItem{})
	clearTable(t, &orm.OwnedResource{})
	clearTable(t, &orm.Resource{})
	clearTable(t, &orm.Item{})
	clearTable(t, &orm.Commander{})

	if err := orm.CreateCommanderRoot(9001, 9001, "Activity Task Tester", 0, 0); err != nil {
		t.Fatalf("create commander: %v", err)
	}
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `INSERT INTO resources (id, item_id, name) VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING`, int64(1), int64(0), "Gold"); err != nil {
		t.Fatalf("seed resource: %v", err)
	}
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `INSERT INTO items (id, name, rarity, shop_id, type, virtual_type) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (id) DO NOTHING`, int64(20001), "Reward Item", int64(1), int64(-2), int64(1), int64(0)); err != nil {
		t.Fatalf("seed reward item: %v", err)
	}
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `INSERT INTO items (id, name, rarity, shop_id, type, virtual_type) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (id) DO NOTHING`, int64(quickTaskTicketItemID), "Quick Task Ticket", int64(1), int64(-2), int64(1), int64(0)); err != nil {
		t.Fatalf("seed ticket item: %v", err)
	}
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `INSERT INTO commander_items (commander_id, item_id, count) VALUES ($1, $2, $3)`, int64(9001), int64(quickTaskTicketItemID), int64(5)); err != nil {
		t.Fatalf("seed ticket count: %v", err)
	}

	seedConfigEntry(t, "ShareCfg/activity_template.json", "5000", `{"id":5000,"type":36,"config_data":[7001,7002]}`)
	seedConfigEntry(t, "ShareCfg/task_data_template.json", "7001", `{"id":7001,"target_num":5,"quick_finish":2,"award_display":[[1,1,100],[2,20001,1]]}`)
	seedConfigEntry(t, "ShareCfg/task_data_template.json", "7002", `{"id":7002,"target_num":3,"quick_finish":1,"award_display":[[1,1,20]]}`)

	commander := &orm.Commander{CommanderID: 9001}
	if err := commander.Load(); err != nil {
		t.Fatalf("load commander: %v", err)
	}
	return &connection.Client{Commander: commander}
}

func TestSubmitActivityTaskSuccess(t *testing.T) {
	client := setupActivityTaskTestClient(t)
	seedActivityTaskProgress(t, 9001, 5000, 7001, 5)

	payload := &protobuf.CS_20205{ActId: proto.Uint32(5000), TaskIds: []uint32{7001}}
	buffer, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if _, packetID, err := SubmitActivityTask(&buffer, client); err != nil {
		t.Fatalf("submit activity task failed: %v", err)
	} else if packetID != 20206 {
		t.Fatalf("expected packet 20206, got %d", packetID)
	}

	var response protobuf.SC_20206
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}
	if len(response.GetAwardList()) != 2 {
		t.Fatalf("expected 2 awards, got %d", len(response.GetAwardList()))
	}

	state, err := orm.GetCommanderActivityTask(9001, 5000, 7001)
	if err != nil {
		t.Fatalf("load task state: %v", err)
	}
	if !state.Submitted {
		t.Fatalf("expected submitted state")
	}
}

func TestSubmitActivityTaskRejectsIncompleteTask(t *testing.T) {
	client := setupActivityTaskTestClient(t)
	seedActivityTaskProgress(t, 9001, 5000, 7001, 4)

	payload := &protobuf.CS_20205{ActId: proto.Uint32(5000), TaskIds: []uint32{7001}}
	buffer, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if _, packetID, err := SubmitActivityTask(&buffer, client); err != nil {
		t.Fatalf("submit activity task failed: %v", err)
	} else if packetID != 20206 {
		t.Fatalf("expected packet 20206, got %d", packetID)
	}

	var response protobuf.SC_20206
	decodeResponse(t, client, &response)
	if response.GetResult() != 1 {
		t.Fatalf("expected failure result, got %d", response.GetResult())
	}
	if len(response.GetAwardList()) != 0 {
		t.Fatalf("expected no awards, got %d", len(response.GetAwardList()))
	}
}

func TestQuickFinishActivityTaskConsumesTicket(t *testing.T) {
	client := setupActivityTaskTestClient(t)

	payload := &protobuf.CS_20207{ActId: proto.Uint32(5000), TaskId: proto.Uint32(7002), ItemCost: proto.Uint32(1)}
	buffer, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if _, packetID, err := QuickFinishActivityTask(&buffer, client); err != nil {
		t.Fatalf("quick finish failed: %v", err)
	} else if packetID != 20208 {
		t.Fatalf("expected packet 20208, got %d", packetID)
	}

	var response protobuf.SC_20208
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}

	if client.Commander.GetItemCount(quickTaskTicketItemID) != 4 {
		t.Fatalf("expected ticket count 4, got %d", client.Commander.GetItemCount(quickTaskTicketItemID))
	}
}

func TestUpdateLowPriorityActivityTaskProgressModes(t *testing.T) {
	client := setupActivityTaskTestClient(t)

	payload := &protobuf.CS_20209{
		Progressinfo: []*protobuf.ACT_TASK_UPDATE{
			{ActId: proto.Uint32(5000), TaskId: proto.Uint32(7001), Mode: proto.Uint32(orm.ActivityTaskProgressModeSet), Progress: proto.Uint32(3)},
			{ActId: proto.Uint32(5000), TaskId: proto.Uint32(7001), Mode: proto.Uint32(orm.ActivityTaskProgressModeAppend), Progress: proto.Uint32(4)},
		},
	}
	buffer, err := proto.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if _, packetID, err := UpdateLowPriorityActivityTaskProgress(&buffer, client); err != nil {
		t.Fatalf("update progress failed: %v", err)
	} else if packetID != 20210 {
		t.Fatalf("expected packet 20210, got %d", packetID)
	}

	var response protobuf.SC_20210
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}

	state, err := orm.GetCommanderActivityTask(9001, 5000, 7001)
	if err != nil {
		t.Fatalf("load task state: %v", err)
	}
	if state.Progress != 7 {
		t.Fatalf("expected progress 7, got %d", state.Progress)
	}
}

func TestActivityTaskStateSyncSendsAllPackets(t *testing.T) {
	client := setupActivityTaskTestClient(t)

	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO commander_activity_tasks (commander_id, act_id, task_id, progress, submitted)
VALUES ($1, $2, $3, $4, $5)
`, int64(9001), int64(5000), int64(7001), int64(9), true); err != nil {
		t.Fatalf("seed activity task row: %v", err)
	}

	empty := []byte{}
	if _, packetID, err := ActivityTaskStateSync(&empty, client); err != nil {
		t.Fatalf("activity sync failed: %v", err)
	} else if packetID != 20204 {
		t.Fatalf("expected packet 20204, got %d", packetID)
	}

	offset := 0
	var sc20201 protobuf.SC_20201
	offset = decodePacketAt(t, client, offset, 20201, &sc20201)
	if len(sc20201.GetInfo()) != 1 {
		t.Fatalf("expected one init list")
	}
	if len(sc20201.GetInfo()[0].GetFinishIds()) != 1 || sc20201.GetInfo()[0].GetFinishIds()[0] != 7001 {
		t.Fatalf("expected finished task 7001")
	}

	var sc20202 protobuf.SC_20202
	offset = decodePacketAt(t, client, offset, 20202, &sc20202)
	var sc20203 protobuf.SC_20203
	offset = decodePacketAt(t, client, offset, 20203, &sc20203)
	var sc20204 protobuf.SC_20204
	_ = decodePacketAt(t, client, offset, 20204, &sc20204)
}

func seedActivityTaskProgress(t *testing.T, commanderID uint32, actID uint32, taskID uint32, progress uint32) {
	t.Helper()
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO commander_activity_tasks (commander_id, act_id, task_id, progress, submitted)
VALUES ($1, $2, $3, $4, false)
`, int64(commanderID), int64(actID), int64(taskID), int64(progress)); err != nil {
		t.Fatalf("seed activity task progress: %v", err)
	}
}
