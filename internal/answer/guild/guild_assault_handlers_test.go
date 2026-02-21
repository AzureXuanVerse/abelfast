package guild

import (
	"testing"
	"time"

	"github.com/ggmolly/belfast/internal/db"
	"github.com/ggmolly/belfast/internal/orm"
	"github.com/ggmolly/belfast/internal/protobuf"
	"google.golang.org/protobuf/proto"
)

func seedGuildAssaultTestContext(t *testing.T, commanderID uint32, guildID uint32, operationID uint32) {
	t.Helper()
	execAnswerTestSQLT(t, "DELETE FROM guild_assault_recommendations")
	execAnswerTestSQLT(t, "DELETE FROM guild_assault_fleet_slots")
	execAnswerTestSQLT(t, "DELETE FROM guild_boss_mission_fleets")
	execAnswerTestSQLT(t, "DELETE FROM guild_operation_boss_ranks")
	execAnswerTestSQLT(t, "DELETE FROM guild_operation_boss_states")
	execAnswerTestSQLT(t, "DELETE FROM guild_operation_events")
	execAnswerTestSQLT(t, "DELETE FROM guild_operation_states")
	execAnswerTestSQLT(t, "DELETE FROM guild_members")
	execAnswerTestSQLT(t, "DELETE FROM guilds")
	execAnswerTestSQLT(t, "DELETE FROM owned_ships")
	execAnswerTestSQLT(t, "DELETE FROM ships")
	execAnswerTestSQLT(t, "DELETE FROM config_entries")

	seedConfigEntry(t, "ShareCfg/guildset.json", "operation_assault_team_cd", `{"key_value":1800}`)

	now := uint32(time.Now().Unix())
	execAnswerTestSQLT(t, "INSERT INTO guilds (id, policy, faction, name, level, announce, manifesto, exp, member_count, change_faction_cd, kick_leader_cd, capital, tech_id) VALUES ($1, 1, 1, 'GA', 1, '', '', 0, 1, 0, 0, 1000, 1)", int64(guildID))
	execAnswerTestSQLT(t, "INSERT INTO guild_members (guild_id, commander_id, duty, liveness, pre_online_time, join_time) VALUES ($1, $2, $3, 0, $4, $4)", int64(guildID), int64(commanderID), int64(orm.GuildDutyCommander), int64(now))
	execAnswerTestSQLT(t, "INSERT INTO guild_operation_states (guild_id, chapter_id, start_time, end_time) VALUES ($1, $2, $3, $4)", int64(guildID), int64(operationID), int64(now-60), int64(now+3600))
	execAnswerTestSQLT(t, "INSERT INTO guild_operation_events (guild_id, event_tid, position, start_time) VALUES ($1, $2, 1, $3)", int64(guildID), int64(operationID), int64(now-60))
}

func seedOwnedShipForGuildAssault(t *testing.T, ownerID uint32, ownedID uint32, templateID uint32) {
	t.Helper()
	execAnswerTestSQLT(t, "INSERT INTO ships (template_id, name, english_name, rarity_id, star, type, nationality, build_time) VALUES ($1, 'GA Ship', 'GA Ship', 1, 1, 1, 1, 1) ON CONFLICT (template_id) DO NOTHING", int64(templateID))
	execAnswerTestSQLT(t, "INSERT INTO owned_ships (owner_id, ship_id, id, level, max_level, exp, surplus_exp, energy, create_time, change_name_timestamp) VALUES ($1, $2, $3, 1, 125, 0, 0, 150, NOW(), NOW())", int64(ownerID), int64(templateID), int64(ownedID))
}

