package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestIslandUpdateIllustrationPersistsAndHydratesBookCond(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandBookCond{})
	clearTable(t, &orm.IslandSnapshot{})
	clearTable(t, &orm.IslandTechnologyState{})
	clearTable(t, &orm.IslandCommanderDressState{})
	clearTable(t, &orm.IslandShopState{})

	seedConfigEntry(t, islandIllustratedGuideCategory, "rows", `[{"type":2,"unlock_id":100100}]`)

	payload := protobuf.CS_21340{Type: proto.Uint32(2), CondId: proto.Uint32(100100)}
	buffer, _ := proto.Marshal(&payload)
	if _, _, err := IslandUpdateIllustration(&buffer, client); err != nil {
		t.Fatalf("IslandUpdateIllustration failed: %v", err)
	}

	var response protobuf.SC_21341
	decodePacketAt(t, client, 0, 21341, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}

	conds, err := orm.ListIslandBookConds(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("list book conds: %v", err)
	}
	if len(conds) != 1 || conds[0].Type != 2 || conds[0].UnlockID != 100100 {
		t.Fatalf("unexpected persisted conds: %+v", conds)
	}

	client.Buffer.Reset()
	if _, _, err := IslandUpdateIllustration(&buffer, client); err != nil {
		t.Fatalf("repeated update failed unexpectedly: %v", err)
	}
	decodePacketAt(t, client, 0, 21341, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero for already unlockable condition")
	}

	invalid := protobuf.CS_21340{Type: proto.Uint32(2), CondId: proto.Uint32(999999)}
	invalidBuffer, _ := proto.Marshal(&invalid)
	client.Buffer.Reset()
	if _, _, err := IslandUpdateIllustration(&invalidBuffer, client); err != nil {
		t.Fatalf("invalid tuple failed unexpectedly: %v", err)
	}
	decodePacketAt(t, client, 0, 21341, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected invalid tuple failure")
	}

	getDataPayload := protobuf.CS_21200{IslandId: proto.Uint32(client.Commander.CommanderID)}
	getDataBuffer, _ := proto.Marshal(&getDataPayload)
	client.Buffer.Reset()
	if _, _, err := IslandGetData(&getDataBuffer, client); err != nil {
		t.Fatalf("island get data failed: %v", err)
	}
	var getDataResponse protobuf.SC_21201
	decodePacketAt(t, client, 0, 21201, &getDataResponse)

	condList := getDataResponse.GetIsland().GetPrivateData().GetViewBook().GetCondList()
	if len(condList) != 1 || condList[0].GetType() != 2 || len(condList[0].GetUnlockIds()) != 1 || condList[0].GetUnlockIds()[0] != 100100 {
		t.Fatalf("expected hydrated view book cond list, got %+v", condList)
	}
}
