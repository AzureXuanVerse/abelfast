package medalshop

import (
	"testing"
	"time"
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
	if reset <= uint32(now.Unix()) {
		t.Fatalf("expected reset in the future")
	}
	resetAt := time.Unix(int64(reset), 0).UTC()
	if resetAt.Day() != 1 {
		t.Fatalf("expected reset to land on first day of month, got %v", resetAt)
	}
}
