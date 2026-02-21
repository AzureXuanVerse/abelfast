package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/consts"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestNormalizeGuildShopSelectionSelectableRejectsEmpty(t *testing.T) {
	config := &guildStorePurchaseEntry{Goods: []uint32{1001}, GoodsType: guildShopGoodsTypeSelectable}
	rewards, total, ok := normalizeGuildShopSelection(config, nil)
	if ok || rewards != nil || total != 0 {
		t.Fatalf("expected selectable empty selection to fail")
	}
}

func TestNormalizeGuildShopSelectionAggregatesDuplicates(t *testing.T) {
	config := &guildStorePurchaseEntry{Goods: []uint32{1001, 1002}, GoodsType: guildShopGoodsTypeSelectable}
	rewards, total, ok := normalizeGuildShopSelection(config, []*protobuf.GUILD_SHOP_INFO{
		{Id: proto.Uint32(1001), Count: proto.Uint32(1)},
		{Id: proto.Uint32(1001), Count: proto.Uint32(2)},
		{Id: proto.Uint32(1002), Count: proto.Uint32(3)},
	})
	if !ok {
		t.Fatalf("expected selection to be valid")
	}
	if total != 6 {
		t.Fatalf("expected total units 6, got %d", total)
	}
	if rewards[1001] != 3 || rewards[1002] != 3 {
		t.Fatalf("unexpected rewards map: %#v", rewards)
	}
}

func TestNormalizeGuildShopSelectionFixedNormalizesEmptySelection(t *testing.T) {
	config := &guildStorePurchaseEntry{Goods: []uint32{1001, 1002}, GoodsType: guildShopGoodsTypeFixed}
	rewards, total, ok := normalizeGuildShopSelection(config, nil)
	if !ok {
		t.Fatalf("expected fixed goods to accept empty selection")
	}
	if total != 1 || rewards[1001] != 1 || rewards[1002] != 1 {
		t.Fatalf("unexpected normalized selection total=%d rewards=%#v", total, rewards)
	}
}

func TestNormalizeGuildShopSelectionFixedRejectsSelectedPayload(t *testing.T) {
	config := &guildStorePurchaseEntry{Goods: []uint32{1001, 1002}, GoodsType: guildShopGoodsTypeFixed}
	rewards, total, ok := normalizeGuildShopSelection(config, []*protobuf.GUILD_SHOP_INFO{{Id: proto.Uint32(1001), Count: proto.Uint32(1)}})
	if ok || rewards != nil || total != 0 {
		t.Fatalf("expected fixed goods to reject selected payload")
	}
}

func TestMapGuildShopDropType(t *testing.T) {
	if dropType, ok := mapGuildShopDropType(consts.DROP_TYPE_ITEM); !ok || dropType != consts.DROP_TYPE_ITEM {
		t.Fatalf("expected item drop type mapping")
	}
	if dropType, ok := mapGuildShopDropType(consts.DROP_TYPE_SHIP); !ok || dropType != consts.DROP_TYPE_SHIP {
		t.Fatalf("expected ship drop type mapping")
	}
	if _, ok := mapGuildShopDropType(consts.DROP_TYPE_EQUIP); ok {
		t.Fatalf("expected unsupported drop type to fail")
	}
}
