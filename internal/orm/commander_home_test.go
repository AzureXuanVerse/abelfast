package orm

import (
	"fmt"
	"testing"
	"time"
)

func TestCommanderHomeRoundTrip(t *testing.T) {
	initCommanderItemTestDB(t)
	clearTable(t, &CommanderHome{})
	clearTable(t, &CommanderHomeSlot{})
	clearTable(t, &Commander{})

	commanderID := uint32(time.Now().UnixNano())
	if err := CreateCommanderRoot(commanderID, commanderID, fmt.Sprintf("Commander Home %d", commanderID), 0, 0); err != nil {
		t.Fatalf("create commander: %v", err)
	}

	home, slots, err := EnsureCommanderHome(commanderID)
	if err != nil {
		t.Fatalf("ensure home: %v", err)
	}
	if len(slots) == 0 {
		t.Fatalf("expected default slots")
	}

	home.Clean = 7
	home.SceneOpen = true
	if err := UpdateCommanderHome(home); err != nil {
		t.Fatalf("update home: %v", err)
	}

	slots[0].OpFlag = CommanderClearCatteryOpFlag(slots[0].OpFlag, CommanderCatteryOpPlay)
	slots[0].ExpTime = 123
	slots[0].CacheExp = 88
	if err := UpdateCommanderHomeSlot(&slots[0]); err != nil {
		t.Fatalf("update slot: %v", err)
	}

	reloadedHome, reloadedSlots, err := GetCommanderHome(commanderID)
	if err != nil {
		t.Fatalf("reload home: %v", err)
	}
	if !reloadedHome.SceneOpen || reloadedHome.Clean != 7 {
		t.Fatalf("unexpected reloaded home: %+v", reloadedHome)
	}
	if reloadedSlots[0].ExpTime != 123 || reloadedSlots[0].CacheExp != 88 {
		t.Fatalf("unexpected slot persistence: %+v", reloadedSlots[0])
	}

	if err := ClearCommanderHomeCacheExp(commanderID); err != nil {
		t.Fatalf("clear cache exp: %v", err)
	}
	_, reloadedSlots, err = GetCommanderHome(commanderID)
	if err != nil {
		t.Fatalf("reload home after clear: %v", err)
	}
	if reloadedSlots[0].CacheExp != 0 {
		t.Fatalf("expected cache exp reset")
	}
}
