package answer_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/ggmolly/belfast/internal/answer"
	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func seedConfigEntryJSON(t *testing.T, category string, key string, value any) {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal config entry: %v", err)
	}
	execAnswerExternalTestSQLT(t, "DELETE FROM config_entries WHERE category = $1 AND key = $2", category, key)
	execAnswerExternalTestSQLT(t, "INSERT INTO config_entries (category, key, data) VALUES ($1, $2, $3::jsonb)", category, key, string(payload))
}

func seedGuildShopPurchaseEntry(t *testing.T, entry guildStoreEntry) {
	t.Helper()
	seedConfigEntryJSON(t, guildStoreConfigCategory, fmt.Sprintf("%d", entry.ID), entry)
	if entry.Type == consts.DROP_TYPE_ITEM {
		for _, itemID := range entry.Goods {
			execAnswerExternalTestSQLT(
				t,
				"INSERT INTO items (id, name, rarity, shop_id, type, virtual_type) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (id) DO NOTHING",
				int64(itemID),
				fmt.Sprintf("Guild Shop Item %d", itemID),
				int64(1),
				int64(-2),
				int64(0),
				int64(0),
			)
		}
	}
}

func seedGuildShopSlot(t *testing.T, commanderID uint32, index uint32, goodsID uint32, count uint32) {
	t.Helper()
	execAnswerExternalTestSQLT(t, "INSERT INTO guild_shop_goods (commander_id, \"index\", goods_id, count) VALUES ($1, $2, $3, $4)", int64(commanderID), int64(index), int64(goodsID), int64(count))
}

func readGuildShopSlotCount(t *testing.T, commanderID uint32, index uint32) uint32 {
	t.Helper()
	return uint32(queryAnswerExternalTestInt64(t, "SELECT count FROM guild_shop_goods WHERE commander_id = $1 AND \"index\" = $2", int64(commanderID), int64(index)))
}

func readGuildCoinAmount(t *testing.T, commanderID uint32) uint32 {
	t.Helper()
	return uint32(queryAnswerExternalTestInt64(t, "SELECT amount FROM owned_resources WHERE commander_id = $1 AND resource_id = 8", int64(commanderID)))
}

func setupGuildShopPurchaseClient(t *testing.T, commanderID uint32, guildCoins uint32) *connection.Client {
	t.Helper()
	os.Setenv("MODE", "test")
	orm.InitDatabase()

	cleanupGuildShopData(t, commanderID)
	seedGuildShopConfig(t)
	commander := setupGuildShopCommander(t, commanderID)
	if err := commander.SetResource(8, guildCoins); err != nil {
		t.Fatalf("set guild coins: %v", err)
	}
	return &connection.Client{Commander: commander}
}

func sendGuildShopPurchase(t *testing.T, client *connection.Client, request *protobuf.CS_60035) *protobuf.SC_60036 {
	t.Helper()
	buf, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, _, err := answer.GuildShopPurchase(&buf, client); err != nil {
		t.Fatalf("GuildShopPurchase failed: %v", err)
	}
	response := &protobuf.SC_60036{}
	decodeTestPacket(t, client, 60036, response)
	return response
}

func findGuildShopDrop(drops []*protobuf.DROPINFO, id uint32) *protobuf.DROPINFO {
	for _, d := range drops {
		if d.GetId() == id {
			return d
		}
	}
	return nil
}

