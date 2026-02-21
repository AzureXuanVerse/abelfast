package island

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/packets"
	"google.golang.org/protobuf/proto"
)

var playerUpdateCommanderID uint32 = 9000

func decodePacketAt(t *testing.T, client *connection.Client, offset int, expectedID int, message proto.Message) int {
	t.Helper()
	buffer := client.Buffer.Bytes()
	if len(buffer) == 0 {
		t.Fatalf("expected response buffer")
	}
	packetID := packets.GetPacketId(offset, &buffer)
	if packetID != expectedID {
		t.Fatalf("expected packet %d, got %d", expectedID, packetID)
	}
	packetSize := packets.GetPacketSize(offset, &buffer) + 2
	if len(buffer) < offset+packetSize {
		t.Fatalf("expected packet size %d, got %d", offset+packetSize, len(buffer))
	}
	payloadStart := offset + packets.HEADER_SIZE
	payloadEnd := payloadStart + (packetSize - packets.HEADER_SIZE)
	if err := proto.Unmarshal(buffer[payloadStart:payloadEnd], message); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	return offset + packetSize
}

func clearTable(t *testing.T, model any) {
	t.Helper()
	tableName, err := tableNameFromModel(model)
	if err != nil {
		t.Fatalf("failed to resolve table name: %v", err)
	}
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", quoteIdentifier(tableName))); err != nil {
		t.Fatalf("failed to clear table: %v", err)
	}
}

