package island

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func TestIslandSetTraceTaskSuccessActiveTask(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.IslandTaskProgress{})

	seedErr := orm.WithIslandTaskProgressTx(client.Commander.CommanderID, nowUTC(), func(state *orm.IslandTaskProgress) error {
		state.TraceTaskID = 7001
		state.TraceDailyTaskID = 7777
		state.ActiveTasks = []orm.IslandTaskEntry{
			{TaskID: 7001, Timestamp: 1},
			{TaskID: 7002, Timestamp: 2},
		}
		return nil
	})
	if seedErr != nil {
		t.Fatalf("seed island state: %v", seedErr)
	}

	payload := protobuf.CS_21034{TaskId: proto.Uint32(7002)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := IslandSetTraceTask(&buffer, client); err != nil {
		t.Fatalf("set trace task failed: %v", err)
	}

	var response protobuf.SC_21035
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}

	state, err := orm.LoadIslandTaskProgress(client.Commander.CommanderID, nowUTC())
	if err != nil {
		t.Fatalf("load island state: %v", err)
	}
	if state.TraceTaskID != 7002 {
		t.Fatalf("expected trace task id 7002, got %d", state.TraceTaskID)
	}
	if state.TraceDailyTaskID != 7777 {
		t.Fatalf("expected trace daily task id unchanged, got %d", state.TraceDailyTaskID)
	}
}

func TestIslandSetTraceTaskClearZero(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.IslandTaskProgress{})

	seedErr := orm.WithIslandTaskProgressTx(client.Commander.CommanderID, nowUTC(), func(state *orm.IslandTaskProgress) error {
		state.TraceTaskID = 7001
		state.TraceDailyTaskID = 8888
		state.ActiveTasks = []orm.IslandTaskEntry{{TaskID: 7001, Timestamp: 1}}
		return nil
	})
	if seedErr != nil {
		t.Fatalf("seed island state: %v", seedErr)
	}

	payload := protobuf.CS_21034{TaskId: proto.Uint32(0)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := IslandSetTraceTask(&buffer, client); err != nil {
		t.Fatalf("clear trace task failed: %v", err)
	}

	var response protobuf.SC_21035
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}

	state, err := orm.LoadIslandTaskProgress(client.Commander.CommanderID, nowUTC())
	if err != nil {
		t.Fatalf("load island state: %v", err)
	}
	if state.TraceTaskID != 0 {
		t.Fatalf("expected trace task id 0, got %d", state.TraceTaskID)
	}
	if state.TraceDailyTaskID != 8888 {
		t.Fatalf("expected trace daily task id unchanged, got %d", state.TraceDailyTaskID)
	}
}

func TestIslandSetTraceTaskRejectsUnknownTask(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.IslandTaskProgress{})

	seedErr := orm.WithIslandTaskProgressTx(client.Commander.CommanderID, nowUTC(), func(state *orm.IslandTaskProgress) error {
		state.TraceTaskID = 7001
		state.TraceDailyTaskID = 9999
		state.ActiveTasks = []orm.IslandTaskEntry{{TaskID: 7001, Timestamp: 1}}
		return nil
	})
	if seedErr != nil {
		t.Fatalf("seed island state: %v", seedErr)
	}

	payload := protobuf.CS_21034{TaskId: proto.Uint32(7999)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := IslandSetTraceTask(&buffer, client); err != nil {
		t.Fatalf("set trace task returned unexpected error: %v", err)
	}

	var response protobuf.SC_21035
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected failure result for unknown task")
	}

	state, err := orm.LoadIslandTaskProgress(client.Commander.CommanderID, nowUTC())
	if err != nil {
		t.Fatalf("load island state: %v", err)
	}
	if state.TraceTaskID != 7001 {
		t.Fatalf("expected trace task id unchanged, got %d", state.TraceTaskID)
	}
	if state.TraceDailyTaskID != 9999 {
		t.Fatalf("expected trace daily task id unchanged, got %d", state.TraceDailyTaskID)
	}
}

func TestIslandSetTraceTaskMalformedPayload(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.IslandTaskProgress{})

	buffer := []byte{}
	if _, _, err := IslandSetTraceTask(&buffer, client); err == nil {
		t.Fatalf("expected decode error for malformed payload")
	}
}