func TestGuildAssaultUpdateFetchAndRecommendFlow(t *testing.T) {
	client := setupConfigTest(t)
	seedGuildAssaultTestContext(t, client.Commander.CommanderID, 9101, 7201)

	seedOwnedShipForGuildAssault(t, client.Commander.CommanderID, 101, 1001)
	seedOwnedShipForGuildAssault(t, client.Commander.CommanderID, 102, 1002)
	client.Commander.OwnedShipsMap = map[uint32]*orm.OwnedShip{
		101: {ID: 101, OwnerID: client.Commander.CommanderID, ShipID: 1001},
		102: {ID: 102, OwnerID: client.Commander.CommanderID, ShipID: 1002},
	}

	client.Buffer.Reset()
	updatePayload := protobuf.CS_61003{ShipIds: []*protobuf.SHIPID_POS{{Pos: proto.Uint32(1), ShipId: proto.Uint32(101)}, {Pos: proto.Uint32(2), ShipId: proto.Uint32(102)}}}
	updateData, _ := proto.Marshal(&updatePayload)
	if _, _, err := GuildUpdateAssaultFleetCommandResponse(&updateData, client); err != nil {
		t.Fatalf("61003 failed: %v", err)
	}
	var updateResp protobuf.SC_61004
	decodeResponse(t, client, &updateResp)
	if updateResp.GetResult() != 0 {
		t.Fatalf("expected 61004 success, got %d", updateResp.GetResult())
	}

	client.Buffer.Reset()
	fetchMineData, _ := proto.Marshal(&protobuf.CS_61009{Type: proto.Uint32(0)})
	if _, _, err := GetMyAssaultFleetCommandResponse(&fetchMineData, client); err != nil {
		t.Fatalf("61009 failed: %v", err)
	}
	var mineResp protobuf.SC_61010
	decodeResponse(t, client, &mineResp)
	if mineResp.GetResult() != 0 || len(mineResp.GetPersonShips()) != 2 {
		t.Fatalf("expected 2 personal ships, got result=%d len=%d", mineResp.GetResult(), len(mineResp.GetPersonShips()))
	}

	client.Buffer.Reset()
	recommendData, _ := proto.Marshal(&protobuf.CS_61033{RecommendUid: proto.Uint32(client.Commander.CommanderID), RecommendShipid: proto.Uint32(101), Cmd: proto.Uint32(0)})
	if _, _, err := MarkAssaultShipRecommendCommandResponse(&recommendData, client); err != nil {
		t.Fatalf("61033 failed: %v", err)
	}
	var recommendResp protobuf.SC_61034
	decodeResponse(t, client, &recommendResp)
	if recommendResp.GetResult() != 0 {
		t.Fatalf("expected recommend success, got %d", recommendResp.GetResult())
	}

	client.Buffer.Reset()
	refreshData, _ := proto.Marshal(&protobuf.CS_61035{Type: proto.Uint32(0)})
	if _, _, err := GuildRefreshAssaultRecommendationsCommandResponse(&refreshData, client); err != nil {
		t.Fatalf("61035 failed: %v", err)
	}
	var refreshResp protobuf.SC_61036
	decodeResponse(t, client, &refreshResp)
	if len(refreshResp.GetRecommends()) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(refreshResp.GetRecommends()))
	}

	client.Buffer.Reset()
	fetchGuildData, _ := proto.Marshal(&protobuf.CS_61011{Type: proto.Uint32(0)})
	if _, _, err := GuildGetAssaultFleetCommandResponse(&fetchGuildData, client); err != nil {
		t.Fatalf("61011 failed: %v", err)
	}
	var guildResp protobuf.SC_61012
	decodeResponse(t, client, &guildResp)
	if guildResp.GetResult() != 0 {
		t.Fatalf("expected 61012 success, got %d", guildResp.GetResult())
	}
	if len(guildResp.GetShips()) == 0 || len(guildResp.GetRecommends()) != 1 {
		t.Fatalf("expected guild ships and recommendation in 61012")
	}
}

func TestGuildUpdateAssaultFleetRespectsCooldown(t *testing.T) {
	client := setupConfigTest(t)
	seedGuildAssaultTestContext(t, client.Commander.CommanderID, 9102, 7202)
	seedOwnedShipForGuildAssault(t, client.Commander.CommanderID, 101, 1001)
	seedOwnedShipForGuildAssault(t, client.Commander.CommanderID, 102, 1002)
	client.Commander.OwnedShipsMap = map[uint32]*orm.OwnedShip{
		101: {ID: 101, OwnerID: client.Commander.CommanderID, ShipID: 1001},
		102: {ID: 102, OwnerID: client.Commander.CommanderID, ShipID: 1002},
	}
	now := uint32(time.Now().Unix())
	execAnswerTestSQLT(t, "INSERT INTO guild_assault_fleet_slots (guild_id, commander_id, pos, ship_id, last_time) VALUES ($1, $2, 1, 101, $3)", int64(9102), int64(client.Commander.CommanderID), int64(now))

	client.Buffer.Reset()
	updateData, _ := proto.Marshal(&protobuf.CS_61003{ShipIds: []*protobuf.SHIPID_POS{{Pos: proto.Uint32(1), ShipId: proto.Uint32(102)}}})
	if _, _, err := GuildUpdateAssaultFleetCommandResponse(&updateData, client); err != nil {
		t.Fatalf("61003 failed: %v", err)
	}
	var updateResp protobuf.SC_61004
	decodeResponse(t, client, &updateResp)
	if updateResp.GetResult() == 0 {
		t.Fatalf("expected cooldown failure")
	}

	var storedShipID int64
	if err := db.DefaultStore.Pool.QueryRow(t.Context(), "SELECT ship_id FROM guild_assault_fleet_slots WHERE guild_id = $1 AND commander_id = $2 AND pos = 1", int64(9102), int64(client.Commander.CommanderID)).Scan(&storedShipID); err != nil {
		t.Fatalf("query slot failed: %v", err)
	}
	if storedShipID != 101 {
		t.Fatalf("expected slot unchanged, got %d", storedShipID)
	}
}

