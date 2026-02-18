package answer

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func setupNewServerShopTest(t *testing.T) *connection.Client {
	t.Helper()
	os.Setenv("MODE", "test")
	orm.InitDatabase()
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.NewServerShopState{})
	clearTable(t, &orm.CommanderItem{})
	clearTable(t, &orm.OwnedResource{})
	clearTable(t, &orm.Commander{})

	if err := orm.CreateCommanderRoot(1, 1, "new server shop tester", 0, 0); err != nil {
		t.Fatalf("create commander: %v", err)
	}
	commander := orm.Commander{CommanderID: 1}
	if err := commander.Load(); err != nil {
		t.Fatalf("load commander: %v", err)
	}
	return &connection.Client{Commander: &commander}
}

func seedNewServerShopActivity(t *testing.T, actID uint32, activityType uint32, goodsIDs []uint32) {
	t.Helper()
	goodsJSON, err := json.Marshal(goodsIDs)
	if err != nil {
		t.Fatalf("marshal goods ids: %v", err)
	}
	start := time.Now().UTC().Add(-time.Hour)
	stop := time.Now().UTC().Add(time.Hour)
	payload := fmt.Sprintf(`{"id":%d,"type":%d,"config_data":%s,"time":["timer",[[%d,%d,%d],[%d,%d,%d]],[[%d,%d,%d],[%d,%d,%d]],1,1]}`,
		actID,
		activityType,
		string(goodsJSON),
		start.Year(), int(start.Month()), start.Day(), start.Hour(), start.Minute(), start.Second(),
		stop.Year(), int(stop.Month()), stop.Day(), stop.Hour(), stop.Minute(), stop.Second(),
	)
	seedConfigEntry(t, "ShareCfg/activity_template.json", fmt.Sprintf("%d", actID), payload)
}

func seedNewServerShopEntry(t *testing.T, id uint32, goodsType uint32, purchaseLimit uint32, goods []uint32) {
	t.Helper()
	goodsJSON, err := json.Marshal(goods)
	if err != nil {
		t.Fatalf("marshal goods: %v", err)
	}
	payload := fmt.Sprintf(`{"id":%d,"goods":%s,"goods_purchase_limit":%d,"goods_type":%d,"num":1,"type":2,"resource_category":1,"resource_type":1,"resource_num":10}`,
		id,
		string(goodsJSON),
		purchaseLimit,
		goodsType,
	)
	seedConfigEntry(t, "ShareCfg/newserver_shop_template.json", fmt.Sprintf("%d", id), payload)
}

func TestGetNewServerShopSuccessAndReplayState(t *testing.T) {
	client := setupNewServerShopTest(t)
	seedNewServerShopActivity(t, 30862, activityTypeNewServerShop, []uint32{301, 303})
	seedNewServerShopEntry(t, 301, newServerShopGoodsTypeFixed, 5, []uint32{20001})
	seedNewServerShopEntry(t, 303, newServerShopGoodsTypeSelectable, 2, []uint32{111, 112})

	if err := orm.UpsertNewServerShopState(&orm.NewServerShopState{
		CommanderID: client.Commander.CommanderID,
		ActivityID:  30862,
		Goods: []orm.NewServerShopGoodsState{
			{ID: 301, Count: 4, BoughtRecord: []uint32{}},
			{ID: 303, Count: 1, BoughtRecord: []uint32{111}},
		},
	}); err != nil {
		t.Fatalf("seed shop state: %v", err)
	}

	request := &protobuf.CS_26041{ActId: proto.Uint32(30862)}
	buf, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, _, err := GetNewServerShop(&buf, client); err != nil {
		t.Fatalf("GetNewServerShop: %v", err)
	}

	var resp protobuf.SC_26042
	decodePacketAt(t, client, 0, 26042, &resp)
	if resp.GetResult() != 0 {
		t.Fatalf("expected result 0, got %d", resp.GetResult())
	}
	if resp.GetStartTime() == 0 || resp.GetStopTime() == 0 {
		t.Fatalf("expected non-zero activity window")
	}
	if len(resp.GetGoods()) != 2 {
		t.Fatalf("expected 2 goods entries, got %d", len(resp.GetGoods()))
	}
	if resp.GetGoods()[1].GetCount() != 1 || len(resp.GetGoods()[1].GetBoughtRecord()) != 1 || resp.GetGoods()[1].GetBoughtRecord()[0] != 111 {
		t.Fatalf("expected bought record replay for goods 303")
	}
}

func TestGetNewServerShopSupportsBlackFridayActivityType(t *testing.T) {
	client := setupNewServerShopTest(t)
	seedNewServerShopActivity(t, 30892, activityTypeBlackFridayShop, []uint32{301})
	seedNewServerShopEntry(t, 301, newServerShopGoodsTypeFixed, 5, []uint32{20001})

	request := &protobuf.CS_26041{ActId: proto.Uint32(30892)}
	buf, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, _, err := GetNewServerShop(&buf, client); err != nil {
		t.Fatalf("GetNewServerShop: %v", err)
	}

	var resp protobuf.SC_26042
	decodePacketAt(t, client, 0, 26042, &resp)
	if resp.GetResult() != 0 {
		t.Fatalf("expected success for black friday activity, got %d", resp.GetResult())
	}
}

func TestGetNewServerShopInvalidActivity(t *testing.T) {
	client := setupNewServerShopTest(t)
	request := &protobuf.CS_26041{ActId: proto.Uint32(99999)}
	buf, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, _, err := GetNewServerShop(&buf, client); err != nil {
		t.Fatalf("GetNewServerShop: %v", err)
	}

	var resp protobuf.SC_26042
	decodePacketAt(t, client, 0, 26042, &resp)
	if resp.GetResult() == 0 {
		t.Fatalf("expected non-zero result for invalid activity")
	}
}
