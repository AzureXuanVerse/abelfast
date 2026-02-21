package answer

import (
	"errors"
	"sync"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

type ChargeSuccessEvent struct {
	ShopID  uint32
	PayID   string
	Gem     uint32
	GemFree uint32
}

var (
	chargeSuccessDedupMu  sync.Mutex
	processedChargeEvents = map[uint32]map[string]struct{}{}
)

func ApplyChargeSuccessEvent(commander *orm.Commander, client *connection.Client, event ChargeSuccessEvent) error {
	if commander == nil {
		return errors.New("missing commander")
	}
	if event.ShopID == 0 || event.PayID == "" {
		return errors.New("invalid charge success event")
	}
	if wasProcessedChargeEvent(commander.CommanderID, event.PayID) {
		return nil
	}

	if event.Gem > 0 {
		if err := commander.AddResource(4, event.Gem); err != nil {
			return err
		}
	}
	if event.GemFree > 0 {
		if err := commander.AddResource(14, event.GemFree); err != nil {
			return err
		}
	}
	markChargeEventProcessed(commander.CommanderID, event.PayID)

	if client == nil {
		return nil
	}
	response := protobuf.SC_11503{
		ShopId:  proto.Uint32(event.ShopID),
		PayId:   proto.String(event.PayID),
		Gem:     proto.Uint32(event.Gem),
		GemFree: proto.Uint32(event.GemFree),
	}
	_, _, err := client.SendMessage(11503, &response)
	return err
}

func wasProcessedChargeEvent(commanderID uint32, payID string) bool {
	chargeSuccessDedupMu.Lock()
	defer chargeSuccessDedupMu.Unlock()
	entries, ok := processedChargeEvents[commanderID]
	if !ok {
		return false
	}
	_, ok = entries[payID]
	return ok
}

func markChargeEventProcessed(commanderID uint32, payID string) {
	chargeSuccessDedupMu.Lock()
	defer chargeSuccessDedupMu.Unlock()
	entries, ok := processedChargeEvents[commanderID]
	if !ok {
		entries = map[string]struct{}{}
		processedChargeEvents[commanderID] = entries
	}
	entries[payID] = struct{}{}
}

func resetChargeSuccessDedupForTest() {
	chargeSuccessDedupMu.Lock()
	defer chargeSuccessDedupMu.Unlock()
	processedChargeEvents = map[uint32]map[string]struct{}{}
}
