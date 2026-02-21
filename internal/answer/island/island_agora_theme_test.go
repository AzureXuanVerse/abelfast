package island

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

var islandAgoraTestDBOnce sync.Once

func initIslandAgoraTestDB(t *testing.T) {
	t.Helper()
	t.Setenv("MODE", "test")
	islandAgoraTestDBOnce.Do(func() {
		orm.InitDatabase()
	})
}

func newIslandAgoraTestClient(t *testing.T) *connection.Client {
	t.Helper()
	initIslandAgoraTestDB(t)

	commanderID := uint32(time.Now().UnixNano())
	if err := orm.CreateCommanderRoot(commanderID, commanderID, "Island Agora Theme Tester", 0, 0); err != nil {
		t.Fatalf("create commander: %v", err)
	}

	commander := orm.Commander{CommanderID: commanderID}
	if err := commander.Load(); err != nil {
		t.Fatalf("load commander: %v", err)
	}

	return &connection.Client{Commander: &commander}
}

func TestDeleteIslandAgoraThemeSuccess(t *testing.T) {
	client := newIslandAgoraTestClient(t)
	ctx := context.Background()

	err := orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		return orm.UpsertIslandAgoraThemeTx(ctx, tx, client.Commander.CommanderID, 3, "To Delete", []byte{1, 2, 3})
	})
	if err != nil {
		t.Fatalf("seed theme: %v", err)
	}

	request := protobuf.CS_21319{Id: proto.Uint32(3)}
	buffer, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	if _, _, err := DeleteIslandAgoraTheme(&buffer, client); err != nil {
		t.Fatalf("DeleteIslandAgoraTheme returned error: %v", err)
	}

	var response protobuf.SC_21320
	decodePacketAt(t, client, 0, 21320, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}

	themes, err := orm.ListIslandAgoraThemes(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("list themes: %v", err)
	}
	if len(themes) != 0 {
		t.Fatalf("expected no themes after delete, got %d", len(themes))
	}
}

func TestDeleteIslandAgoraThemeNotFoundIsIdempotent(t *testing.T) {
	client := newIslandAgoraTestClient(t)

	request := protobuf.CS_21319{Id: proto.Uint32(99)}
	buffer, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	if _, _, err := DeleteIslandAgoraTheme(&buffer, client); err != nil {
		t.Fatalf("DeleteIslandAgoraTheme returned error: %v", err)
	}

	var response protobuf.SC_21320
	decodePacketAt(t, client, 0, 21320, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}
}

func TestDeleteIslandAgoraThemeMalformedPayloadDoesNotMutate(t *testing.T) {
	client := newIslandAgoraTestClient(t)
	ctx := context.Background()

	err := orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		return orm.UpsertIslandAgoraThemeTx(ctx, tx, client.Commander.CommanderID, 7, "Keep", []byte{4, 5, 6})
	})
	if err != nil {
		t.Fatalf("seed theme: %v", err)
	}

	beforeCount := queryAnswerTestInt64(t, `SELECT COUNT(*) FROM island_agora_themes WHERE commander_id = $1`, client.Commander.CommanderID)

	malformed := []byte{0xff}
	_, packetID, err := DeleteIslandAgoraTheme(&malformed, client)
	if err == nil {
		t.Fatalf("expected unmarshal error")
	}
	if packetID != 21320 {
		t.Fatalf("expected packet id 21320, got %d", packetID)
	}

	afterCount := queryAnswerTestInt64(t, `SELECT COUNT(*) FROM island_agora_themes WHERE commander_id = $1`, client.Commander.CommanderID)
	if afterCount != beforeCount {
		t.Fatalf("expected row count unchanged, before=%d after=%d", beforeCount, afterCount)
	}
}

func TestDeleteIslandAgoraThemeDbFailureReturnsNonZero(t *testing.T) {
	client := newIslandAgoraTestClient(t)

	request := protobuf.CS_21319{Id: proto.Uint32(1)}
	buffer, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	originalStore := db.DefaultStore
	db.DefaultStore = nil
	defer func() { db.DefaultStore = originalStore }()

	if _, _, err := DeleteIslandAgoraTheme(&buffer, client); err != nil {
		t.Fatalf("DeleteIslandAgoraTheme returned error: %v", err)
	}

	var response protobuf.SC_21320
	decodePacketAt(t, client, 0, 21320, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result when db is unavailable")
	}
}