func tableNameFromModel(model any) (string, error) {
	type tableNamer interface{ TableName() string }
	if named, ok := model.(tableNamer); ok {
		return named.TableName(), nil
	}

	t := reflect.TypeOf(model)
	if t == nil {
		return "", fmt.Errorf("model is nil")
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return "", fmt.Errorf("model must be struct or pointer to struct")
	}

	name := t.Name()
	if name == "" {
		return "", fmt.Errorf("model type has no name")
	}

	if name == "OwnedSpWeapon" {
		return "owned_spweapons", nil
	}
	if name == "ChapterProgress" {
		return "chapter_progress", nil
	}

	var b strings.Builder
	runes := []rune(name)
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
				if unicode.IsLower(prev) || nextIsLower {
					b.WriteRune('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	snake := b.String()
	if strings.HasSuffix(snake, "y") && len(snake) > 1 {
		last := rune(snake[len(snake)-2])
		if !strings.ContainsRune("aeiou", last) {
			return snake[:len(snake)-1] + "ies", nil
		}
	}
	if strings.HasSuffix(snake, "s") || strings.HasSuffix(snake, "x") || strings.HasSuffix(snake, "z") || strings.HasSuffix(snake, "ch") || strings.HasSuffix(snake, "sh") {
		return snake + "es", nil
	}
	return snake + "s", nil
}

func quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func seedConfigEntry(t *testing.T, category string, key string, payload string) {
	t.Helper()
	if err := orm.UpsertConfigEntry(category, key, json.RawMessage(payload)); err != nil {
		t.Fatalf("seed config entry failed: %v", err)
	}
}

func setupHandlerCommander(t *testing.T) *connection.Client {
	t.Helper()
	os.Setenv("MODE", "test")
	orm.InitDatabase()
	clearTable(t, &orm.Commander{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.OwnedSkin{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.CommanderMiscItem{})
	clearTable(t, &orm.OwnedResource{})
	clearTable(t, &orm.OwnedSpWeapon{})
	clearTable(t, &orm.Mail{})
	clearTable(t, &orm.MailAttachment{})
	clearTable(t, &orm.Notice{})
	clearTable(t, &orm.Build{})
	clearTable(t, &orm.Ship{})
	clearTable(t, &orm.RandomFlagShip{})
	clearTable(t, &orm.Like{})
	clearTable(t, &orm.CommanderAppreciationState{})
	clearTable(t, &orm.CommanderMedalDisplay{})
	clearTable(t, &orm.CommanderTrophyProgress{})
	clearTable(t, &orm.CommanderStoreupAwardProgress{})
	clearTable(t, &orm.SecondaryPasswordState{})
	clearTable(t, &orm.ActivityPermanentState{})
	clearTable(t, &orm.EscortState{})
	clearTable(t, &orm.IslandWildGatherSignState{})
	clearTable(t, &orm.IslandWildGatherCollectState{})
	clearTable(t, &orm.IslandCollectFragmentState{})
	clearTable(t, &orm.IslandCollectFragmentSignState{})
	clearTable(t, &orm.IslandCollectionCompleteState{})
	clearTable(t, &orm.IslandSlotCollectState{})
	clearTable(t, &orm.CommanderHomeSlot{})
	clearTable(t, &orm.CommanderHome{})
	commanderID := uint32(time.Now().UnixNano())
	if err := orm.CreateCommanderRoot(commanderID, commanderID, fmt.Sprintf("Handler Commander %d", commanderID), 0, 0); err != nil {
		t.Fatalf("create commander: %v", err)
	}
	commander := orm.Commander{CommanderID: commanderID}
	if err := commander.Load(); err != nil {
		t.Fatalf("load commander: %v", err)
	}
	client := &connection.Client{Commander: &commander}
	client.Server = connection.NewServer("127.0.0.1", 0, func(pkt *[]byte, c *connection.Client, size int) {})
	return client
}

func seedShipTemplate(t *testing.T, templateID uint32, poolID uint32, rarity uint32, shipType uint32, englishName string, star uint32) {
	t.Helper()
	ship := orm.Ship{
		TemplateID:  templateID,
		Name:        fmt.Sprintf("Ship %d", templateID),
		EnglishName: englishName,
		RarityID:    rarity,
		Star:        star,
		Type:        shipType,
		Nationality: 1,
		BuildTime:   1,
		PoolID:      &poolID,
	}
	if err := ship.Create(); err != nil {
		t.Fatalf("seed ship: %v", err)
	}
}

func seedOwnedShip(t *testing.T, client *connection.Client, shipTemplateID uint32) *orm.OwnedShip {
	t.Helper()
	ship := orm.OwnedShip{OwnerID: client.Commander.CommanderID, ShipID: shipTemplateID}
	if err := ship.Create(); err != nil {
		t.Fatalf("seed owned ship: %v", err)
	}
	if err := client.Commander.Load(); err != nil {
		t.Fatalf("reload commander: %v", err)
	}
	return client.Commander.OwnedShipsMap[ship.ID]
}

func seedHandlerCommanderItem(t *testing.T, client *connection.Client, itemID uint32, count uint32) {
	t.Helper()
	if err := client.Commander.SetItem(itemID, count); err != nil {
		t.Fatalf("seed item: %v", err)
	}
}

func seedHandlerCommanderResource(t *testing.T, client *connection.Client, resourceID uint32, amount uint32) {
	t.Helper()
	if err := client.Commander.SetResource(resourceID, amount); err != nil {
		t.Fatalf("seed resource: %v", err)
	}
}

func execAnswerTestSQLT(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), query, args...); err != nil {
		t.Fatalf("exec sql failed: %v", err)
	}
}

func queryAnswerTestInt64(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var value int64
	if err := db.DefaultStore.Pool.QueryRow(context.Background(), query, args...).Scan(&value); err != nil {
		t.Fatalf("query row failed: %v", err)
	}
	return value
}

func decodeResponse(t *testing.T, client *connection.Client, response proto.Message) {
	t.Helper()
	data := client.Buffer.Bytes()
	if len(data) < 7 {
		t.Fatalf("expected buffer to include header and payload")
	}
	if err := proto.Unmarshal(data[7:], response); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
}

func setupPlayerUpdateTest(t *testing.T) *connection.Client {
	t.Helper()
	os.Setenv("MODE", "test")
	orm.InitDatabase()
	clearTable(t, &orm.CommanderCommonFlag{})
	clearTable(t, &orm.CommanderStory{})
	clearTable(t, &orm.CommanderAttire{})
	clearTable(t, &orm.CommanderLivingAreaCover{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.CommanderMiscItem{})
	clearTable(t, &orm.OwnedShip{})
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.Commander{})

	commanderID := atomic.AddUint32(&playerUpdateCommanderID, 1)
	commander := orm.Commander{
		CommanderID: commanderID,
		AccountID:   1,
		Level:       30,
		Exp:         0,
		Name:        fmt.Sprintf("Update Tester %d", commanderID),
		LastLogin:   time.Now().UTC(),
	}
	if err := orm.CreateCommanderRoot(commanderID, 1, commander.Name, 0, 0); err != nil {
		t.Fatalf("create commander: %v", err)
	}
	execAnswerTestSQLT(t, "UPDATE commanders SET level = $1, exp = $2, last_login = $3 WHERE commander_id = $4", int64(commander.Level), int64(commander.Exp), commander.LastLogin, int64(commanderID))
	if err := commander.Load(); err != nil {
		t.Fatalf("load commander: %v", err)
	}
	return &connection.Client{Commander: &commander}
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
