package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestIslandFollowerOpAddAndRemove(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.IslandFollower{})
	clearTable(t, &orm.IslandShip{})
	seedConfigEntry(t, islandFollowerConfigCategory, "max_follower_cnt", `{"key":"max_follower_cnt","key_value_int":4}`)

	if err := orm.UpsertIslandShip(&orm.IslandShip{CommanderID: client.Commander.CommanderID, ShipID: 101600, Level: 1, BreakLv: 1, CanFollow: true}); err != nil {
		t.Fatalf("seed island ship: %v", err)
	}

	add := protobuf.CS_21630{ShipId: proto.Uint32(101600), Type: proto.Uint32(islandFollowerOpAdd)}
	addBuffer, err := proto.Marshal(&add)
	if err != nil {
		t.Fatalf("marshal add: %v", err)
	}
	if _, _, err := IslandFollowerOp(&addBuffer, client); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	var addResp protobuf.SC_21631
	decodeResponse(t, client, &addResp)
	if addResp.GetResult() != islandFollowerResultSuccess {
		t.Fatalf("expected add success, got %d", addResp.GetResult())
	}

	followers, err := orm.ListIslandFollowers(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("list followers: %v", err)
	}
	if len(followers) != 1 || followers[0].ShipID != 101600 {
		t.Fatalf("expected follower persisted, got %#v", followers)
	}

	client.Buffer.Reset()
	remove := protobuf.CS_21630{ShipId: proto.Uint32(101600), Type: proto.Uint32(islandFollowerOpRemove)}
	removeBuffer, err := proto.Marshal(&remove)
	if err != nil {
		t.Fatalf("marshal remove: %v", err)
	}
	if _, _, err := IslandFollowerOp(&removeBuffer, client); err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	var removeResp protobuf.SC_21631
	decodeResponse(t, client, &removeResp)
	if removeResp.GetResult() != islandFollowerResultSuccess {
		t.Fatalf("expected remove success, got %d", removeResp.GetResult())
	}

	followers, err = orm.ListIslandFollowers(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("list followers: %v", err)
	}
	if len(followers) != 0 {
		t.Fatalf("expected followers removed, got %#v", followers)
	}
}

func TestIslandFollowerOpFailsWhenCapReached(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.IslandFollower{})
	clearTable(t, &orm.IslandShip{})
	seedConfigEntry(t, islandFollowerConfigCategory, "max_follower_cnt", `{"key":"max_follower_cnt","key_value_int":1}`)

	if err := orm.UpsertIslandShip(&orm.IslandShip{CommanderID: client.Commander.CommanderID, ShipID: 101600, Level: 1, BreakLv: 1, CanFollow: true}); err != nil {
		t.Fatalf("seed ship 1: %v", err)
	}
	if err := orm.UpsertIslandShip(&orm.IslandShip{CommanderID: client.Commander.CommanderID, ShipID: 1070300, Level: 1, BreakLv: 1, CanFollow: true}); err != nil {
		t.Fatalf("seed ship 2: %v", err)
	}

	first := protobuf.CS_21630{ShipId: proto.Uint32(101600), Type: proto.Uint32(islandFollowerOpAdd)}
	firstBuffer, _ := proto.Marshal(&first)
	if _, _, err := IslandFollowerOp(&firstBuffer, client); err != nil {
		t.Fatalf("first add failed: %v", err)
	}

	client.Buffer.Reset()
	second := protobuf.CS_21630{ShipId: proto.Uint32(1070300), Type: proto.Uint32(islandFollowerOpAdd)}
	secondBuffer, _ := proto.Marshal(&second)
	if _, _, err := IslandFollowerOp(&secondBuffer, client); err != nil {
		t.Fatalf("second add failed: %v", err)
	}
	var response protobuf.SC_21631
	decodeResponse(t, client, &response)
	if response.GetResult() != islandFollowerResultMaxReached {
		t.Fatalf("expected max cap result, got %d", response.GetResult())
	}
}

func TestIslandFollowerOpAssignsOrderIdxFromCurrentMax(t *testing.T) {
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.IslandFollower{})
	clearTable(t, &orm.IslandShip{})
	seedConfigEntry(t, islandFollowerConfigCategory, "max_follower_cnt", `{"key":"max_follower_cnt","key_value_int":4}`)

	for _, shipID := range []uint32{101600, 1070300, 3031900} {
		if err := orm.UpsertIslandShip(&orm.IslandShip{CommanderID: client.Commander.CommanderID, ShipID: shipID, Level: 1, BreakLv: 1, CanFollow: true}); err != nil {
			t.Fatalf("seed ship %d: %v", shipID, err)
		}
	}

	for _, shipID := range []uint32{101600, 1070300} {
		add := protobuf.CS_21630{ShipId: proto.Uint32(shipID), Type: proto.Uint32(islandFollowerOpAdd)}
		addBuffer, err := proto.Marshal(&add)
		if err != nil {
			t.Fatalf("marshal add %d: %v", shipID, err)
		}
		if _, _, err := IslandFollowerOp(&addBuffer, client); err != nil {
			t.Fatalf("add %d failed: %v", shipID, err)
		}
	}

	remove := protobuf.CS_21630{ShipId: proto.Uint32(101600), Type: proto.Uint32(islandFollowerOpRemove)}
	removeBuffer, err := proto.Marshal(&remove)
	if err != nil {
		t.Fatalf("marshal remove: %v", err)
	}
	if _, _, err := IslandFollowerOp(&removeBuffer, client); err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	addThird := protobuf.CS_21630{ShipId: proto.Uint32(3031900), Type: proto.Uint32(islandFollowerOpAdd)}
	addThirdBuffer, err := proto.Marshal(&addThird)
	if err != nil {
		t.Fatalf("marshal third add: %v", err)
	}
	if _, _, err := IslandFollowerOp(&addThirdBuffer, client); err != nil {
		t.Fatalf("third add failed: %v", err)
	}

	followers, err := orm.ListIslandFollowers(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("list followers: %v", err)
	}
	if len(followers) != 2 {
		t.Fatalf("expected 2 followers after remove+add, got %#v", followers)
	}

	orderIdxByShip := map[uint32]uint32{}
	for i := range followers {
		orderIdxByShip[followers[i].ShipID] = followers[i].OrderIdx
	}
	if orderIdxByShip[1070300] != 1 {
		t.Fatalf("expected existing follower to keep order_idx 1, got %d", orderIdxByShip[1070300])
	}
	if orderIdxByShip[3031900] != 2 {
		t.Fatalf("expected new follower to get max+1 order_idx 2, got %d", orderIdxByShip[3031900])
	}
}
