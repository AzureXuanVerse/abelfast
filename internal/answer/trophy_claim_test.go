package answer

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func seedMedalTemplate(t *testing.T, id uint32, next uint32, targetNum uint32, countInherit uint32) {
	t.Helper()
	payload := fmt.Sprintf(`{"id":%d,"next":%d,"target_num":%d,"count_inherit":%d}`, id, next, targetNum, countInherit)
	seedConfigEntry(t, medalTemplateCategory, strconv.FormatUint(uint64(id), 10), payload)
}

func TestClaimTrophyClaimsAndPersistsTimestamp(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderTrophyProgress{})

	seedMedalTemplate(t, 1001, 0, 10, 0)
	row := orm.CommanderTrophyProgress{CommanderID: client.Commander.CommanderID, TrophyID: 1001, Progress: 10, Timestamp: 0}
	if err := orm.UpdateCommanderTrophyProgress(&row); err != nil {
		t.Fatalf("seed trophy progress: %v", err)
	}

	payload := protobuf.CS_17301{Id: proto.Uint32(1001)}
	buf, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := ClaimTrophy(&buf, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_17302
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if response.GetTimestamp() == 0 {
		t.Fatalf("expected timestamp to be set")
	}

	stored, err := orm.GetCommanderTrophyProgress(client.Commander.CommanderID, 1001)
	if err != nil {
		t.Fatalf("load stored trophy: %v", err)
	}
	if stored.Timestamp != response.GetTimestamp() {
		t.Fatalf("expected stored timestamp %d, got %d", response.GetTimestamp(), stored.Timestamp)
	}
}

func TestClaimTrophyCreatesProgressRowWhenMissing(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderTrophyProgress{})

	seedMedalTemplate(t, 6001, 0, 10, 0)

	payload := protobuf.CS_17301{Id: proto.Uint32(6001)}
	buf, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := ClaimTrophy(&buf, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_17302
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if response.GetTimestamp() == 0 {
		t.Fatalf("expected timestamp to be set")
	}

	stored, err := orm.GetCommanderTrophyProgress(client.Commander.CommanderID, 6001)
	if err != nil {
		t.Fatalf("load stored trophy: %v", err)
	}
	if stored.Progress != 10 {
		t.Fatalf("expected stored progress 10, got %d", stored.Progress)
	}
	if stored.Timestamp != response.GetTimestamp() {
		t.Fatalf("expected stored timestamp %d, got %d", response.GetTimestamp(), stored.Timestamp)
	}
}

func TestClaimTrophyUnlocksNextWhenMissing(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderTrophyProgress{})

	seedMedalTemplate(t, 2001, 2002, 1, 0)
	seedMedalTemplate(t, 2002, 0, 999, 0)
	row := orm.CommanderTrophyProgress{CommanderID: client.Commander.CommanderID, TrophyID: 2001, Progress: 1, Timestamp: 0}
	if err := orm.UpdateCommanderTrophyProgress(&row); err != nil {
		t.Fatalf("seed trophy progress: %v", err)
	}

	payload := protobuf.CS_17301{Id: proto.Uint32(2001)}
	buf, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := ClaimTrophy(&buf, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_17302
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if len(response.GetNext()) != 1 {
		t.Fatalf("expected 1 next trophy, got %d", len(response.GetNext()))
	}
	next := response.GetNext()[0]
	if next.GetId() != 2002 {
		t.Fatalf("expected next id 2002, got %d", next.GetId())
	}
	if next.GetTimestamp() != 0 {
		t.Fatalf("expected next timestamp 0")
	}
	if next.GetProgress() != 0 {
		t.Fatalf("expected next progress 0")
	}

	stored, err := orm.GetCommanderTrophyProgress(client.Commander.CommanderID, 2002)
	if err != nil {
		t.Fatalf("load stored next trophy: %v", err)
	}
	if stored.Timestamp != 0 {
		t.Fatalf("expected stored next timestamp 0")
	}
}

func TestClaimTrophyNextInheritsProgressWhenConfigured(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderTrophyProgress{})

	seedMedalTemplate(t, 3001, 3002, 1, 3002)
	seedMedalTemplate(t, 3002, 0, 999, 0)
	row := orm.CommanderTrophyProgress{CommanderID: client.Commander.CommanderID, TrophyID: 3001, Progress: 77, Timestamp: 0}
	if err := orm.UpdateCommanderTrophyProgress(&row); err != nil {
		t.Fatalf("seed trophy progress: %v", err)
	}

	payload := protobuf.CS_17301{Id: proto.Uint32(3001)}
	buf, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := ClaimTrophy(&buf, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_17302
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if len(response.GetNext()) != 1 {
		t.Fatalf("expected 1 next trophy")
	}
	if response.GetNext()[0].GetProgress() != 77 {
		t.Fatalf("expected inherited progress 77, got %d", response.GetNext()[0].GetProgress())
	}

	stored, err := orm.GetCommanderTrophyProgress(client.Commander.CommanderID, 3002)
	if err != nil {
		t.Fatalf("load stored next trophy: %v", err)
	}
	if stored.Progress != 77 {
		t.Fatalf("expected stored inherited progress 77, got %d", stored.Progress)
	}
}

