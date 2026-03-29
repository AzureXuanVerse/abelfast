package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestChapterTrackingSuccess(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.OwnedResource{})
	clearTable(t, &orm.ChapterState{})
	clearTable(t, &orm.ChapterProgress{})
	seedChapterTrackingConfig(t)
	if err := prepareChapterTrackingClient(t, client); err != nil {
		t.Fatalf("prepare chapter tracking client: %v", err)
	}

	execAnswerTestSQLT(t, "INSERT INTO commander_items (commander_id, item_id, count) VALUES ($1, $2, $3)", int64(client.Commander.CommanderID), int64(20001), int64(1))
	client.Commander.CommanderItemsMap[20001] = &orm.CommanderItem{CommanderID: client.Commander.CommanderID, ItemID: 20001, Count: 1}

	payload := protobuf.CS_13101{
		Id: proto.Uint32(101),
		Fleet: &protobuf.FLEET_INFO{
			Id: proto.Uint32(1),
			MainTeam: []*protobuf.TEAM_INFO{
				{Id: proto.Uint32(1), ShipList: []uint32{101}},
			},
		},
		OperationItem: proto.Uint32(20001),
	}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := ChapterTracking(&buffer, client); err != nil {
		t.Fatalf("chapter tracking failed: %v", err)
	}

	var response protobuf.SC_13102
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if response.GetCurrentChapter().GetId() != 101 {
		t.Fatalf("expected chapter id 101, got %d", response.GetCurrentChapter().GetId())
	}
	if len(response.GetCurrentChapter().GetMainGroupList()) != 1 {
		t.Fatalf("expected 1 main group, got %d", len(response.GetCurrentChapter().GetMainGroupList()))
	}
	if len(response.GetCurrentChapter().GetOperationBuff()) != 1 || response.GetCurrentChapter().GetOperationBuff()[0] != 2 {
		t.Fatalf("expected operation buff 2")
	}
	for _, cell := range response.GetCurrentChapter().GetCellList() {
		if cell.GetItemType() == 6 && cell.GetItemId() == 0 {
			t.Fatalf("expected enemy cell to include item_id")
		}
	}

	if _, err := orm.GetChapterState(client.Commander.CommanderID); err != nil {
		t.Fatalf("chapter state missing: %v", err)
	}
	if _, err := orm.GetChapterProgress(client.Commander.CommanderID, 101); err != nil {
		t.Fatalf("chapter progress missing: %v", err)
	}
	oil := queryAnswerTestInt64(t, "SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = $2", int64(client.Commander.CommanderID), int64(2))
	if oil != 88 {
		t.Fatalf("expected oil 88, got %d", oil)
	}
	item := queryAnswerTestInt64(t, "SELECT count FROM commander_items WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(20001))
	if item != 0 {
		t.Fatalf("expected item count 0, got %d", item)
	}
}

func TestChapterTrackingInvalidChapter(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.OwnedResource{})
	clearTable(t, &orm.ChapterState{})
	clearTable(t, &orm.ChapterProgress{})
	if err := prepareChapterTrackingClient(t, client); err != nil {
		t.Fatalf("prepare chapter tracking client: %v", err)
	}

	payload := protobuf.CS_13101{
		Id: proto.Uint32(999),
		Fleet: &protobuf.FLEET_INFO{
			Id: proto.Uint32(1),
			MainTeam: []*protobuf.TEAM_INFO{
				{Id: proto.Uint32(1), ShipList: []uint32{101}},
			},
		},
	}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := ChapterTracking(&buffer, client); err != nil {
		t.Fatalf("chapter tracking failed: %v", err)
	}
	var response protobuf.SC_13102
	decodeResponse(t, client, &response)
	if response.GetResult() != 1 {
		t.Fatalf("expected result 1, got %d", response.GetResult())
	}
	if _, err := orm.GetChapterState(client.Commander.CommanderID); err == nil {
		t.Fatalf("expected no chapter state")
	}
}