func TestGuildUpdateBossMissionFleetPersistsForActivationSnapshot(t *testing.T) {
	client := setupConfigTest(t)
	seedGuildAssaultTestContext(t, client.Commander.CommanderID, 9103, 7203)

	execAnswerTestSQLT(t, "INSERT INTO commanders (commander_id, account_id, name) VALUES (2, 2, 'Member Two') ON CONFLICT (commander_id) DO NOTHING")
	execAnswerTestSQLT(t, "INSERT INTO guild_members (guild_id, commander_id, duty, liveness, pre_online_time, join_time) VALUES ($1, 2, $2, 0, $3, $3)", int64(9103), int64(orm.GuildDutyOrdinary), int64(time.Now().Unix()))

	seedOwnedShipForGuildAssault(t, client.Commander.CommanderID, 101, 1001)
	seedOwnedShipForGuildAssault(t, 2, 201, 2001)
	execAnswerTestSQLT(t, "INSERT INTO guild_assault_fleet_slots (guild_id, commander_id, pos, ship_id, last_time) VALUES ($1, $2, 1, 101, 0)", int64(9103), int64(client.Commander.CommanderID))
	execAnswerTestSQLT(t, "INSERT INTO guild_assault_fleet_slots (guild_id, commander_id, pos, ship_id, last_time) VALUES ($1, 2, 1, 201, 0)", int64(9103))

	payload := &protobuf.CS_61013{Fleet: []*protobuf.BOSSEVENTFLEET{{
		FleetId: proto.Uint32(1),
		Ships: []*protobuf.TEAM_CELL{
			{UserId: proto.Uint32(client.Commander.CommanderID), ShipId: proto.Uint32(101)},
			{UserId: proto.Uint32(2), ShipId: proto.Uint32(201)},
		},
		Commanders: []*protobuf.COMMANDERSINFO{{Pos: proto.Uint32(1), Id: proto.Uint32(5001)}},
	}}}
	data, _ := proto.Marshal(payload)
	client.Buffer.Reset()
	if _, _, err := GuildUpdateBossMissionFleetCommandResponse(&data, client); err != nil {
		t.Fatalf("61013 failed: %v", err)
	}
	var updateResp protobuf.SC_61014
	decodeResponse(t, client, &updateResp)
	if updateResp.GetResult() != 0 {
		t.Fatalf("expected 61014 success, got %d", updateResp.GetResult())
	}

	client.Buffer.Reset()
	activationData, _ := proto.Marshal(&protobuf.CS_61005{Type: proto.Uint32(0)})
	if _, _, err := GuildGetActivationEventCommandResponse(&activationData, client); err != nil {
		t.Fatalf("61005 failed: %v", err)
	}
	var activationResp protobuf.SC_61006
	decodeResponse(t, client, &activationResp)
	if activationResp.GetResult() != 0 {
		t.Fatalf("expected activation snapshot success, got %d", activationResp.GetResult())
	}
	if len(activationResp.GetOperation().GetFleets()) != 1 {
		t.Fatalf("expected persisted boss fleet in operation snapshot")
	}
}

func TestMarkAssaultShipRecommendAllowsUnmarkForFormerMember(t *testing.T) {
	client := setupConfigTest(t)
	const guildID uint32 = 9104
	const operationID uint32 = 7204
	const formerCommanderID uint32 = 2
	const shipID uint32 = 301

	seedGuildAssaultTestContext(t, client.Commander.CommanderID, guildID, operationID)
	execAnswerTestSQLT(t, "INSERT INTO commanders (commander_id, account_id, name) VALUES ($1, $1, 'Former Member') ON CONFLICT (commander_id) DO NOTHING", int64(formerCommanderID))
	execAnswerTestSQLT(t, "INSERT INTO guild_members (guild_id, commander_id, duty, liveness, pre_online_time, join_time) VALUES ($1, $2, $3, 0, 0, 0)", int64(guildID), int64(formerCommanderID), int64(orm.GuildDutyOrdinary))
	execAnswerTestSQLT(t, "INSERT INTO guild_assault_recommendations (guild_id, commander_id, ship_id) VALUES ($1, $2, $3)", int64(guildID), int64(formerCommanderID), int64(shipID))
	execAnswerTestSQLT(t, "DELETE FROM guild_members WHERE guild_id = $1 AND commander_id = $2", int64(guildID), int64(formerCommanderID))

	client.Buffer.Reset()
	unmarkData, _ := proto.Marshal(&protobuf.CS_61033{RecommendUid: proto.Uint32(formerCommanderID), RecommendShipid: proto.Uint32(shipID), Cmd: proto.Uint32(1)})
	if _, _, err := MarkAssaultShipRecommendCommandResponse(&unmarkData, client); err != nil {
		t.Fatalf("61033 unmark failed: %v", err)
	}
	var unmarkResp protobuf.SC_61034
	decodeResponse(t, client, &unmarkResp)
	if unmarkResp.GetResult() != 0 {
		t.Fatalf("expected unmark success, got %d", unmarkResp.GetResult())
	}

	var remaining int64
	if err := db.DefaultStore.Pool.QueryRow(t.Context(), `
SELECT COUNT(*)
FROM guild_assault_recommendations
WHERE guild_id = $1
  AND commander_id = $2
  AND ship_id = $3
`, int64(guildID), int64(formerCommanderID), int64(shipID)).Scan(&remaining); err != nil {
		t.Fatalf("count recommendations failed: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("expected recommendation removed, found %d", remaining)
	}
}
