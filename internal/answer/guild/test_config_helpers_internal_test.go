package guild

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"google.golang.org/protobuf/proto"
)

func setupConfigTest(t *testing.T) *connection.Client {
	t.Helper()
	os.Setenv("MODE", "test")
	orm.InitDatabase()

	commanderID := uint32(time.Now().UnixNano() % 1_000_000_000)
	name := "Guild Config Tester"
	if err := orm.CreateCommanderRoot(commanderID, commanderID, name, 0, 0); err != nil {
		t.Fatalf("failed to seed commander: %v", err)
	}
	commander := &orm.Commander{CommanderID: commanderID}
	if err := commander.Load(); err != nil {
		t.Fatalf("failed to load commander: %v", err)
	}
	return &connection.Client{Commander: commander}
}

func seedConfigEntry(t *testing.T, category string, key string, payload string) {
	t.Helper()
	if err := orm.UpsertConfigEntry(category, key, json.RawMessage(payload)); err != nil {
		t.Fatalf("seed config entry failed: %v", err)
	}
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
