package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestCommanderCollectionIncludesTrophyProgressAndNo17003Push(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.CommanderTrophyProgress{})

	if err := orm.UpdateCommanderTrophyProgress(&orm.CommanderTrophyProgress{CommanderID: client.Commander.CommanderID, TrophyID: 7001, Progress: 3, Timestamp: 0}); err != nil {
		t.Fatalf("seed trophy progress 7001: %v", err)
	}
	if err := orm.UpdateCommanderTrophyProgress(&orm.CommanderTrophyProgress{CommanderID: client.Commander.CommanderID, TrophyID: 7002, Progress: 8, Timestamp: 99}); err != nil {
		t.Fatalf("seed trophy progress 7002: %v", err)
	}

	client.Buffer.Reset()
	empty := []byte{}
	if _, _, err := CommanderCollection(&empty, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_17001
	offset := decodePacketAt(t, client, 0, 17001, &response)

	if len(response.GetProgressList()) != 2 {
		t.Fatalf("expected 2 progress entries, got %d", len(response.GetProgressList()))
	}
	progressByID := map[uint32]*protobuf.ACHIEVEMENT_INFO{}
	for i := range response.GetProgressList() {
		progressByID[response.GetProgressList()[i].GetId()] = response.GetProgressList()[i]
	}
	if progressByID[7001] == nil || progressByID[7001].GetProgress() != 3 || progressByID[7001].GetTimestamp() != 0 {
		t.Fatalf("unexpected trophy progress for 7001")
	}
	if progressByID[7002] == nil || progressByID[7002].GetProgress() != 8 || progressByID[7002].GetTimestamp() != 99 {
		t.Fatalf("unexpected trophy progress for 7002")
	}

	if offset != len(client.Buffer.Bytes()) {
		t.Fatalf("expected only SC_17001 packet, got extra output")
	}
}

func TestCommanderCollectionNo17003CompletionPush(t *testing.T) {
	client := setupHandlerCommander(t)
	client.Buffer.Reset()
	empty := []byte{}
	if _, _, err := CommanderCollection(&empty, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_17001
	offset := decodePacketAt(t, client, 0, 17001, &response)
	if response.GetDailyDiscuss() != 0 {
		t.Fatalf("expected daily discuss 0")
	}
	if offset != len(client.Buffer.Bytes()) {
		t.Fatalf("expected only SC_17001 packet")
	}
}

func TestBuildAchievementInfoSetsRequiredFields(t *testing.T) {
	info := buildAchievementInfo(orm.CommanderTrophyProgress{TrophyID: 42, Progress: 13, Timestamp: 7})
	if info.GetId() != 42 || info.GetProgress() != 13 || info.GetTimestamp() != 7 {
		t.Fatalf("unexpected achievement info fields")
	}
	if info.Id == nil || info.Progress == nil || info.Timestamp == nil {
		t.Fatalf("expected required achievement fields to be present")
	}
	if !proto.Equal(info, &protobuf.ACHIEVEMENT_INFO{Id: proto.Uint32(42), Progress: proto.Uint32(13), Timestamp: proto.Uint32(7)}) {
		t.Fatalf("expected canonical achievement payload")
	}
}
