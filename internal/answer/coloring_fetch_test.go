package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func setupColoringTestClient(t *testing.T) *connection.Client {
	t.Helper()
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.CommanderColoringState{})
	seedConfigEntry(t, "ShareCfg/activity_template.json", "4890", `{"id":4890,"type":43,"config_data":[[92,2,20001,1],[93,2,20002,1]],"time":"always"}`)
	seedConfigEntry(t, "ShareCfg/activity_coloring_template.json", "92", `{"id":92,"blank":0,"color_id_list":[3001,3002],"cells":[[1,1,1],[1,2,2]]}`)
	seedConfigEntry(t, "ShareCfg/activity_coloring_template.json", "93", `{"id":93,"blank":1,"color_id_list":[3001,3002],"cells":[[1,1,1],[1,2,2]]}`)
	if err := client.Commander.SetItem(3001, 10); err != nil {
		t.Fatalf("seed paint item 3001: %v", err)
	}
	if err := client.Commander.SetItem(3002, 8); err != nil {
		t.Fatalf("seed paint item 3002: %v", err)
	}
	return client
}

func TestColoringFetchSeedsSnapshotForFirstTimeState(t *testing.T) {
	client := setupColoringTestClient(t)
	payload := marshalPacketRequest(t, &protobuf.CS_26008{ActId: proto.Uint32(4890)})
	if _, _, err := ColoringFetch(&payload, client); err != nil {
		t.Fatalf("ColoringFetch failed: %v", err)
	}
	response := &protobuf.SC_26001{}
	decodeLoveLetterPacketMessage(t, client, 26001, response)
	if response.GetId() != 92 {
		t.Fatalf("expected current page 92, got %d", response.GetId())
	}
	if response.GetStartTime() == 0 {
		t.Fatalf("expected start time to be seeded")
	}
	if getColorItemCount(response.GetColorList(), 3001) != 10 || getColorItemCount(response.GetColorList(), 3002) != 8 {
		t.Fatalf("unexpected color counts: %+v", response.GetColorList())
	}
	if len(response.GetAwardList()) != 0 {
		t.Fatalf("expected empty award list for first fetch")
	}
}

func TestColoringFetchReturnsPersistedState(t *testing.T) {
	client := setupColoringTestClient(t)
	state, err := orm.GetOrCreateCommanderColoringState(client.Commander.CommanderID, 4890, 1700000001)
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	state.Cells = []orm.ColoringCellState{{PageID: 92, Row: 1, Column: 1, Color: 1}}
	state.Awards = []orm.ColoringAwardState{}
	if err := orm.SaveCommanderColoringState(state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	payload := marshalPacketRequest(t, &protobuf.CS_26008{ActId: proto.Uint32(4890)})
	if _, _, err := ColoringFetch(&payload, client); err != nil {
		t.Fatalf("ColoringFetch failed: %v", err)
	}
	response := &protobuf.SC_26001{}
	decodeLoveLetterPacketMessage(t, client, 26001, response)
	if len(response.GetCellList()) != 1 || response.GetCellList()[0].GetRow() != 1 || response.GetCellList()[0].GetColumn() != 1 {
		t.Fatalf("unexpected cell list: %+v", response.GetCellList())
	}
	if len(response.GetAwardList()) != 0 {
		t.Fatalf("unexpected award list: %+v", response.GetAwardList())
	}
}

func TestColoringFetchInvalidActivityReturnsError(t *testing.T) {
	client := setupColoringTestClient(t)
	payload := marshalPacketRequest(t, &protobuf.CS_26008{ActId: proto.Uint32(9999)})
	if _, _, err := ColoringFetch(&payload, client); err == nil {
		t.Fatalf("expected invalid activity fetch to return error")
	}
}

func getColorItemCount(list []*protobuf.COLORINFO, itemID uint32) uint32 {
	for _, entry := range list {
		if entry.GetId() == itemID {
			return entry.GetNumber()
		}
	}
	return 0
}
