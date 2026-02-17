package answer

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
)

func TestIslandOpsTransferOverflowItemsSuccess(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandOverflowInventory{})
	clearTable(t, &orm.IslandInventory{})

	if err := orm.UpsertIslandOverflowInventory(client.Commander.CommanderID, 5001, 12); err != nil {
		t.Fatalf("seed overflow: %v", err)
	}

	payload := protobuf.CS_21006{Type: proto.Uint32(0)}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := IslandTransferOverflowItems(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_21007
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 || len(response.GetItemList()) != 1 || response.GetItemList()[0].GetId() != 5001 {
		t.Fatalf("unexpected response: %+v", response)
	}

	item, err := orm.GetIslandInventoryItem(client.Commander.CommanderID, 5001)
	if err != nil || item.Count != 12 {
		t.Fatalf("expected transferred inventory, err=%v item=%+v", err, item)
	}
}

func TestIslandOpsRemoveExpiredTicketSuccess(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandSpeedupTicket{})

	if err := orm.UpsertIslandSpeedupTicket(client.Commander.CommanderID, 1001, 200, 4); err != nil {
		t.Fatalf("seed speedup ticket: %v", err)
	}

	payload := protobuf.CS_21425{TicketKeys: []*protobuf.PB_SPEEDUP_KEY{{SpeedId: proto.Uint32(1001), EndTime: proto.Uint32(200)}}}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := IslandRemoveExpiredTicket(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_21426
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}
	tickets, err := orm.ListIslandSpeedupTickets(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("list tickets: %v", err)
	}
	if len(tickets) != 0 {
		t.Fatalf("expected ticket deletion")
	}
}

func TestIslandOpsUseTicketConsumesAndReducesTarget(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandSpeedupTicket{})
	clearTable(t, &orm.IslandSpeedupTarget{})
	seedConfigEntry(t, islandSpeedupTicketCategory, "1001", `{"id":1001,"speedup_time":30}`)

	now := nowUnix()
	if err := orm.UpsertIslandSpeedupTicket(client.Commander.CommanderID, 1001, now+3600, 3); err != nil {
		t.Fatalf("seed speedup ticket: %v", err)
	}
	if err := orm.UpsertIslandSpeedupTarget(client.Commander.CommanderID, islandTicketTypeOrderCD, 9001, now+300); err != nil {
		t.Fatalf("seed speedup target: %v", err)
	}

	payload := protobuf.CS_21423{
		Type:     proto.Uint32(islandTicketTypeOrderCD),
		TargetId: proto.Uint32(9001),
		Tickets: []*protobuf.PB_SPEEDUP_TICKET{{
			Key: &protobuf.PB_SPEEDUP_KEY{SpeedId: proto.Uint32(1001), EndTime: proto.Uint32(now + 3600)},
			Num: proto.Uint32(2),
		}},
	}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := IslandUseTicket(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_21424
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}
	tickets, err := orm.ListIslandSpeedupTickets(client.Commander.CommanderID)
	if err != nil {
		t.Fatalf("list tickets: %v", err)
	}
	if len(tickets) != 1 || tickets[0].Count != 1 {
		t.Fatalf("expected ticket decrement, got %+v", tickets)
	}
}

