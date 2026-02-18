package answer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/connection"
	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func setupCommanderPacketClient(t *testing.T) *connection.Client {
	t.Helper()
	t.Setenv("MODE", "test")
	orm.InitDatabase()

	clearTable(t, &orm.CommanderPrefabFleet{})
	clearTable(t, &orm.CommanderPacketState{})
	clearTable(t, &orm.ConfigEntry{})
	clearTable(t, &orm.OwnedResource{})
	clearTable(t, &orm.Resource{})
	clearTable(t, &orm.Commander{})

	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO resources (id, item_id, name)
VALUES (1, 1, 'Gold')
ON CONFLICT (id) DO NOTHING
`); err != nil {
		t.Fatalf("seed gold resource: %v", err)
	}

	commanderID := uint32(7101)
	if err := orm.CreateCommanderRoot(commanderID, commanderID, "Packet Tester", 0, 0); err != nil {
		t.Fatalf("create commander root: %v", err)
	}
	commander := &orm.Commander{CommanderID: commanderID}
	if err := commander.Load(); err != nil {
		t.Fatalf("load commander: %v", err)
	}
	if err := commander.SetResource(1, 1000); err != nil {
		t.Fatalf("seed commander gold: %v", err)
	}

	seedCommanderPacketConfig(t)

	return &connection.Client{Commander: commander}
}

func seedCommanderPacketConfig(t *testing.T) {
	t.Helper()
	seed := func(category string, key string, payload string) {
		t.Helper()
		if err := orm.UpsertConfigEntry(category, key, json.RawMessage(payload)); err != nil {
			t.Fatalf("seed config %s/%s: %v", category, key, err)
		}
	}

	seed("ShareCfg/commander_ability_group.json", "1", `{"id":1,"ability_list":[1001,1002,1003]}`)
	seed("ShareCfg/commander_ability_template.json", "1001", `{"id":1001,"group_id":1,"cost":0,"worth":1,"next":1002}`)
	seed("ShareCfg/commander_ability_template.json", "1002", `{"id":1002,"group_id":1,"cost":200,"worth":2,"next":1003}`)
	seed("ShareCfg/commander_ability_template.json", "1003", `{"id":1003,"group_id":1,"cost":400,"worth":3,"next":0}`)

	seed("ShareCfg/gameset.json", "commander_rename_coldtime", `{"key_value":600,"description":""}`)
	seed("ShareCfg/gameset.json", "commander_ability_reset_coldtime", `{"key_value":300,"description":""}`)
	seed("ShareCfg/gameset.json", "commander_skill_reset_cost", `{"key_value":0,"description":[[300,600,900]]}`)
}

func seedCommanderPacketState(t *testing.T, ownerID uint32, commanderID uint32, state *orm.CommanderPacketState) {
	t.Helper()
	if state == nil {
		state = &orm.CommanderPacketState{}
	}
	state.OwnerCommanderID = ownerID
	state.CommanderID = commanderID
	if state.Level == 0 {
		state.Level = 1
	}
	if state.Name == "" {
		state.Name = "Commander"
	}
	if state.AbilityResetAt.IsZero() {
		state.AbilityResetAt = time.Unix(0, 0).UTC()
	}
	if state.RenameCooldownAt.IsZero() {
		state.RenameCooldownAt = time.Unix(0, 0).UTC()
	}
	if err := orm.SaveCommanderPacketState(state); err != nil {
		t.Fatalf("save commander packet state: %v", err)
	}
}

func marshalPacket[T proto.Message](t *testing.T, message T) []byte {
	t.Helper()
	raw, err := proto.Marshal(message)
	if err != nil {
		t.Fatalf("marshal packet: %v", err)
	}
	return raw
}

func TestFetchCommanderCandidateTalentsSuccess(t *testing.T) {
	client := setupCommanderPacketClient(t)
	seedCommanderPacketState(t, client.Commander.CommanderID, 10001, &orm.CommanderPacketState{
		AbilityIDs:       []uint32{1001},
		AbilityOriginIDs: []uint32{1001},
		UsedPt:           1,
	})

	buffer := marshalPacket(t, &protobuf.CS_25010{Commanderid: proto.Uint32(10001)})
	client.Buffer.Reset()
	if _, _, err := FetchCommanderCandidateTalents(&buffer, client); err != nil {
		t.Fatalf("fetch commander candidate talents: %v", err)
	}

	response := protobuf.SC_25011{}
	decodeResponse(t, client, &response)
	if response.GetResult() != commanderPacketResultOK {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}
	if len(response.GetAbilityid()) != 1 || response.GetAbilityid()[0] != 1002 {
		t.Fatalf("expected candidate [1002], got %+v", response.GetAbilityid())
	}

	stored, err := orm.GetCommanderPacketState(client.Commander.CommanderID, 10001)
	if err != nil {
		t.Fatalf("load commander packet state: %v", err)
	}
	if len(stored.PendingAbilityIDs) != 1 || stored.PendingAbilityIDs[0] != 1002 {
		t.Fatalf("expected pending [1002], got %+v", stored.PendingAbilityIDs)
	}
}

func TestFetchCommanderCandidateTalentsRejectsWhenPendingExists(t *testing.T) {
	client := setupCommanderPacketClient(t)
	seedCommanderPacketState(t, client.Commander.CommanderID, 10002, &orm.CommanderPacketState{
		AbilityIDs:        []uint32{1001},
		AbilityOriginIDs:  []uint32{1001},
		PendingAbilityIDs: []uint32{1002},
		UsedPt:            1,
	})

	buffer := marshalPacket(t, &protobuf.CS_25010{Commanderid: proto.Uint32(10002)})
	client.Buffer.Reset()
	if _, _, err := FetchCommanderCandidateTalents(&buffer, client); err != nil {
		t.Fatalf("fetch commander candidate talents: %v", err)
	}

	response := protobuf.SC_25011{}
	decodeResponse(t, client, &response)
	if response.GetResult() == commanderPacketResultOK {
		t.Fatalf("expected ineligible refresh failure")
	}

	stored, err := orm.GetCommanderPacketState(client.Commander.CommanderID, 10002)
	if err != nil {
		t.Fatalf("load commander packet state: %v", err)
	}
	if len(stored.PendingAbilityIDs) != 1 || stored.PendingAbilityIDs[0] != 1002 {
		t.Fatalf("expected pending list to remain unchanged, got %+v", stored.PendingAbilityIDs)
	}
}

func TestLearnCommanderTalentSuccess(t *testing.T) {
	client := setupCommanderPacketClient(t)
	seedCommanderPacketState(t, client.Commander.CommanderID, 10003, &orm.CommanderPacketState{
		AbilityIDs:        []uint32{1001},
		AbilityOriginIDs:  []uint32{1001},
		PendingAbilityIDs: []uint32{1002},
		UsedPt:            1,
	})

	buffer := marshalPacket(t, &protobuf.CS_25012{
		Commanderid: proto.Uint32(10003),
		Targetid:    proto.Uint32(1002),
		Replaceid:   proto.Uint32(1001),
	})
	client.Buffer.Reset()
	if _, _, err := LearnCommanderTalent(&buffer, client); err != nil {
		t.Fatalf("learn commander talent: %v", err)
	}

	response := protobuf.SC_25013{}
	decodeResponse(t, client, &response)
	if response.GetResult() != commanderPacketResultOK {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}

	stored, err := orm.GetCommanderPacketState(client.Commander.CommanderID, 10003)
	if err != nil {
		t.Fatalf("load commander packet state: %v", err)
	}
	if len(stored.AbilityIDs) != 1 || stored.AbilityIDs[0] != 1002 {
		t.Fatalf("expected commander ability replaced with 1002, got %+v", stored.AbilityIDs)
	}
	if stored.UsedPt != 3 {
		t.Fatalf("expected used pt 3, got %d", stored.UsedPt)
	}
	if len(stored.PendingAbilityIDs) != 0 {
		t.Fatalf("expected pending ability list cleared")
	}
	if got := client.Commander.GetResourceCount(1); got != 800 {
		t.Fatalf("expected gold 800 after talent cost, got %d", got)
	}
}

func TestLearnCommanderTalentRejectsInvalidReplaceID(t *testing.T) {
	client := setupCommanderPacketClient(t)
	seedCommanderPacketState(t, client.Commander.CommanderID, 10004, &orm.CommanderPacketState{
		AbilityIDs:        []uint32{1001},
		AbilityOriginIDs:  []uint32{1001},
		PendingAbilityIDs: []uint32{1002},
		UsedPt:            1,
	})

	buffer := marshalPacket(t, &protobuf.CS_25012{
		Commanderid: proto.Uint32(10004),
		Targetid:    proto.Uint32(1002),
		Replaceid:   proto.Uint32(9999),
	})
	client.Buffer.Reset()
	if _, _, err := LearnCommanderTalent(&buffer, client); err != nil {
		t.Fatalf("learn commander talent: %v", err)
	}

	response := protobuf.SC_25013{}
	decodeResponse(t, client, &response)
	if response.GetResult() == commanderPacketResultOK {
		t.Fatalf("expected invalid replace id failure")
	}

	stored, err := orm.GetCommanderPacketState(client.Commander.CommanderID, 10004)
	if err != nil {
		t.Fatalf("load commander packet state: %v", err)
	}
	if len(stored.AbilityIDs) != 1 || stored.AbilityIDs[0] != 1001 {
		t.Fatalf("expected abilities unchanged, got %+v", stored.AbilityIDs)
	}
	if stored.UsedPt != 1 {
		t.Fatalf("expected used pt unchanged, got %d", stored.UsedPt)
	}
}

func TestResetCommanderTalentsSuccess(t *testing.T) {
	client := setupCommanderPacketClient(t)
	seedCommanderPacketState(t, client.Commander.CommanderID, 10005, &orm.CommanderPacketState{
		AbilityIDs:       []uint32{1002},
		AbilityOriginIDs: []uint32{1001},
		UsedPt:           2,
		AbilityResetAt:   time.Unix(0, 0).UTC(),
	})

	buffer := marshalPacket(t, &protobuf.CS_25014{Commanderid: proto.Uint32(10005)})
	client.Buffer.Reset()
	if _, _, err := ResetCommanderTalents(&buffer, client); err != nil {
		t.Fatalf("reset commander talents: %v", err)
	}

	response := protobuf.SC_25015{}
	decodeResponse(t, client, &response)
	if response.GetResult() != commanderPacketResultOK {
		t.Fatalf("expected success result, got %d", response.GetResult())
	}

	stored, err := orm.GetCommanderPacketState(client.Commander.CommanderID, 10005)
	if err != nil {
		t.Fatalf("load commander packet state: %v", err)
	}
	if len(stored.AbilityIDs) != 1 || stored.AbilityIDs[0] != 1001 {
		t.Fatalf("expected ability reset to origin, got %+v", stored.AbilityIDs)
	}
	if stored.UsedPt != 0 {
		t.Fatalf("expected used pt reset to 0, got %d", stored.UsedPt)
	}
	if got := client.Commander.GetResourceCount(1); got != 400 {
		t.Fatalf("expected gold 400 after reset cost, got %d", got)
	}
}

func TestResetCommanderTalentsRejectsCooldown(t *testing.T) {
	client := setupCommanderPacketClient(t)
	seedCommanderPacketState(t, client.Commander.CommanderID, 10006, &orm.CommanderPacketState{
		AbilityIDs:       []uint32{1002},
		AbilityOriginIDs: []uint32{1001},
		UsedPt:           2,
		AbilityResetAt:   time.Now().UTC(),
	})

	buffer := marshalPacket(t, &protobuf.CS_25014{Commanderid: proto.Uint32(10006)})
	client.Buffer.Reset()
	if _, _, err := ResetCommanderTalents(&buffer, client); err != nil {
		t.Fatalf("reset commander talents: %v", err)
	}

	response := protobuf.SC_25015{}
	decodeResponse(t, client, &response)
	if response.GetResult() == commanderPacketResultOK {
		t.Fatalf("expected cooldown failure result")
	}
}

func TestSetCommanderLockStateValidationAndPersistence(t *testing.T) {
	client := setupCommanderPacketClient(t)
	seedCommanderPacketState(t, client.Commander.CommanderID, 10007, &orm.CommanderPacketState{})

	invalid := marshalPacket(t, &protobuf.CS_25016{Commanderid: proto.Uint32(10007), Flag: proto.Uint32(2)})
	client.Buffer.Reset()
	if _, _, err := SetCommanderLockState(&invalid, client); err != nil {
		t.Fatalf("set commander lock invalid: %v", err)
	}
	invalidResponse := protobuf.SC_25017{}
	decodeResponse(t, client, &invalidResponse)
	if invalidResponse.GetResult() == commanderPacketResultOK {
		t.Fatalf("expected invalid flag to fail")
	}

	valid := marshalPacket(t, &protobuf.CS_25016{Commanderid: proto.Uint32(10007), Flag: proto.Uint32(1)})
	client.Buffer.Reset()
	if _, _, err := SetCommanderLockState(&valid, client); err != nil {
		t.Fatalf("set commander lock valid: %v", err)
	}
	validResponse := protobuf.SC_25017{}
	decodeResponse(t, client, &validResponse)
	if validResponse.GetResult() != commanderPacketResultOK {
		t.Fatalf("expected successful lock update")
	}

	stored, err := orm.GetCommanderPacketState(client.Commander.CommanderID, 10007)
	if err != nil {
		t.Fatalf("load commander packet state: %v", err)
	}
	if !stored.IsLocked {
		t.Fatalf("expected commander to be locked")
	}
}

func TestSetCommanderLockStateCreatesStateOnDemand(t *testing.T) {
	client := setupCommanderPacketClient(t)

	req := marshalPacket(t, &protobuf.CS_25016{Commanderid: proto.Uint32(12001), Flag: proto.Uint32(1)})
	client.Buffer.Reset()
	if _, _, err := SetCommanderLockState(&req, client); err != nil {
		t.Fatalf("set commander lock: %v", err)
	}
	response := protobuf.SC_25017{}
	decodeResponse(t, client, &response)
	if response.GetResult() != commanderPacketResultOK {
		t.Fatalf("expected lock update to succeed for new packet state")
	}

	stored, err := orm.GetCommanderPacketState(client.Commander.CommanderID, 12001)
	if err != nil {
		t.Fatalf("load commander packet state: %v", err)
	}
	if !stored.IsLocked {
		t.Fatalf("expected newly created packet state to persist lock flag")
	}
}

func TestRenameCommanderRespectsCooldown(t *testing.T) {
	client := setupCommanderPacketClient(t)
	seedCommanderPacketState(t, client.Commander.CommanderID, 10008, &orm.CommanderPacketState{Name: "Alpha"})

	first := marshalPacket(t, &protobuf.CS_25020{Commanderid: proto.Uint32(10008), Name: proto.String("Beta")})
	client.Buffer.Reset()
	if _, _, err := RenameCommander(&first, client); err != nil {
		t.Fatalf("rename commander first: %v", err)
	}
	firstResp := protobuf.SC_25021{}
	decodeResponse(t, client, &firstResp)
	if firstResp.GetResult() != commanderPacketResultOK {
		t.Fatalf("expected rename success, got %d", firstResp.GetResult())
	}

	second := marshalPacket(t, &protobuf.CS_25020{Commanderid: proto.Uint32(10008), Name: proto.String("Gamma")})
	client.Buffer.Reset()
	if _, _, err := RenameCommander(&second, client); err != nil {
		t.Fatalf("rename commander second: %v", err)
	}
	secondResp := protobuf.SC_25021{}
	decodeResponse(t, client, &secondResp)
	if secondResp.GetResult() == commanderPacketResultOK {
		t.Fatalf("expected rename cooldown failure")
	}
}

func TestSetCommanderPrefabFleetAndRename(t *testing.T) {
	client := setupCommanderPacketClient(t)
	seedCommanderPacketState(t, client.Commander.CommanderID, 11001, &orm.CommanderPacketState{Name: "One"})
	seedCommanderPacketState(t, client.Commander.CommanderID, 11002, &orm.CommanderPacketState{Name: "Two"})

	setReq := marshalPacket(t, &protobuf.CS_25022{
		Id: proto.Uint32(1),
		Commandersid: []*protobuf.COMMANDERSINFO{
			{Pos: proto.Uint32(1), Id: proto.Uint32(11001)},
			{Pos: proto.Uint32(2), Id: proto.Uint32(0)},
		},
	})
	client.Buffer.Reset()
	if _, _, err := SetCommanderPrefabFleet(&setReq, client); err != nil {
		t.Fatalf("set commander prefab fleet: %v", err)
	}
	setResp := protobuf.SC_25023{}
	decodeResponse(t, client, &setResp)
	if setResp.GetResult() != commanderPacketResultOK {
		t.Fatalf("expected prefab set success, got %d", setResp.GetResult())
	}

	prefab, err := orm.GetCommanderPrefabFleet(client.Commander.CommanderID, 1)
	if err != nil {
		t.Fatalf("load commander prefab fleet: %v", err)
	}
	if len(prefab.CommanderSlots) != 2 {
		t.Fatalf("expected two prefab slots, got %d", len(prefab.CommanderSlots))
	}

	renameReq := marshalPacket(t, &protobuf.CS_25024{Id: proto.Uint32(1), Name: proto.String("Strike")})
	client.Buffer.Reset()
	if _, _, err := RenameCommanderPrefabFleet(&renameReq, client); err != nil {
		t.Fatalf("rename commander prefab fleet: %v", err)
	}
	renameResp := protobuf.SC_25025{}
	decodeResponse(t, client, &renameResp)
	if renameResp.GetResult() != commanderPacketResultOK {
		t.Fatalf("expected prefab rename success, got %d", renameResp.GetResult())
	}

	reloaded, err := orm.GetCommanderPrefabFleet(client.Commander.CommanderID, 1)
	if err != nil {
		t.Fatalf("reload commander prefab fleet: %v", err)
	}
	if reloaded.Name != "Strike" {
		t.Fatalf("expected renamed prefab fleet, got %q", reloaded.Name)
	}

	invalidSetReq := marshalPacket(t, &protobuf.CS_25022{
		Id: proto.Uint32(1),
		Commandersid: []*protobuf.COMMANDERSINFO{
			{Pos: proto.Uint32(1), Id: proto.Uint32(0)},
			{Pos: proto.Uint32(2), Id: proto.Uint32(0)},
		},
	})
	client.Buffer.Reset()
	if _, _, err := SetCommanderPrefabFleet(&invalidSetReq, client); err != nil {
		t.Fatalf("set invalid commander prefab fleet: %v", err)
	}
	invalidSetResp := protobuf.SC_25023{}
	decodeResponse(t, client, &invalidSetResp)
	if invalidSetResp.GetResult() == commanderPacketResultOK {
		t.Fatalf("expected all-zero prefab payload to fail")
	}
}
