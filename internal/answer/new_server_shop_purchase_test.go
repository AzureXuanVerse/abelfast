package answer

import (
	"testing"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func TestNewServerShopPurchaseSuccessAndStateUpdate(t *testing.T) {
	client := setupNewServerShopTest(t)
	seedNewServerShopActivity(t, 30862, activityTypeNewServerShop, []uint32{303})
	seedNewServerShopEntry(t, 303, newServerShopGoodsTypeSelectable, 2, []uint32{111, 112})
	if err := client.Commander.SetResource(1, 100); err != nil {
		t.Fatalf("seed gold: %v", err)
	}

	request := &protobuf.CS_26043{
		ActId:   proto.Uint32(30862),
		Goodsid: proto.Uint32(303),
		Selected: []*protobuf.ACT_GOODS_BUY{
			{Itemid: proto.Uint32(111), Count: proto.Uint32(1)},
		},
	}
	buf, err := proto.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if _, _, err := NewServerShopPurchase(&buf, client); err != nil {
		t.Fatalf("NewServerShopPurchase: %v", err)
	}

	var resp protobuf.SC_26044
	decodePacketAt(t, client, 0, 26044, &resp)
	if resp.GetResult() != 0 {
		t.Fatalf("expected success, got %d", resp.GetResult())
	}
	if len(resp.GetDropList()) != 1 || resp.GetDropList()[0].GetId() != 111 {
		t.Fatalf("expected selected item drop")
	}

	state, err := orm.GetNewServerShopState(client.Commander.CommanderID, 30862)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.Goods[0].Count != 1 {
		t.Fatalf("expected remaining count 1, got %d", state.Goods[0].Count)
	}
	if len(state.Goods[0].BoughtRecord) != 1 || state.Goods[0].BoughtRecord[0] != 111 {
		t.Fatalf("expected bought record for selected item")
	}

	client.Buffer.Reset()
	getRequest := &protobuf.CS_26041{ActId: proto.Uint32(30862)}
	getBuf, err := proto.Marshal(getRequest)
	if err != nil {
		t.Fatalf("marshal get request: %v", err)
	}
	if _, _, err := GetNewServerShop(&getBuf, client); err != nil {
		t.Fatalf("GetNewServerShop: %v", err)
	}
	var getResp protobuf.SC_26042
	decodePacketAt(t, client, 0, 26042, &getResp)
	if getResp.GetGoods()[0].GetCount() != 1 {
		t.Fatalf("expected replayed count 1 from 26041")
	}
}

func TestNewServerShopPurchaseFailureScenarios(t *testing.T) {
	t.Run("insufficient currency", func(t *testing.T) {
		client := setupNewServerShopTest(t)
		seedNewServerShopActivity(t, 30862, activityTypeNewServerShop, []uint32{301})
		seedNewServerShopEntry(t, 301, newServerShopGoodsTypeFixed, 3, []uint32{20001})
		if err := client.Commander.SetResource(1, 1); err != nil {
			t.Fatalf("seed gold: %v", err)
		}

		request := &protobuf.CS_26043{ActId: proto.Uint32(30862), Goodsid: proto.Uint32(301)}
		buf, _ := proto.Marshal(request)
		if _, _, err := NewServerShopPurchase(&buf, client); err != nil {
			t.Fatalf("NewServerShopPurchase: %v", err)
		}

		var resp protobuf.SC_26044
		decodePacketAt(t, client, 0, 26044, &resp)
		if resp.GetResult() == 0 {
			t.Fatalf("expected non-zero result")
		}
		state, err := orm.GetNewServerShopState(client.Commander.CommanderID, 30862)
		if err != nil {
			if db.IsNotFound(err) {
				return
			}
			t.Fatalf("load state: %v", err)
		}
		if state.Goods[0].Count != 3 {
			t.Fatalf("expected shop state unchanged")
		}
	})

	t.Run("over limit", func(t *testing.T) {
		client := setupNewServerShopTest(t)
		seedNewServerShopActivity(t, 30862, activityTypeNewServerShop, []uint32{301})
		seedNewServerShopEntry(t, 301, newServerShopGoodsTypeFixed, 0, []uint32{20001})
		if err := client.Commander.SetResource(1, 100); err != nil {
			t.Fatalf("seed gold: %v", err)
		}

		request := &protobuf.CS_26043{ActId: proto.Uint32(30862), Goodsid: proto.Uint32(301)}
		buf, _ := proto.Marshal(request)
		if _, _, err := NewServerShopPurchase(&buf, client); err != nil {
			t.Fatalf("NewServerShopPurchase: %v", err)
		}

		var resp protobuf.SC_26044
		decodePacketAt(t, client, 0, 26044, &resp)
		if resp.GetResult() == 0 {
			t.Fatalf("expected limit failure")
		}
	})

	t.Run("invalid selected item", func(t *testing.T) {
		client := setupNewServerShopTest(t)
		seedNewServerShopActivity(t, 30862, activityTypeNewServerShop, []uint32{303})
		seedNewServerShopEntry(t, 303, newServerShopGoodsTypeSelectable, 2, []uint32{111, 112})
		if err := client.Commander.SetResource(1, 100); err != nil {
			t.Fatalf("seed gold: %v", err)
		}

		request := &protobuf.CS_26043{
			ActId:   proto.Uint32(30862),
			Goodsid: proto.Uint32(303),
			Selected: []*protobuf.ACT_GOODS_BUY{
				{Itemid: proto.Uint32(999), Count: proto.Uint32(1)},
			},
		}
		buf, _ := proto.Marshal(request)
		if _, _, err := NewServerShopPurchase(&buf, client); err != nil {
			t.Fatalf("NewServerShopPurchase: %v", err)
		}

		var resp protobuf.SC_26044
		decodePacketAt(t, client, 0, 26044, &resp)
		if resp.GetResult() == 0 {
			t.Fatalf("expected invalid selected item failure")
		}
	})

	t.Run("selectable replay denied", func(t *testing.T) {
		client := setupNewServerShopTest(t)
		seedNewServerShopActivity(t, 30862, activityTypeNewServerShop, []uint32{303})
		seedNewServerShopEntry(t, 303, newServerShopGoodsTypeSelectable, 2, []uint32{111, 112})
		if err := client.Commander.SetResource(1, 100); err != nil {
			t.Fatalf("seed gold: %v", err)
		}

		request := &protobuf.CS_26043{
			ActId:   proto.Uint32(30862),
			Goodsid: proto.Uint32(303),
			Selected: []*protobuf.ACT_GOODS_BUY{
				{Itemid: proto.Uint32(111), Count: proto.Uint32(1)},
			},
		}
		buf, _ := proto.Marshal(request)
		if _, _, err := NewServerShopPurchase(&buf, client); err != nil {
			t.Fatalf("first NewServerShopPurchase: %v", err)
		}
		client.Buffer.Reset()

		if _, _, err := NewServerShopPurchase(&buf, client); err != nil {
			t.Fatalf("second NewServerShopPurchase: %v", err)
		}
		var resp protobuf.SC_26044
		decodePacketAt(t, client, 0, 26044, &resp)
		if resp.GetResult() == 0 {
			t.Fatalf("expected replay failure")
		}
	})

	t.Run("transaction rollback on grant failure", func(t *testing.T) {
		client := setupNewServerShopTest(t)
		seedNewServerShopActivity(t, 30862, activityTypeNewServerShop, []uint32{305})
		seedConfigEntry(t, "ShareCfg/newserver_shop_template.json", "305", `{"id":305,"goods":[20001],"goods_purchase_limit":3,"goods_type":1,"num":1,"type":9999,"resource_category":1,"resource_type":1,"resource_num":10}`)
		if err := client.Commander.SetResource(1, 100); err != nil {
			t.Fatalf("seed gold: %v", err)
		}

		request := &protobuf.CS_26043{ActId: proto.Uint32(30862), Goodsid: proto.Uint32(305)}
		buf, _ := proto.Marshal(request)
		if _, _, err := NewServerShopPurchase(&buf, client); err != nil {
			t.Fatalf("NewServerShopPurchase: %v", err)
		}

		var resp protobuf.SC_26044
		decodePacketAt(t, client, 0, 26044, &resp)
		if resp.GetResult() == 0 {
			t.Fatalf("expected failure for unsupported drop type")
		}
		if len(resp.GetDropList()) != 0 {
			t.Fatalf("expected empty drop list on failure")
		}
		state, err := orm.GetNewServerShopState(client.Commander.CommanderID, 30862)
		if err != nil {
			if db.IsNotFound(err) {
				return
			}
			t.Fatalf("load state: %v", err)
		}
		if state.Goods[0].Count != 3 {
			t.Fatalf("expected count unchanged after rollback")
		}
	})
}