func TestGuildShopPurchaseSuccessAggregatesDuplicates(t *testing.T) {
	commanderID := uint32(7101)
	client := setupGuildShopPurchaseClient(t, commanderID, 100)
	defer cleanupGuildShopData(t, commanderID)

	seedGuildShopPurchaseEntry(t, guildStoreEntry{ID: 3001, Price: 5, Goods: []uint32{20001, 20002}, GoodsType: 2, Num: 2, Type: consts.DROP_TYPE_ITEM})
	seedGuildShopSlot(t, commanderID, 1, 3001, 6)

	response := sendGuildShopPurchase(t, client, &protobuf.CS_60035{
		Goodsid: proto.Uint32(3001),
		Index:   proto.Uint32(1),
		Selected: []*protobuf.GUILD_SHOP_INFO{
			{Id: proto.Uint32(20001), Count: proto.Uint32(1)},
			{Id: proto.Uint32(20001), Count: proto.Uint32(2)},
			{Id: proto.Uint32(20002), Count: proto.Uint32(1)},
		},
	})

	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if len(response.GetDropList()) != 2 {
		t.Fatalf("expected 2 drops, got %d", len(response.GetDropList()))
	}
	if drop := findGuildShopDrop(response.GetDropList(), 20001); drop == nil || drop.GetType() != consts.DROP_TYPE_ITEM || drop.GetNumber() != 6 {
		t.Fatalf("expected item drop 20001 x6")
	}
	if drop := findGuildShopDrop(response.GetDropList(), 20002); drop == nil || drop.GetType() != consts.DROP_TYPE_ITEM || drop.GetNumber() != 2 {
		t.Fatalf("expected item drop 20002 x2")
	}
	if got := readGuildCoinAmount(t, commanderID); got != 80 {
		t.Fatalf("expected guild coins 80, got %d", got)
	}
	if got := readGuildShopSlotCount(t, commanderID, 1); got != 2 {
		t.Fatalf("expected slot stock 2, got %d", got)
	}
}

func TestGuildShopPurchaseSupportsFixedGoodsWithoutSelection(t *testing.T) {
	commanderID := uint32(7102)
	client := setupGuildShopPurchaseClient(t, commanderID, 40)
	defer cleanupGuildShopData(t, commanderID)

	seedGuildShopPurchaseEntry(t, guildStoreEntry{ID: 3002, Price: 10, Goods: []uint32{20003}, GoodsType: 1, Num: 1, Type: consts.DROP_TYPE_ITEM})
	seedGuildShopSlot(t, commanderID, 2, 3002, 3)

	response := sendGuildShopPurchase(t, client, &protobuf.CS_60035{
		Goodsid: proto.Uint32(3002),
		Index:   proto.Uint32(2),
	})

	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if drop := findGuildShopDrop(response.GetDropList(), 20003); drop == nil || drop.GetNumber() != 1 {
		t.Fatalf("expected item drop 20003 x1")
	}
	if got := readGuildCoinAmount(t, commanderID); got != 30 {
		t.Fatalf("expected guild coins 30, got %d", got)
	}
	if got := readGuildShopSlotCount(t, commanderID, 2); got != 2 {
		t.Fatalf("expected slot stock 2, got %d", got)
	}
}

func TestGuildShopPurchaseFixedBundleConsumesSingleUnit(t *testing.T) {
	commanderID := uint32(7108)
	client := setupGuildShopPurchaseClient(t, commanderID, 40)
	defer cleanupGuildShopData(t, commanderID)

	seedGuildShopPurchaseEntry(t, guildStoreEntry{ID: 3010, Price: 10, Goods: []uint32{20030, 20031}, GoodsType: 1, Num: 1, Type: consts.DROP_TYPE_ITEM})
	seedGuildShopSlot(t, commanderID, 8, 3010, 3)

	response := sendGuildShopPurchase(t, client, &protobuf.CS_60035{Goodsid: proto.Uint32(3010), Index: proto.Uint32(8)})

	if response.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", response.GetResult())
	}
	if drop := findGuildShopDrop(response.GetDropList(), 20030); drop == nil || drop.GetNumber() != 1 {
		t.Fatalf("expected item drop 20030 x1")
	}
	if drop := findGuildShopDrop(response.GetDropList(), 20031); drop == nil || drop.GetNumber() != 1 {
		t.Fatalf("expected item drop 20031 x1")
	}
	if got := readGuildCoinAmount(t, commanderID); got != 30 {
		t.Fatalf("expected guild coins 30, got %d", got)
	}
	if got := readGuildShopSlotCount(t, commanderID, 8); got != 2 {
		t.Fatalf("expected slot stock 2, got %d", got)
	}
}