func TestClaimTrophyRejectsAlreadyClaimedWithoutMutation(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderTrophyProgress{})

	seedMedalTemplate(t, 4001, 0, 1, 0)
	row := orm.CommanderTrophyProgress{CommanderID: client.Commander.CommanderID, TrophyID: 4001, Progress: 1, Timestamp: 123}
	if err := orm.UpdateCommanderTrophyProgress(&row); err != nil {
		t.Fatalf("seed trophy progress: %v", err)
	}

	payload := protobuf.CS_17301{Id: proto.Uint32(4001)}
	buf, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := ClaimTrophy(&buf, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_17302
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result")
	}
	stored, err := orm.GetCommanderTrophyProgress(client.Commander.CommanderID, 4001)
	if err != nil {
		t.Fatalf("load stored trophy: %v", err)
	}
	if stored.Timestamp != 123 {
		t.Fatalf("expected timestamp unchanged")
	}
}

func TestClaimTrophyRejectsInsufficientProgressWithoutMutation(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderTrophyProgress{})

	seedMedalTemplate(t, 5001, 0, 10, 0)
	row := orm.CommanderTrophyProgress{CommanderID: client.Commander.CommanderID, TrophyID: 5001, Progress: 9, Timestamp: 0}
	if err := orm.UpdateCommanderTrophyProgress(&row); err != nil {
		t.Fatalf("seed trophy progress: %v", err)
	}

	payload := protobuf.CS_17301{Id: proto.Uint32(5001)}
	buf, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := ClaimTrophy(&buf, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_17302
	decodeResponse(t, client, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result")
	}
	stored, err := orm.GetCommanderTrophyProgress(client.Commander.CommanderID, 5001)
	if err != nil {
		t.Fatalf("load stored trophy: %v", err)
	}
	if stored.Timestamp != 0 {
		t.Fatalf("expected timestamp unchanged")
	}
}

func TestClaimTrophyEmitsSC17002DeltaAfterSuccessfulClaim(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderTrophyProgress{})

	seedMedalTemplate(t, 8001, 8002, 5, 0)
	seedMedalTemplate(t, 8002, 0, 999, 0)
	row := orm.CommanderTrophyProgress{CommanderID: client.Commander.CommanderID, TrophyID: 8001, Progress: 5, Timestamp: 0}
	if err := orm.UpdateCommanderTrophyProgress(&row); err != nil {
		t.Fatalf("seed trophy progress: %v", err)
	}

	payload := protobuf.CS_17301{Id: proto.Uint32(8001)}
	buf, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := ClaimTrophy(&buf, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var claim protobuf.SC_17302
	offset := decodePacketAt(t, client, 0, 17302, &claim)
	if claim.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", claim.GetResult())
	}

	var delta protobuf.SC_17002
	offset = decodePacketAt(t, client, offset, 17002, &delta)
	if len(delta.GetProgressList()) != 2 {
		t.Fatalf("expected 2 progress delta rows, got %d", len(delta.GetProgressList()))
	}
	byID := map[uint32]*protobuf.ACHIEVEMENT_INFO{}
	for i := range delta.GetProgressList() {
		byID[delta.GetProgressList()[i].GetId()] = delta.GetProgressList()[i]
	}
	if byID[8001] == nil || byID[8001].GetProgress() != 5 || byID[8001].GetTimestamp() == 0 {
		t.Fatalf("expected claimed trophy row in delta")
	}
	if byID[8002] == nil || byID[8002].GetProgress() != 0 || byID[8002].GetTimestamp() != 0 {
		t.Fatalf("expected unlocked next trophy row in delta")
	}
	if offset != len(client.Buffer.Bytes()) {
		t.Fatalf("expected exactly two packets")
	}
}

func TestClaimTrophyRejectedClaimDoesNotEmitSC17002(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.CommanderTrophyProgress{})

	seedMedalTemplate(t, 9001, 0, 1, 0)
	row := orm.CommanderTrophyProgress{CommanderID: client.Commander.CommanderID, TrophyID: 9001, Progress: 1, Timestamp: 123}
	if err := orm.UpdateCommanderTrophyProgress(&row); err != nil {
		t.Fatalf("seed trophy progress: %v", err)
	}

	payload := protobuf.CS_17301{Id: proto.Uint32(9001)}
	buf, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := ClaimTrophy(&buf, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var claim protobuf.SC_17302
	offset := decodePacketAt(t, client, 0, 17302, &claim)
	if claim.GetResult() == 0 {
		t.Fatalf("expected non-zero result")
	}
	if offset != len(client.Buffer.Bytes()) {
		t.Fatalf("expected only SC_17302 packet")
	}
}
