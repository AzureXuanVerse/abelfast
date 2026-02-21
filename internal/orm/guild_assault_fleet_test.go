package orm

import (
	"context"
	"testing"

	"github.com/ggmolly/belfast/internal/db"
)

func seedGuildAssaultORMFixtures(t *testing.T, guildID uint32, commanderID uint32) {
	t.Helper()
	initCommanderItemTestDB(t)
	clearTable(t, &GuildAssaultRecommendation{})
	clearTable(t, &GuildAssaultFleetSlot{})
	clearTable(t, &Commander{})
	clearTable(t, &GuildMember{})
	clearTable(t, &Guild{})

	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `INSERT INTO commanders (commander_id, account_id, name) VALUES ($1, $2, $3)`, int64(commanderID), int64(commanderID), "Guild Assault ORM"); err != nil {
		t.Fatalf("seed commander: %v", err)
	}
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `INSERT INTO guilds (id, policy, faction, name, level, announce, manifesto, exp, member_count, change_faction_cd, kick_leader_cd, capital, tech_id) VALUES ($1, 1, 1, 'ORM Guild', 1, '', '', 0, 1, 0, 0, 1000, 1)`, int64(guildID)); err != nil {
		t.Fatalf("seed guild: %v", err)
	}
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), `INSERT INTO guild_members (guild_id, commander_id, duty, liveness, pre_online_time, join_time) VALUES ($1, $2, 1, 0, 0, 0)`, int64(guildID), int64(commanderID)); err != nil {
		t.Fatalf("seed guild member: %v", err)
	}
}

func TestGuildAssaultFleetUpsertAndList(t *testing.T) {
	seedGuildAssaultORMFixtures(t, 9201, 3201)

	if err := UpsertGuildAssaultFleetSlots(9201, 3201, []GuildAssaultFleetSlot{{Pos: 2, ShipID: 2002}, {Pos: 1, ShipID: 2001}}, 100); err != nil {
		t.Fatalf("upsert slots: %v", err)
	}
	if err := UpsertGuildAssaultFleetSlots(9201, 3201, []GuildAssaultFleetSlot{{Pos: 1, ShipID: 2999}}, 200); err != nil {
		t.Fatalf("upsert slot update: %v", err)
	}

	slots, err := ListGuildAssaultFleetSlotsByCommander(9201, 3201)
	if err != nil {
		t.Fatalf("list slots: %v", err)
	}
	if len(slots) != 2 {
		t.Fatalf("expected 2 slots, got %d", len(slots))
	}
	if slots[0].Pos != 1 || slots[0].ShipID != 2999 || slots[0].LastTime != 200 {
		t.Fatalf("unexpected first slot: %+v", slots[0])
	}
}

func TestGuildAssaultRecommendationLimit(t *testing.T) {
	seedGuildAssaultORMFixtures(t, 9202, 3202)
	for i := uint32(0); i < GuildAssaultRecommendationLimit; i++ {
		if err := SetGuildAssaultRecommendation(9202, 3202, 4000+i, true); err != nil {
			t.Fatalf("seed recommendation %d: %v", i, err)
		}
	}
	if err := SetGuildAssaultRecommendation(9202, 3202, 9999, true); err != ErrGuildPermission {
		t.Fatalf("expected ErrGuildPermission at recommendation limit, got %v", err)
	}
}
