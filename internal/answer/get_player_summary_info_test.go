package answer

import (
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestGetPlayerSummaryInfoSuccess(t *testing.T) {
	client := setupHandlerCommander(t)

	seedShipTemplate(t, 10011, 1, 3, 1, "Ship 10011", 1)
	seedShipTemplate(t, 10021, 1, 3, 1, "Ship 10021", 1)
	ship1 := seedOwnedShip(t, client, 10011)
	ship2 := seedOwnedShip(t, client, 10021)

	firstProposeAt := time.Unix(1700000000, 0).UTC()
	secondProposeAt := time.Unix(1700003600, 0).UTC()
	execAnswerTestSQLT(t, `
UPDATE owned_ships
SET propose = TRUE,
	max_level = 130,
	intimacy = 21000,
	custom_name = 'Beloved',
	create_time = $3
WHERE owner_id = $1
	AND id = $2
`, int64(client.Commander.CommanderID), int64(ship1.ID), firstProposeAt)
	execAnswerTestSQLT(t, `
UPDATE owned_ships
SET propose = FALSE,
	max_level = 120,
	intimacy = 19000,
	is_secretary = TRUE,
	secretary_position = 0,
	create_time = $3
WHERE owner_id = $1
	AND id = $2
`, int64(client.Commander.CommanderID), int64(ship2.ID), secondProposeAt)

	if err := orm.UpsertChapterProgress(&orm.ChapterProgress{CommanderID: client.Commander.CommanderID, ChapterID: 120}); err != nil {
		t.Fatalf("upsert chapter progress: %v", err)
	}
	execAnswerTestSQLT(t, `INSERT INTO commander_furnitures (commander_id, furniture_id, count, get_time) VALUES ($1, $2, $3, $4)`, int64(client.Commander.CommanderID), int64(1), int64(2), int64(1))
	execAnswerTestSQLT(t, `INSERT INTO commander_furnitures (commander_id, furniture_id, count, get_time) VALUES ($1, $2, $3, $4)`, int64(client.Commander.CommanderID), int64(2), int64(3), int64(1))
	if err := orm.SetCommanderMedalDisplay(client.Commander.CommanderID, []uint32{11, 22}); err != nil {
		t.Fatalf("set medal display: %v", err)
	}
	execAnswerTestSQLT(t, `INSERT INTO skins (id, name, ship_group) VALUES ($1, $2, $3)`, int64(50011), "Skin 1", int64(1001))
	execAnswerTestSQLT(t, `INSERT INTO skins (id, name, ship_group) VALUES ($1, $2, $3)`, int64(50021), "Skin 2", int64(1002))
	if err := client.Commander.GiveSkin(50011); err != nil {
		t.Fatalf("give skin 1: %v", err)
	}
	if err := client.Commander.GiveSkin(50021); err != nil {
		t.Fatalf("give skin 2: %v", err)
	}

	client.Buffer.Reset()
	payload := protobuf.CS_26021{ActId: proto.Uint32(1)}
	data, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if _, _, err := GetPlayerSummaryInfo(&data, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_26022
	decodeResponse(t, client, &response)

	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if response.RegisterDate == nil || response.GuildName == nil || response.ChapterId == nil || response.CharacterId == nil {
		t.Fatalf("expected required fields to be set")
	}
	if response.GetChapterId() != 120 {
		t.Fatalf("expected chapter 120, got %d", response.GetChapterId())
	}
	if response.GetMarryNumber() != 1 {
		t.Fatalf("expected marry number 1, got %d", response.GetMarryNumber())
	}
	if response.GetMedalNumber() != 2 {
		t.Fatalf("expected medal number 2, got %d", response.GetMedalNumber())
	}
	if response.GetFurnitureNumber() != 5 {
		t.Fatalf("expected furniture number 5, got %d", response.GetFurnitureNumber())
	}
	if response.GetCharacterId() != 10021 {
		t.Fatalf("expected character id 10021, got %d", response.GetCharacterId())
	}
	if response.GetFirstLadyId() != 10011 {
		t.Fatalf("expected first lady id 10011, got %d", response.GetFirstLadyId())
	}
	if response.GetFirstLadyName() != "Beloved" {
		t.Fatalf("expected first lady name Beloved, got %q", response.GetFirstLadyName())
	}
	if response.GetFirstLadyTime() != uint32(firstProposeAt.Unix()) {
		t.Fatalf("expected first lady time %d, got %d", firstProposeAt.Unix(), response.GetFirstLadyTime())
	}
	if response.GetCollectNum() != 2 {
		t.Fatalf("expected collect num 2, got %d", response.GetCollectNum())
	}
	if response.GetShipNumTotal() != 2 || response.GetShipNum_120() != 2 || response.GetShipNum_125() != 1 {
		t.Fatalf("unexpected ship count stats: total=%d lvl120=%d lvl125=%d", response.GetShipNumTotal(), response.GetShipNum_120(), response.GetShipNum_125())
	}
	if response.GetLove200Num() != 1 {
		t.Fatalf("expected love200 number 1, got %d", response.GetLove200Num())
	}
	if response.GetSkinNum() != 2 || response.GetSkinShipNum() != 2 {
		t.Fatalf("unexpected skin stats: skin_num=%d skin_ship_num=%d", response.GetSkinNum(), response.GetSkinShipNum())
	}
	if response.GetFirstOnline() != response.GetRegisterDate() {
		t.Fatalf("expected first_online to match register_date")
	}
}

func TestGetPlayerSummaryInfoChapterFallbackAndNoProposal(t *testing.T) {
	client := setupHandlerCommander(t)

	client.Buffer.Reset()
	payload := protobuf.CS_26021{ActId: proto.Uint32(1)}
	data, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if _, _, err := GetPlayerSummaryInfo(&data, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_26022
	decodeResponse(t, client, &response)
	if response.GetChapterId() != 101 {
		t.Fatalf("expected chapter fallback 101, got %d", response.GetChapterId())
	}
	if response.GetFirstLadyId() != 0 || response.GetFirstLadyTime() != 0 || response.GetFirstLadyName() != "" {
		t.Fatalf("expected empty first lady fields")
	}
}

func TestGetPlayerSummaryInfoMalformedPayload(t *testing.T) {
	client := setupHandlerCommander(t)
	data := []byte{0x01, 0x02}
	_, packetID, err := GetPlayerSummaryInfo(&data, client)
	if err == nil {
		t.Fatalf("expected unmarshal error")
	}
	if packetID != 26022 {
		t.Fatalf("expected packet id 26022, got %d", packetID)
	}
}