func TestGuildShopPurchaseFixedBundleRejectsSelectedPayload(t *testing.T) {
	commanderID := uint32(7109)
	client := setupGuildShopPurchaseClient(t, commanderID, 40)
	defer cleanupGuildShopData(t, commanderID)

	seedGuildShopPurchaseEntry(t, guildStoreEntry{ID: 3011, Price: 10, Goods: []uint32{20032, 20033}, GoodsType: 1, Num: 1, Type: consts.DROP_TYPE_ITEM})
	seedGuildShopSlot(t, commanderID, 9, 3011, 3)

	response := sendGuildShopPurchase(t, client, &protobuf.CS_60035{
		Goodsid: proto.Uint32(3011),
		Index:   proto.Uint32(9),
		Selected: []*protobuf.GUILD_SHOP_INFO{
			{Id: proto.Uint32(20032), Count: proto.Uint32(1)},
		},
	})

	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result")
	}
	if got := readGuildCoinAmount(t, commanderID); got != 40 {
		t.Fatalf("expected guild coins unchanged, got %d", got)
	}
	if got := readGuildShopSlotCount(t, commanderID, 9); got != 3 {
		t.Fatalf("expected slot stock unchanged, got %d", got)
	}
}

func TestGuildShopPurchaseRejectsStaleSlotGoodsID(t *testing.T) {
	commanderID := uint32(7103)
	client := setupGuildShopPurchaseClient(t, commanderID, 100)
	defer cleanupGuildShopData(t, commanderID)

	seedGuildShopPurchaseEntry(t, guildStoreEntry{ID: 3003, Price: 10, Goods: []uint32{20004}, GoodsType: 2, Num: 1, Type: consts.DROP_TYPE_ITEM})
	seedGuildShopSlot(t, commanderID, 3, 3999, 2)

	response := sendGuildShopPurchase(t, client, &protobuf.CS_60035{
		Goodsid: proto.Uint32(3003),
		Index:   proto.Uint32(3),
		Selected: []*protobuf.GUILD_SHOP_INFO{
			{Id: proto.Uint32(20004), Count: proto.Uint32(1)},
		},
	})

	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result")
	}
	if got := readGuildCoinAmount(t, commanderID); got != 100 {
		t.Fatalf("expected guild coins unchanged, got %d", got)
	}
	if got := readGuildShopSlotCount(t, commanderID, 3); got != 2 {
		t.Fatalf("expected slot stock unchanged, got %d", got)
	}
}

func TestGuildShopPurchaseRejectsInvalidSelection(t *testing.T) {
	commanderID := uint32(7104)
	client := setupGuildShopPurchaseClient(t, commanderID, 100)
	defer cleanupGuildShopData(t, commanderID)

	seedGuildShopPurchaseEntry(t, guildStoreEntry{ID: 3004, Price: 5, Goods: []uint32{20005}, GoodsType: 2, Num: 1, Type: consts.DROP_TYPE_ITEM})
	seedGuildShopSlot(t, commanderID, 4, 3004, 4)

	response := sendGuildShopPurchase(t, client, &protobuf.CS_60035{
		Goodsid: proto.Uint32(3004),
		Index:   proto.Uint32(4),
		Selected: []*protobuf.GUILD_SHOP_INFO{
			{Id: proto.Uint32(99999), Count: proto.Uint32(1)},
		},
	})

	if response.GetResult() == 0 {
		t.Fatalf("expected non-zero result")
	}
	if got := readGuildCoinAmount(t, commanderID); got != 100 {
		t.Fatalf("expected guild coins unchanged, got %d", got)
	}
	if got := readGuildShopSlotCount(t, commanderID, 4); got != 4 {
		t.Fatalf("expected slot stock unchanged, got %d", got)
	}
}