func TestIslandOpsUseDelegationTicketUpdatesTimeline(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandDelegation{})
	clearTable(t, &orm.IslandSpeedupTicket{})
	seedConfigEntry(t, islandSpeedupTicketCategory, "1001", `{"id":1001,"speedup_time":60}`)

	now := nowUnix()
	seedIslandDelegation(t, client.Commander.CommanderID, orm.IslandDelegation{
		BuildID:      10101,
		AreaID:       301,
		ShipID:       1,
		HasRole:      true,
		FormulaID:    100,
		CostTimeList: []uint32{now + 300},
	})
	if err := orm.UpsertIslandSpeedupTicket(client.Commander.CommanderID, 1001, now+3600, 2); err != nil {
		t.Fatalf("seed speedup ticket: %v", err)
	}

	payload := protobuf.CS_21427{
		AreaId: proto.Uint32(301),
		Tickets: []*protobuf.PB_SPEEDUP_TICKET{{
			Key: &protobuf.PB_SPEEDUP_KEY{SpeedId: proto.Uint32(1001), EndTime: proto.Uint32(now + 3600)},
			Num: proto.Uint32(1),
		}},
	}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := IslandUseDelegationTicket(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_21428
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 || len(response.GetTimeList()) != 1 || response.GetTimeList()[0] >= now+300 {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestIslandOpsShipOrderLoadUpCompletesSlot(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandInventory{})
	clearTable(t, &orm.IslandShipOrderSlot{})
	clearTable(t, &orm.ConfigEntry{})

	seedConfigEntry(t, islandSetCategory, "order_ship_award_coefficient", `{"key_value":[9001,25,0]}`)
	seedConfigEntry(t, islandItemTemplateCategory, "7001", `{"id":7001,"order_price":100}`)

	if err := orm.UpsertIslandShipOrderSlot(&orm.IslandShipOrderSlot{
		CommanderID: client.Commander.CommanderID,
		ShipSlotID:  91,
		State:       0,
		CostList:    []orm.IslandShipOrderCost{{ID: 7001, Num: 2, State: 0}},
	}); err != nil {
		t.Fatalf("seed ship order slot: %v", err)
	}
	if err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return orm.AddIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, 7001, 5)
	}); err != nil {
		t.Fatalf("seed inventory: %v", err)
	}

	payload := protobuf.CS_21416{ShipSlotId: proto.Uint32(91), ItemId: []uint32{7001}}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := IslandShipOrderLoadUp(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	var response protobuf.SC_21417
	decodeResponse(t, client, &response)
	if response.GetResult() != 0 || response.GetGetTime() == 0 {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestIslandOpsStartAndFinishDelegationFlow(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.IslandDelegation{})
	clearTable(t, &orm.IslandInventory{})

	templateID := uint32(202124)
	seedShipTemplate(t, templateID, 1, 5, 2, "Belfast", 6)
	ownedShip := seedOwnedShip(t, client, templateID)

	seedConfigEntry(t, islandFormulaCategory, "555", `{"id":555,"stamina_cost":10,"commission_cost":[[8001,2]],"duration":1}`)
	if err := db.DefaultStore.WithPGXTx(context.Background(), func(tx pgx.Tx) error {
		return orm.AddIslandInventoryTx(context.Background(), tx, client.Commander.CommanderID, 8001, 20)
	}); err != nil {
		t.Fatalf("seed island inventory: %v", err)
	}

	startPayload := protobuf.CS_21501{
		BuildId:   proto.Uint32(120),
		AreaId:    proto.Uint32(12),
		ShipId:    proto.Uint32(ownedShip.ID),
		FormulaId: proto.Uint32(555),
		Num:       proto.Uint32(2),
	}
	startBuffer, err := proto.Marshal(&startPayload)
	if err != nil {
		t.Fatalf("marshal start payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := IslandStartDelegation(&startBuffer, client); err != nil {
		t.Fatalf("start handler failed: %v", err)
	}

	var startResponse protobuf.SC_21502
	decodeResponse(t, client, &startResponse)
	if startResponse.GetResult() != 0 || startResponse.GetShipAppoint().GetShipId() != ownedShip.ID {
		t.Fatalf("unexpected start response: %+v", startResponse)
	}

	slot, err := orm.GetIslandDelegation(client.Commander.CommanderID, 120, 12)
	if err != nil {
		t.Fatalf("load slot: %v", err)
	}
	slot.CostTimeList = []uint32{nowUnix() - 1}
	if err := orm.UpsertIslandDelegation(slot); err != nil {
		t.Fatalf("force finished slot: %v", err)
	}

	finishPayload := protobuf.CS_21503{BuildId: proto.Uint32(120), AreaId: proto.Uint32(12)}
	finishBuffer, err := proto.Marshal(&finishPayload)
	if err != nil {
		t.Fatalf("marshal finish payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := IslandFinishDelegation(&finishBuffer, client); err != nil {
		t.Fatalf("finish handler failed: %v", err)
	}

	var finishResponse protobuf.SC_21504
	decodeResponse(t, client, &finishResponse)
	if finishResponse.GetResult() != 0 || len(finishResponse.GetAward()) == 0 {
		t.Fatalf("unexpected finish response: %+v", finishResponse)
	}

	reloaded, err := orm.GetIslandDelegation(client.Commander.CommanderID, 120, 12)
	if err != nil {
		t.Fatalf("reload slot: %v", err)
	}
	if reloaded.HasRole {
		t.Fatalf("expected role cleared after finish")
	}
}

func TestIslandOpsUseTicketRejectsExpiredLots(t *testing.T) {
	client := setupHandlerCommander(t)
	clearTable(t, &orm.IslandSpeedupTicket{})
	seedConfigEntry(t, islandSpeedupTicketCategory, "1002", `{"id":1002,"speedup_time":10}`)

	if err := orm.UpsertIslandSpeedupTicket(client.Commander.CommanderID, 1002, nowUnix()-5, 1); err != nil {
		t.Fatalf("seed speedup ticket: %v", err)
	}
	payload := protobuf.CS_21423{
		Type:     proto.Uint32(islandTicketTypeOrderCD),
		TargetId: proto.Uint32(1),
		Tickets: []*protobuf.PB_SPEEDUP_TICKET{{
			Key: &protobuf.PB_SPEEDUP_KEY{SpeedId: proto.Uint32(1002), EndTime: proto.Uint32(nowUnix() - 5)},
			Num: proto.Uint32(1),
		}},
	}
	buffer, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	client.Buffer.Reset()
	if _, _, err := IslandUseTicket(&buffer, client); err != nil {
		t.Fatalf("handler failed: %v", err)
	}
	var response protobuf.SC_21424
	decodeResponse(t, client, &response)
	if response.GetResult() != 3 {
		t.Fatalf("expected expired result code 3, got %d", response.GetResult())
	}
}

func TestIslandOpsHandlersSmoke(t *testing.T) {
	_ = fmt.Sprintf("%d", islandTicketTypeShipOrderReload)
}