func TestChapterTrackingIncludesBoxItemIDForAttachBox(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.OwnedResource{})
	clearTable(t, &orm.ChapterState{})
	clearTable(t, &orm.ChapterProgress{})
	clearTable(t, &orm.ConfigEntry{})
	if err := prepareChapterTrackingClient(t, client); err != nil {
		t.Fatalf("prepare chapter tracking client: %v", err)
	}

	seedConfigEntry(t, "sharecfgdata/chapter_template.json", "102", `{"id":102,"grids":[[4,2,true,1],[6,5,true,2],[4,5,true,4],[4,4,true,6],[4,6,true,8]],"box_list":[[6,5,[1,101,1004,5001]]],"random_box_list":[1,21,101,1004],"ammo_total":5,"ammo_submarine":0,"group_num":1,"submarine_num":0,"support_group_num":0,"chapter_strategy":[],"boss_expedition_id":[9002],"expedition_id_weight_list":[[102010,160,0]],"elite_expedition_list":[102210],"ambush_expedition_list":[102220],"guarder_expedition_list":[102100],"star_require_1":1,"num_1":1,"star_require_2":2,"num_2":1,"star_require_3":4,"num_3":3,"progress_boss":100,"oil":0,"time":100}`)

	payload := protobuf.CS_13101{
		Id: proto.Uint32(102),
		Fleet: &protobuf.FLEET_INFO{
			Id: proto.Uint32(1),
			MainTeam: []*protobuf.TEAM_INFO{
				{Id: proto.Uint32(1), ShipList: []uint32{101}},
			},
		},
	}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := ChapterTracking(&buffer, client); err != nil {
		t.Fatalf("chapter tracking failed: %v", err)
	}

	var response protobuf.SC_13102
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}

	var boxCell *protobuf.CHAPTERCELLINFO_P13
	for _, cell := range response.GetCurrentChapter().GetCellList() {
		if cell.GetItemType() == 2 {
			boxCell = cell
			break
		}
	}
	if boxCell == nil {
		t.Fatalf("expected chapter to contain a box cell")
	}
	if boxCell.GetItemId() == 0 {
		t.Fatalf("expected box cell to include item_id")
	}
	if boxCell.GetItemId() != 1 {
		t.Fatalf("expected box item_id 1 from box_list, got %d", boxCell.GetItemId())
	}

	state, err := orm.GetChapterState(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("chapter state missing: %v", err)
	}
	var current protobuf.CURRENTCHAPTERINFO
	if err := proto.Unmarshal(state.State, &current); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	for _, cell := range current.GetCellList() {
		if cell.GetItemType() == 2 && cell.GetItemId() != 1 {
			t.Fatalf("expected persisted box item_id 1, got %d", cell.GetItemId())
		}
	}
}

func TestChapterTrackingIncludesLandbaseItemIDForAttachLandbase(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.OwnedResource{})
	clearTable(t, &orm.ChapterState{})
	clearTable(t, &orm.ChapterProgress{})
	clearTable(t, &orm.ConfigEntry{})
	if err := prepareChapterTrackingClient(t, client); err != nil {
		t.Fatalf("prepare chapter tracking client: %v", err)
	}

	seedConfigEntry(t, "sharecfgdata/chapter_template.json", "1603", `{"id":1603,"grids":[[4,2,true,1],[5,10,false,100],[1,3,false,100],[4,6,true,8]],"land_based":[[5,10,101],[1,3,103]],"ammo_total":5,"ammo_submarine":2,"group_num":1,"submarine_num":1,"support_group_num":0,"chapter_strategy":[],"boss_expedition_id":[9603],"expedition_id_weight_list":[],"elite_expedition_list":[],"ambush_expedition_list":[],"guarder_expedition_list":[],"star_require_1":1,"num_1":1,"star_require_2":2,"num_2":1,"star_require_3":4,"num_3":3,"progress_boss":100,"oil":0,"time":100}`)

	payload := protobuf.CS_13101{
		Id: proto.Uint32(1603),
		Fleet: &protobuf.FLEET_INFO{
			Id:       proto.Uint32(1),
			MainTeam: []*protobuf.TEAM_INFO{{Id: proto.Uint32(1), ShipList: []uint32{101}}},
		},
	}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := ChapterTracking(&buffer, client); err != nil {
		t.Fatalf("chapter tracking failed: %v", err)
	}

	var response protobuf.SC_13102
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}

	landbaseIDs := map[[2]uint32]uint32{}
	for _, cell := range response.GetCurrentChapter().GetCellList() {
		if cell.GetItemType() == 100 {
			landbaseIDs[[2]uint32{cell.GetPos().GetRow(), cell.GetPos().GetColumn()}] = cell.GetItemId()
		}
	}
	if len(landbaseIDs) != 2 {
		t.Fatalf("expected 2 landbase cells, got %d", len(landbaseIDs))
	}
	if landbaseIDs[[2]uint32{5, 10}] != 101 {
		t.Fatalf("expected landbase [5,10] item_id 101, got %d", landbaseIDs[[2]uint32{5, 10}])
	}
	if landbaseIDs[[2]uint32{1, 3}] != 103 {
		t.Fatalf("expected landbase [1,3] item_id 103, got %d", landbaseIDs[[2]uint32{1, 3}])
	}
}

