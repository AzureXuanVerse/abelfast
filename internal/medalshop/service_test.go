package medalshop

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/shopreset"
)

func TestBuildGoods(t *testing.T) {
	config := &Config{GoodsIDs: []uint32{1, 2}, PurchaseLimit: map[uint32]uint32{2: 5}}
	goods := buildGoods(10, config)
	if len(goods) != 2 {
		t.Fatalf("expected 2 goods")
	}
	if goods[0].Count != 1 {
		t.Fatalf("expected default count 1")
	}
	if goods[1].Count != 5 {
		t.Fatalf("expected configured count 5")
	}
}

func TestNextDailyReset(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	reset := nextMonthlyReset(now)
	expected := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	if window, err := shopreset.MonthlyWindow(now); err == nil {
		expected = window.End
	}
	if reset != uint32(expected.Unix()) {
		t.Fatalf("expected reset to match monthly window end, got %v", time.Unix(int64(reset), 0).UTC())
	}
}

func TestNextDailyResetAtExactMonthBoundary(t *testing.T) {
	now := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	reset := nextMonthlyReset(now)
	if reset <= uint32(now.Unix()) {
		t.Fatalf("expected reset after boundary instant, got %v", time.Unix(int64(reset), 0).UTC())
	}
	expected := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if window, err := shopreset.MonthlyWindow(now); err == nil {
		expected = window.End
	}
	if reset != uint32(expected.Unix()) {
		t.Fatalf("expected reset to match next monthly window end, got %v", time.Unix(int64(reset), 0).UTC())
	}
}

func TestSelectMonthTemplateAcceptsEmptySingleTemplate(t *testing.T) {
	payload, err := json.Marshal(monthShopTemplate{ID: 5, HonorMedalShopGoods: []uint32{}})
	if err != nil {
		t.Fatalf("marshal template: %v", err)
	}

	goods, err := selectMonthTemplate([]orm.ConfigEntry{{Data: payload}}, time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("select month template: %v", err)
	}
	if goods == nil {
		t.Fatalf("expected empty list, got nil")
	}
	if len(goods) != 0 {
		t.Fatalf("expected empty goods list, got %d", len(goods))
	}
}
