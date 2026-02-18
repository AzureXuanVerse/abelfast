package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func seedFinishedColoringPage(t *testing.T, commanderID uint32, actID uint32, pageID uint32) {
	t.Helper()
	state, err := orm.GetOrCreateCommanderColoringState(commanderID, actID, 1700000001)
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	state.Cells = []orm.ColoringCellState{{PageID: pageID, Row: 1, Column: 1, Color: 1}, {PageID: pageID, Row: 1, Column: 2, Color: 2}}
	if err := orm.SaveCommanderColoringState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}
}

func TestColoringAchieveSuccessAndDuplicateFailure(t *testing.T) {
	client := setupColoringTestClient(t)
	seedFinishedColoringPage(t, client.Commander.CommanderID, 4890, 92)

	payload := marshalPacketRequest(t, &protobuf.CS_26002{ActId: proto.Uint32(4890), Id: proto.Uint32(92)})
	if _, _, err := ColoringAchieve(&payload, client); err != nil {
		t.Fatalf("ColoringAchieve failed: %v", err)
	}
	response := &protobuf.SC_26003{}
	decodeLoveLetterPacketMessage(t, client, 26003, response)
	if response.GetResult() != 0 || len(response.GetDropList()) == 0 {
		t.Fatalf("expected successful claim with drops, got result=%d drops=%+v", response.GetResult(), response.GetDropList())
	}
	state, err := orm.GetCommanderColoringState(client.Commander.CommanderID, 4890)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if !coloringIsPageClaimed(state, 92) {
		t.Fatalf("expected successful claim marker to persist")
	}

	payload = marshalPacketRequest(t, &protobuf.CS_26002{ActId: proto.Uint32(4890), Id: proto.Uint32(92)})
	if _, _, err := ColoringAchieve(&payload, client); err != nil {
		t.Fatalf("ColoringAchieve duplicate failed: %v", err)
	}
	response = &protobuf.SC_26003{}
	decodeLoveLetterPacketMessage(t, client, 26003, response)
	if response.GetResult() == 0 {
		t.Fatalf("expected duplicate claim failure")
	}
}

func TestColoringAchieveRejectsUnfinishedOrWrongActivity(t *testing.T) {
	client := setupColoringTestClient(t)
	unfinished := marshalPacketRequest(t, &protobuf.CS_26002{ActId: proto.Uint32(4890), Id: proto.Uint32(92)})
	if _, _, err := ColoringAchieve(&unfinished, client); err != nil {
		t.Fatalf("unfinished claim returned error: %v", err)
	}
	response := &protobuf.SC_26003{}
	decodeLoveLetterPacketMessage(t, client, 26003, response)
	if response.GetResult() == 0 {
		t.Fatalf("expected unfinished claim failure")
	}

	seedConfigEntry(t, "ShareCfg/activity_template.json", "5000", `{"id":5000,"type":88,"config_data":[[92,2,20001,1]],"time":"always"}`)
	wrongType := marshalPacketRequest(t, &protobuf.CS_26002{ActId: proto.Uint32(5000), Id: proto.Uint32(92)})
	if _, _, err := ColoringAchieve(&wrongType, client); err != nil {
		t.Fatalf("wrong activity type returned error: %v", err)
	}
	response = &protobuf.SC_26003{}
	decodeLoveLetterPacketMessage(t, client, 26003, response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-coloring activity failure")
	}
}

func TestColoringAchieveAllowsBlankPageClaim(t *testing.T) {
	client := setupColoringTestClient(t)
	payload := marshalPacketRequest(t, &protobuf.CS_26002{ActId: proto.Uint32(4890), Id: proto.Uint32(93)})
	if _, _, err := ColoringAchieve(&payload, client); err != nil {
		t.Fatalf("ColoringAchieve blank page failed: %v", err)
	}
	response := &protobuf.SC_26003{}
	decodeLoveLetterPacketMessage(t, client, 26003, response)
	if response.GetResult() != 0 {
		t.Fatalf("expected blank page claim success")
	}
	state, err := orm.GetCommanderColoringState(client.Commander.CommanderID, 4890)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if !coloringIsPageClaimed(state, 93) {
		t.Fatalf("expected blank page claim marker to persist")
	}
}

func TestColoringAchieveRollsBackOnDropFailure(t *testing.T) {
	client := setupColoringTestClient(t)
	seedConfigEntry(t, "ShareCfg/activity_template.json", "4890", `{"id":4890,"type":43,"config_data":[[92,999,1,1]],"time":"always"}`)
	seedFinishedColoringPage(t, client.Commander.CommanderID, 4890, 92)

	payload := marshalPacketRequest(t, &protobuf.CS_26002{ActId: proto.Uint32(4890), Id: proto.Uint32(92)})
	if _, _, err := ColoringAchieve(&payload, client); err != nil {
		t.Fatalf("ColoringAchieve drop failure returned error: %v", err)
	}
	response := &protobuf.SC_26003{}
	decodeLoveLetterPacketMessage(t, client, 26003, response)
	if response.GetResult() == 0 {
		t.Fatalf("expected claim failure for unsupported drop type")
	}
	state, err := orm.GetCommanderColoringState(client.Commander.CommanderID, 4890)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if coloringIsPageClaimed(state, 92) {
		t.Fatalf("expected award marker rollback on failure")
	}
}
