package answer

import (
	"fmt"
	"testing"

	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func seedTaskTemplate(t *testing.T, taskID uint32, targetNum uint32, quickFinish uint32, awardDisplay string) {
	t.Helper()
	payload := fmt.Sprintf(`{"id":%d,"target_num":%d,"quick_finish":%d,"award_display":%s}`, taskID, targetNum, quickFinish, awardDisplay)
	seedConfigEntry(t, "ShareCfg/task_data_template.json", fmt.Sprintf("%d", taskID), payload)
}

func TestUpdateTaskProgressSupportsUpdateAndAppend(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	seedTaskTemplate(t, 9001, 10, 0, `[[1,1,10]]`)

	request := &protobuf.CS_20009{Progressinfo: []*protobuf.TASK_UPDATE{
		{Id: proto.Uint32(9001), Mode: proto.Uint32(0), Progress: proto.Uint32(5)},
		{Id: proto.Uint32(9001), Mode: proto.Uint32(1), Progress: proto.Uint32(3)},
	}}
	buf, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, _, err := UpdateTaskProgress(&buf, client); err != nil {
		t.Fatalf("UpdateTaskProgress: %v", err)
	}

	var response protobuf.SC_20010
	decodePacketAt(t, client, 0, 20010, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result=0, got %d", response.GetResult())
	}
	progress := queryAnswerTestInt64(t, "SELECT progress FROM commander_tasks WHERE commander_id = $1 AND task_id = $2", int64(client.Commander.CommanderID), int64(9001))
	if progress != 8 {
		t.Fatalf("expected progress=8, got %d", progress)
	}
}

func TestUpdateTaskProgressRejectsInvalidMode(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	seedTaskTemplate(t, 9001, 10, 0, `[[1,1,10]]`)

	request := &protobuf.CS_20009{Progressinfo: []*protobuf.TASK_UPDATE{{Id: proto.Uint32(9001), Mode: proto.Uint32(9), Progress: proto.Uint32(1)}}}
	buf, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, _, err := UpdateTaskProgress(&buf, client); err != nil {
		t.Fatalf("UpdateTaskProgress: %v", err)
	}

	var response protobuf.SC_20010
	decodePacketAt(t, client, 0, 20010, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result for invalid mode")
	}
	count := queryAnswerTestInt64(t, "SELECT COUNT(*) FROM commander_tasks WHERE commander_id = $1 AND task_id = $2", int64(client.Commander.CommanderID), int64(9001))
	if count != 0 {
		t.Fatalf("expected no persisted task row")
	}
}

func TestTaskProgressEventValidPayload(t *testing.T) {
	client := setupPlayerUpdateTest(t)

	request := &protobuf.CS_20016{EventType: proto.Uint32(1), EventTarget: proto.Uint32(9101), EventCount: proto.Uint32(2)}
	buf, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, _, err := TaskProgressEvent(&buf, client); err != nil {
		t.Fatalf("TaskProgressEvent: %v", err)
	}

	var response protobuf.SC_20017
	decodePacketAt(t, client, 0, 20017, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result=0, got %d", response.GetResult())
	}
}

func TestTaskProgressEventInvalidPayload(t *testing.T) {
	client := setupPlayerUpdateTest(t)

	request := &protobuf.CS_20016{EventType: proto.Uint32(1), EventTarget: proto.Uint32(0), EventCount: proto.Uint32(0)}
	buf, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, _, err := TaskProgressEvent(&buf, client); err != nil {
		t.Fatalf("TaskProgressEvent: %v", err)
	}

	var response protobuf.SC_20017
	decodePacketAt(t, client, 0, 20017, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result for invalid payload")
	}
}

func TestSubmitTaskSuccess(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	seedTaskTemplate(t, 9201, 5, 0, `[[1,1,30],[2,40001,2]]`)
	execAnswerTestSQLT(t, "INSERT INTO commander_tasks (commander_id, task_id, progress, accept_time, submit_time) VALUES ($1, $2, $3, $4, 0)", int64(client.Commander.CommanderID), int64(9201), int64(5), int64(1))

	request := &protobuf.CS_20005{Id: proto.Uint32(9201)}
	buf, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, _, err := SubmitTask(&buf, client); err != nil {
		t.Fatalf("SubmitTask: %v", err)
	}

	var response protobuf.SC_20006
	decodePacketAt(t, client, 0, 20006, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result=0, got %d", response.GetResult())
	}
	if len(response.GetAwardList()) != 2 {
		t.Fatalf("expected 2 drops, got %d", len(response.GetAwardList()))
	}
	submitTime := queryAnswerTestInt64(t, "SELECT submit_time FROM commander_tasks WHERE commander_id = $1 AND task_id = $2", int64(client.Commander.CommanderID), int64(9201))
	if submitTime == 0 {
		t.Fatalf("expected submit_time to be persisted")
	}
}

func TestSubmitTaskBatchPartialSuccess(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	seedTaskTemplate(t, 9301, 1, 0, `[[1,1,10]]`)
	seedTaskTemplate(t, 9302, 2, 0, `[[1,1,20]]`)
	execAnswerTestSQLT(t, "INSERT INTO commander_tasks (commander_id, task_id, progress, accept_time, submit_time) VALUES ($1, $2, $3, $4, 0)", int64(client.Commander.CommanderID), int64(9301), int64(1), int64(1))
	execAnswerTestSQLT(t, "INSERT INTO commander_tasks (commander_id, task_id, progress, accept_time, submit_time) VALUES ($1, $2, $3, $4, 0)", int64(client.Commander.CommanderID), int64(9302), int64(1), int64(1))

	request := &protobuf.CS_20011{IdList: []uint32{9301, 9302, 9301, 9999}}
	buf, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, _, err := SubmitTaskBatch(&buf, client); err != nil {
		t.Fatalf("SubmitTaskBatch: %v", err)
	}

	var response protobuf.SC_20012
	decodePacketAt(t, client, 0, 20012, &response)
	if len(response.GetIdList()) != 1 || response.GetIdList()[0] != 9301 {
		t.Fatalf("expected only task 9301 submitted, got %v", response.GetIdList())
	}
}

func TestSubmitQuickTaskConsumesTicket(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	seedTaskTemplate(t, 9401, 2, 3, `[[2,40002,1]]`)
	if err := client.Commander.SetItem(quickTaskPassTicketID, 3); err != nil {
		t.Fatalf("seed quick tickets: %v", err)
	}

	request := &protobuf.CS_20013{Id: proto.Uint32(9401), ItemCost: proto.Uint32(3)}
	buf, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, _, err := SubmitQuickTask(&buf, client); err != nil {
		t.Fatalf("SubmitQuickTask: %v", err)
	}

	var response protobuf.SC_20014
	decodePacketAt(t, client, 0, 20014, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result=0, got %d", response.GetResult())
	}
	if client.Commander.GetItemCount(quickTaskPassTicketID) != 0 {
		t.Fatalf("expected quick task ticket count to be consumed")
	}
	submitTime := queryAnswerTestInt64(t, "SELECT submit_time FROM commander_tasks WHERE commander_id = $1 AND task_id = $2", int64(client.Commander.CommanderID), int64(9401))
	if submitTime == 0 {
		t.Fatalf("expected submit_time to be persisted")
	}
}

func TestSubmitQuickTaskDoesNotConsumeTicketWhenAlreadySubmitted(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	seedTaskTemplate(t, 9402, 2, 3, `[[2,40002,1]]`)
	execAnswerTestSQLT(t, "INSERT INTO commander_tasks (commander_id, task_id, progress, accept_time, submit_time) VALUES ($1, $2, $3, $4, $5)", int64(client.Commander.CommanderID), int64(9402), int64(2), int64(1), int64(1))
	if err := client.Commander.SetItem(quickTaskPassTicketID, 3); err != nil {
		t.Fatalf("seed quick tickets: %v", err)
	}

	request := &protobuf.CS_20013{Id: proto.Uint32(9402), ItemCost: proto.Uint32(3)}
	buf, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, _, err := SubmitQuickTask(&buf, client); err != nil {
		t.Fatalf("SubmitQuickTask: %v", err)
	}

	var response protobuf.SC_20014
	decodePacketAt(t, client, 0, 20014, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result for already submitted task")
	}
	if got := client.Commander.GetItemCount(quickTaskPassTicketID); got != 3 {
		t.Fatalf("expected quick task tickets unchanged, got %d", got)
	}
}