func TestGuildShopPurchaseInsufficientCoinsDoesNotMutate(t *testing.T) {
	commanderID := uint32(7105)
	client := setupGuildShopPurchaseClient(t, commanderID, 9)
	defer cleanupGuildShopData(t, commanderID)

	seedGuildShopPurchaseEntry(t, guildStoreEntry{ID: 3005, Price: 10, Goods: []uint32{20006}, GoodsType: 2, Num: 1, Type: consts.DROP_TYPE_ITEM})
	seedGuildShopSlot(t, commanderID, 5, 3005, 3)

	response := sendGuildShopPurchase(t, client, &protobuf.CS_60035{
		Goodsid: proto.Uint32(3005),
		Index:   proto.Uint32(5),
		Selected: []*protobuf.GUILD_SHOP_INFO{
			{Id: proto.Uint32(20006), Count: proto.Uint32(1)},
		},
	})

	if response.GetResult() != 2 {
		t.Fatalf("expected result 2, got %d", response.GetResult())
	}
	if got := readGuildCoinAmount(t, commanderID); got != 9 {
		t.Fatalf("expected guild coins unchanged, got %d", got)
	}
	if got := readGuildShopSlotCount(t, commanderID, 5); got != 3 {
		t.Fatalf("expected slot stock unchanged, got %d", got)
	}
}

func TestGuildShopPurchaseInsufficientStockDoesNotMutate(t *testing.T) {
	commanderID := uint32(7106)
	client := setupGuildShopPurchaseClient(t, commanderID, 100)
	defer cleanupGuildShopData(t, commanderID)

	seedGuildShopPurchaseEntry(t, guildStoreEntry{ID: 3006, Price: 3, Goods: []uint32{20007}, GoodsType: 2, Num: 1, Type: consts.DROP_TYPE_ITEM})
	seedGuildShopSlot(t, commanderID, 6, 3006, 1)

	response := sendGuildShopPurchase(t, client, &protobuf.CS_60035{
		Goodsid: proto.Uint32(3006),
		Index:   proto.Uint32(6),
		Selected: []*protobuf.GUILD_SHOP_INFO{
			{Id: proto.Uint32(20007), Count: proto.Uint32(2)},
		},
	})

	if response.GetResult() != 3 {
		t.Fatalf("expected result 3, got %d", response.GetResult())
	}
	if got := readGuildCoinAmount(t, commanderID); got != 100 {
		t.Fatalf("expected guild coins unchanged, got %d", got)
	}
	if got := readGuildShopSlotCount(t, commanderID, 6); got != 1 {
		t.Fatalf("expected slot stock unchanged, got %d", got)
	}
}

func TestGuildShopPurchaseUnsupportedDropTypeRollsBack(t *testing.T) {
	commanderID := uint32(7107)
	client := setupGuildShopPurchaseClient(t, commanderID, 100)
	defer cleanupGuildShopData(t, commanderID)

	seedGuildShopPurchaseEntry(t, guildStoreEntry{ID: 3007, Price: 7, Goods: []uint32{20008}, GoodsType: 2, Num: 1, Type: 9})
	seedGuildShopSlot(t, commanderID, 7, 3007, 3)

	response := sendGuildShopPurchase(t, client, &protobuf.CS_60035{
		Goodsid: proto.Uint32(3007),
		Index:   proto.Uint32(7),
		Selected: []*protobuf.GUILD_SHOP_INFO{
			{Id: proto.Uint32(20008), Count: proto.Uint32(1)},
		},
	})

	if response.GetResult() != 4 {
		t.Fatalf("expected result 4, got %d", response.GetResult())
	}
	if got := readGuildCoinAmount(t, commanderID); got != 100 {
		t.Fatalf("expected guild coins unchanged, got %d", got)
	}
	if got := readGuildShopSlotCount(t, commanderID, 7); got != 3 {
		t.Fatalf("expected slot stock unchanged, got %d", got)
	}
}