func TestSaveIslandAgoraThemeAndList(t *testing.T) {
	client := newIslandAgoraTestClient(t)

	request := protobuf.CS_21317{Theme: &protobuf.PB_PLACEMENT_THEME{
		Id:   proto.Uint32(5),
		Name: proto.String("Theme A"),
		PlacedData: &protobuf.PB_PLACEMENT_DATA{
			PlacedList: []*protobuf.PB_FURNITURE_DATA{{Id: proto.Uint32(1001), X: proto.Int32(3), Y: proto.Int32(4), Dir: proto.Uint32(2)}},
			FloorData:  []uint32{1, 2},
			TileData:   []uint32{9},
		},
	}}
	buffer, err := proto.Marshal(&request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	if _, _, err := SaveIslandAgoraTheme(&buffer, client); err != nil {
		t.Fatalf("SaveIslandAgoraTheme returned error: %v", err)
	}

	var saveResponse protobuf.SC_21318
	decodePacketAt(t, client, 0, 21318, &saveResponse)
	if saveResponse.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", saveResponse.GetResult())
	}

	listReq := protobuf.CS_21321{Id: proto.Uint32(client.Commander.CommanderID)}
	listBuffer, _ := proto.Marshal(&listReq)
	client.Buffer.Reset()
	if _, _, err := ListIslandAgoraThemes(&listBuffer, client); err != nil {
		t.Fatalf("ListIslandAgoraThemes returned error: %v", err)
	}

	var listResponse protobuf.SC_21322
	decodePacketAt(t, client, 0, 21322, &listResponse)
	if len(listResponse.GetThemeList()) != 1 {
		t.Fatalf("expected one theme, got %d", len(listResponse.GetThemeList()))
	}
	theme := listResponse.GetThemeList()[0]
	if theme.GetId() != 5 || theme.GetName() != "Theme A" {
		t.Fatalf("unexpected theme metadata: %+v", theme)
	}
	if len(theme.GetPlacedData().GetPlacedList()) != 1 || theme.GetPlacedData().GetPlacedList()[0].GetX() != 3 {
		t.Fatalf("unexpected placement payload: %+v", theme.GetPlacedData())
	}
}

func TestSaveIslandAgoraThemeOverwriteSlot(t *testing.T) {
	client := newIslandAgoraTestClient(t)

	first := protobuf.CS_21317{Theme: &protobuf.PB_PLACEMENT_THEME{Id: proto.Uint32(2), Name: proto.String("Old"), PlacedData: &protobuf.PB_PLACEMENT_DATA{FloorData: []uint32{1}}}}
	firstBuffer, _ := proto.Marshal(&first)
	if _, _, err := SaveIslandAgoraTheme(&firstBuffer, client); err != nil {
		t.Fatalf("first save failed: %v", err)
	}

	second := protobuf.CS_21317{Theme: &protobuf.PB_PLACEMENT_THEME{Id: proto.Uint32(2), Name: proto.String("New"), PlacedData: &protobuf.PB_PLACEMENT_DATA{FloorData: []uint32{7}}}}
	secondBuffer, _ := proto.Marshal(&second)
	client.Buffer.Reset()
	if _, _, err := SaveIslandAgoraTheme(&secondBuffer, client); err != nil {
		t.Fatalf("second save failed: %v", err)
	}

	var saveResponse protobuf.SC_21318
	decodePacketAt(t, client, 0, 21318, &saveResponse)
	if saveResponse.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", saveResponse.GetResult())
	}

	themes, err := orm.ListIslandAgoraThemes(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("list themes: %v", err)
	}
	if len(themes) != 1 || themes[0].Name != "New" {
		t.Fatalf("expected single overwritten theme, got %+v", themes)
	}
}

func TestSaveIslandAgoraThemeMalformedPayloadAndDBFailure(t *testing.T) {
	client := newIslandAgoraTestClient(t)

	malformed := []byte{0xff}
	_, packetID, err := SaveIslandAgoraTheme(&malformed, client)
	if err == nil {
		t.Fatalf("expected unmarshal error")
	}
	if packetID != 21318 {
		t.Fatalf("expected packet id 21318, got %d", packetID)
	}

	request := protobuf.CS_21317{Theme: &protobuf.PB_PLACEMENT_THEME{Id: proto.Uint32(1), Name: proto.String("X"), PlacedData: &protobuf.PB_PLACEMENT_DATA{}}}
	buffer, _ := proto.Marshal(&request)
	originalStore := db.DefaultStore
	db.DefaultStore = nil
	defer func() { db.DefaultStore = originalStore }()

	if _, _, err := SaveIslandAgoraTheme(&buffer, client); err != nil {
		t.Fatalf("SaveIslandAgoraTheme returned error: %v", err)
	}

	var response protobuf.SC_21318
	decodePacketAt(t, client, 0, 21318, &response)
	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result when db unavailable")
	}
}

func TestListIslandAgoraThemesMalformedPlacedDataAndScope(t *testing.T) {
	client := newIslandAgoraTestClient(t)
	other := newIslandAgoraTestClient(t)
	ctx := context.Background()

	err := orm.WithPGXTx(ctx, func(tx pgx.Tx) error {
		if err := orm.UpsertIslandAgoraThemeTx(ctx, tx, client.Commander.CommanderID, 1, "Bad", []byte{0xff, 0x01}); err != nil {
			return err
		}
		return orm.UpsertIslandAgoraThemeTx(ctx, tx, other.Commander.CommanderID, 8, "Other", []byte{1, 2, 3})
	})
	if err != nil {
		t.Fatalf("seed themes: %v", err)
	}

	request := protobuf.CS_21321{Id: proto.Uint32(other.Commander.CommanderID)}
	buffer, _ := proto.Marshal(&request)
	if _, _, err := ListIslandAgoraThemes(&buffer, client); err != nil {
		t.Fatalf("ListIslandAgoraThemes returned error: %v", err)
	}

	var response protobuf.SC_21322
	decodePacketAt(t, client, 0, 21322, &response)
	if len(response.GetThemeList()) != 1 || response.GetThemeList()[0].GetId() != 1 {
		t.Fatalf("expected only caller theme list, got %+v", response.GetThemeList())
	}
	if len(response.GetThemeList()[0].GetPlacedData().GetPlacedList()) != 0 {
		t.Fatalf("expected malformed placed data to fallback to empty")
	}
}
