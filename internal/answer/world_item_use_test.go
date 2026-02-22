package answer

import (
	"fmt"
	"testing"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

const (
	worldItemTemplateCategory = "ShareCfg/world_item_data_template.json"
	dropRestoreCategory       = "ShareCfg/drop_data_restore.json"
)

func TestWorldItemUseConsumesRecoverAPItem(t *testing.T) {
	client := setupWorldItemUseTestClient(t)
	seedWorldItemConfig(t, 251, `{"id":251,"usage":"usage_world_recoverAP","usage_arg":[20]}`)
	if err := client.Commander.AddItem(251, 3); err != nil {
		t.Fatalf("seed item: %v", err)
	}

	payload := marshalPacketRequest(t, &protobuf.CS_33301{Id: proto.Uint32(251), Count: proto.Uint32(2), Arg: []uint32{}})
	if _, _, err := WorldItemUse(&payload, client); err != nil {
		t.Fatalf("world item use failed: %v", err)
	}

	response := &protobuf.SC_33302{}
	decodeLoveLetterPacketMessage(t, client, 33302, response)
	if response.GetResult() != worldItemUseResultSuccess {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}
	if len(response.GetDropList()) != 0 {
		t.Fatalf("expected empty drops for recover AP, got %+v", response.GetDropList())
	}
	remaining := queryAnswerTestInt64(t, "SELECT count FROM commander_items WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(251))
	if remaining != 1 {
		t.Fatalf("expected remaining item count 1, got %d", remaining)
	}
	if client.Commander.GetItemCount(251) != 1 {
		t.Fatalf("expected in-memory item count to be refreshed")
	}
	runtime, err := orm.LoadWorldRuntime(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("load world runtime: %v", err)
	}
	if runtime.ActionPower != 240 {
		t.Fatalf("expected action power to increase from recover AP item, got %d", runtime.ActionPower)
	}
}

func TestWorldItemUseFailurePaths(t *testing.T) {
	t.Run("insufficient count", func(t *testing.T) {
		client := setupWorldItemUseTestClient(t)
		seedWorldItemConfig(t, 251, `{"id":251,"usage":"usage_world_recoverAP","usage_arg":[20]}`)
		if err := client.Commander.AddItem(251, 1); err != nil {
			t.Fatalf("seed item: %v", err)
		}
		payload := marshalPacketRequest(t, &protobuf.CS_33301{Id: proto.Uint32(251), Count: proto.Uint32(2), Arg: []uint32{}})
		if _, _, err := WorldItemUse(&payload, client); err != nil {
			t.Fatalf("world item use failed: %v", err)
		}
		response := &protobuf.SC_33302{}
		decodeLoveLetterPacketMessage(t, client, 33302, response)
		if response.GetResult() != worldItemUseResultFailure {
			t.Fatalf("expected failure result, got %d", response.GetResult())
		}
		remaining := queryAnswerTestInt64(t, "SELECT count FROM commander_items WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(251))
		if remaining != 1 {
			t.Fatalf("expected item count unchanged, got %d", remaining)
		}
	})

	t.Run("invalid item id", func(t *testing.T) {
		client := setupWorldItemUseTestClient(t)
		payload := marshalPacketRequest(t, &protobuf.CS_33301{Id: proto.Uint32(999999), Count: proto.Uint32(1), Arg: []uint32{}})
		if _, _, err := WorldItemUse(&payload, client); err != nil {
			t.Fatalf("world item use failed: %v", err)
		}
		response := &protobuf.SC_33302{}
		decodeLoveLetterPacketMessage(t, client, 33302, response)
		if response.GetResult() != worldItemUseResultFailure {
			t.Fatalf("expected failure result, got %d", response.GetResult())
		}
	})

	t.Run("healing requires targets", func(t *testing.T) {
		client := setupWorldItemUseTestClient(t)
		seedWorldItemConfig(t, 205, `{"id":205,"usage":"usage_world_healing","usage_arg":[6,3000]}`)
		if err := client.Commander.AddItem(205, 1); err != nil {
			t.Fatalf("seed item: %v", err)
		}
		payload := marshalPacketRequest(t, &protobuf.CS_33301{Id: proto.Uint32(205), Count: proto.Uint32(1), Arg: []uint32{}})
		if _, _, err := WorldItemUse(&payload, client); err != nil {
			t.Fatalf("world item use failed: %v", err)
		}
		response := &protobuf.SC_33302{}
		decodeLoveLetterPacketMessage(t, client, 33302, response)
		if response.GetResult() != worldItemUseResultFailure {
			t.Fatalf("expected failure result, got %d", response.GetResult())
		}
		remaining := queryAnswerTestInt64(t, "SELECT count FROM commander_items WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(205))
		if remaining != 1 {
			t.Fatalf("expected item count unchanged, got %d", remaining)
		}
	})

	t.Run("recover ap runtime failure keeps item", func(t *testing.T) {
		client := setupWorldItemUseTestClient(t)
		seedWorldItemConfig(t, 251, `{"id":251,"usage":"usage_world_recoverAP","usage_arg":[20]}`)
		if err := client.Commander.AddItem(251, 1); err != nil {
			t.Fatalf("seed item: %v", err)
		}
		if err := orm.UpsertConfigEntry("ShareCfg/gameset.json", "world_movepower_maxvalue", []byte(`{"key_value":"bad"}`)); err != nil {
			t.Fatalf("seed invalid gameset: %v", err)
		}

		payload := marshalPacketRequest(t, &protobuf.CS_33301{Id: proto.Uint32(251), Count: proto.Uint32(1), Arg: []uint32{}})
		if _, _, err := WorldItemUse(&payload, client); err != nil {
			t.Fatalf("world item use failed: %v", err)
		}
		response := &protobuf.SC_33302{}
		decodeLoveLetterPacketMessage(t, client, 33302, response)
		if response.GetResult() != worldItemUseResultFailure {
			t.Fatalf("expected failure result, got %d", response.GetResult())
		}
		remaining := queryAnswerTestInt64(t, "SELECT count FROM commander_items WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(251))
		if remaining != 1 {
			t.Fatalf("expected item count unchanged, got %d", remaining)
		}
	})
}

func TestWorldItemUseDropAndAppointedFlows(t *testing.T) {
	t.Run("usage_drop returns configured drop", func(t *testing.T) {
		client := setupWorldItemUseTestClient(t)
		seedWorldItemConfig(t, 2002, `{"id":2002,"usage":"usage_drop","usage_arg":1030001}`)
		seedDropRestoreEntry(t, "9001", `{"id":9001,"drop_id":1030001,"resource_type":1,"resource_num":50,"target_type":0,"target_id":0,"type":1}`)
		if err := client.Commander.AddItem(2002, 1); err != nil {
			t.Fatalf("seed item: %v", err)
		}
		startGold := client.Commander.GetResourceCount(1)

		payload := marshalPacketRequest(t, &protobuf.CS_33301{Id: proto.Uint32(2002), Count: proto.Uint32(1), Arg: []uint32{}})
		if _, _, err := WorldItemUse(&payload, client); err != nil {
			t.Fatalf("world item use failed: %v", err)
		}
		response := &protobuf.SC_33302{}
		decodeLoveLetterPacketMessage(t, client, 33302, response)
		if response.GetResult() != worldItemUseResultSuccess {
			t.Fatalf("expected success result, got %d", response.GetResult())
		}
		if len(response.GetDropList()) != 1 || response.GetDropList()[0].GetType() != 1 || response.GetDropList()[0].GetId() != 1 || response.GetDropList()[0].GetNumber() != 50 {
			t.Fatalf("unexpected drops: %+v", response.GetDropList())
		}
		if client.Commander.GetResourceCount(1) != startGold+50 {
			t.Fatalf("expected gold increase by 50")
		}
	})

	t.Run("usage_drop_appointed validates arg", func(t *testing.T) {
		client := setupWorldItemUseTestClient(t)
		seedWorldItemConfig(t, 2120, `{"id":2120,"usage":"usage_drop_appointed","usage_arg":[[2,18117,1],[2,18119,1]]}`)
		if err := client.Commander.AddItem(2120, 2); err != nil {
			t.Fatalf("seed item: %v", err)
		}

		invalid := marshalPacketRequest(t, &protobuf.CS_33301{Id: proto.Uint32(2120), Count: proto.Uint32(1), Arg: []uint32{2, 18121, 1}})
		if _, _, err := WorldItemUse(&invalid, client); err != nil {
			t.Fatalf("world item use invalid arg failed: %v", err)
		}
		invalidResponse := &protobuf.SC_33302{}
		decodeLoveLetterPacketMessage(t, client, 33302, invalidResponse)
		if invalidResponse.GetResult() != worldItemUseResultFailure {
			t.Fatalf("expected invalid arg failure, got %d", invalidResponse.GetResult())
		}

		valid := marshalPacketRequest(t, &protobuf.CS_33301{Id: proto.Uint32(2120), Count: proto.Uint32(1), Arg: []uint32{2, 18119, 1}})
		if _, _, err := WorldItemUse(&valid, client); err != nil {
			t.Fatalf("world item use valid arg failed: %v", err)
		}
		validResponse := &protobuf.SC_33302{}
		decodeLoveLetterPacketMessage(t, client, 33302, validResponse)
		if validResponse.GetResult() != worldItemUseResultSuccess {
			t.Fatalf("expected valid arg success, got %d", validResponse.GetResult())
		}
		if len(validResponse.GetDropList()) != 1 || validResponse.GetDropList()[0].GetType() != 2 || validResponse.GetDropList()[0].GetId() != 18119 || validResponse.GetDropList()[0].GetNumber() != 1 {
			t.Fatalf("unexpected appointed drops: %+v", validResponse.GetDropList())
		}
		remaining := queryAnswerTestInt64(t, "SELECT count FROM commander_items WHERE commander_id = $1 AND item_id = $2", int64(client.Commander.CommanderID), int64(2120))
		if remaining != 1 {
			t.Fatalf("expected one appointed item remaining, got %d", remaining)
		}
	})
}

func setupWorldItemUseTestClient(t *testing.T) *connection.Client {
	t.Helper()
	client := setupPlayerUpdateTest(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderItem{})
	if err := client.Commander.Load(); err != nil {
		t.Fatalf("reload commander: %v", err)
	}
	return client
}

func seedWorldItemConfig(t *testing.T, itemID uint32, payload string) {
	t.Helper()
	if err := orm.UpsertConfigEntry(worldItemTemplateCategory, fmt.Sprintf("%d", itemID), []byte(payload)); err != nil {
		t.Fatalf("seed world item config %d: %v", itemID, err)
	}
}

func seedDropRestoreEntry(t *testing.T, key string, payload string) {
	t.Helper()
	if err := orm.UpsertConfigEntry(dropRestoreCategory, key, []byte(payload)); err != nil {
		t.Fatalf("seed drop restore %s: %v", key, err)
	}
}