func TestChapterTrackingIncludesSupplyAmountForAttachSupply(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.OwnedResource{})
	clearTable(t, &orm.ChapterState{})
	clearTable(t, &orm.ChapterProgress{})
	clearTable(t, &orm.ConfigEntry{})
	if err := prepareChapterTrackingClient(t, client); err != nil {
		t.Fatalf("prepare chapter tracking client: %v", err)
	}

	seedConfigEntry(t, "sharecfgdata/chapter_template.json", "104", `{"id":104,"grids":[[4,2,true,1],[5,5,true,3],[4,4,true,6],[4,6,true,8]],"ammo_total":5,"ammo_submarine":0,"group_num":1,"submarine_num":0,"support_group_num":0,"chapter_strategy":[],"boss_expedition_id":[9004],"expedition_id_weight_list":[[104010,160,0]],"elite_expedition_list":[],"ambush_expedition_list":[],"guarder_expedition_list":[],"star_require_1":1,"num_1":1,"star_require_2":2,"num_2":1,"star_require_3":4,"num_3":3,"progress_boss":100,"oil":0,"time":100}`)

	payload := protobuf.CS_13101{Id: proto.Uint32(104), Fleet: &protobuf.FLEET_INFO{Id: proto.Uint32(1), MainTeam: []*protobuf.TEAM_INFO{{Id: proto.Uint32(1), ShipList: []uint32{101}}}}}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := ChapterTracking(&buffer, client); err != nil {
		t.Fatalf("chapter tracking failed: %v", err)
	}

	var response protobuf.SC_13102
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}

	for _, cell := range response.GetCurrentChapter().GetCellList() {
		if cell.GetItemType() == 3 {
			if cell.GetItemId() != 5 {
				t.Fatalf("expected supply item_id 5, got %d", cell.GetItemId())
			}
			return
		}
	}
	t.Fatalf("expected supply cell in chapter state")
}

func TestChapterTrackingIncludesEscortListForTransportChapter(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.OwnedResource{})
	clearTable(t, &orm.ChapterState{})
	clearTable(t, &orm.ChapterProgress{})
	clearTable(t, &orm.ConfigEntry{})
	if err := prepareChapterTrackingClient(t, client); err != nil {
		t.Fatalf("prepare chapter tracking client: %v", err)
	}

	seedConfigEntry(t, "sharecfgdata/chapter_template.json", "20001", `{"id":20001,"grids":[[4,2,true,1],[7,1,true,17],[3,7,true,18],[4,4,true,6],[4,6,true,8]],"friendly_id":1,"ammo_total":5,"ammo_submarine":0,"group_num":1,"submarine_num":0,"support_group_num":0,"chapter_strategy":[],"boss_expedition_id":[9200],"expedition_id_weight_list":[[200010,160,0]],"elite_expedition_list":[200210],"ambush_expedition_list":[],"guarder_expedition_list":[],"star_require_1":1,"num_1":1,"star_require_2":2,"num_2":1,"star_require_3":4,"num_3":3,"progress_boss":100,"oil":0,"time":100}`)
	seedConfigEntry(t, "ShareCfg/friendly_data_template.json", "1", `{"id":1,"hp":20}`)

	payload := protobuf.CS_13101{Id: proto.Uint32(20001), Fleet: &protobuf.FLEET_INFO{Id: proto.Uint32(1), MainTeam: []*protobuf.TEAM_INFO{{Id: proto.Uint32(1), ShipList: []uint32{101}}}}}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if _, _, err := ChapterTracking(&buffer, client); err != nil {
		t.Fatalf("chapter tracking failed: %v", err)
	}

	var response protobuf.SC_13102
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if len(response.GetCurrentChapter().GetEscortList()) != 1 {
		t.Fatalf("expected 1 escort entry, got %d", len(response.GetCurrentChapter().GetEscortList()))
	}
	escort := response.GetCurrentChapter().GetEscortList()[0]
	if escort.GetItemType() != 17 || escort.GetItemId() != 1 || escort.GetItemData() != 20 {
		t.Fatalf("unexpected escort cell: type=%d id=%d data=%d", escort.GetItemType(), escort.GetItemId(), escort.GetItemData())
	}
	for _, cell := range response.GetCurrentChapter().GetCellList() {
		if cell.GetItemType() == 18 {
			return
		}
	}
	t.Fatalf("expected transport target cell in chapter state")
}

func seedChapterTrackingConfig(t *testing.T) {
	seedConfigEntry(t, "sharecfgdata/chapter_template.json", "101", `{"id":101,"grids":[[1,1,true,1],[1,2,true,6],[1,3,true,8]],"ammo_total":5,"ammo_submarine":2,"group_num":1,"submarine_num":0,"support_group_num":0,"chapter_strategy":[1016],"boss_expedition_id":[9001],"expedition_id_weight_list":[[101010,160,0]],"elite_expedition_list":[101210],"ambush_expedition_list":[101220],"guarder_expedition_list":[101100],"star_require_1":1,"num_1":1,"star_require_2":2,"num_2":1,"star_require_3":4,"num_3":3,"progress_boss":100,"oil":10,"time":100}`)
	seedConfigEntry(t, "sharecfgdata/item_data_statistics.json", "20001", `{"id":20001,"usage_arg":[1]}`)
	seedConfigEntry(t, "ShareCfg/benefit_buff_template.json", "1", `{"id":1,"benefit_type":"more_oil","benefit_effect":"20","benefit_condition":"0"}`)
	seedConfigEntry(t, "ShareCfg/benefit_buff_template.json", "2", `{"id":2,"benefit_type":"desc","benefit_effect":"0","benefit_condition":"20001"}`)
}
