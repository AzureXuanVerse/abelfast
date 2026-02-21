package orm

import (
	"context"
	"testing"

	"github.com/ggmolly/belfast/internal/db"
)

func TestGuildOperationBossStateRoundTrip(t *testing.T) {
	t.Setenv("MODE", "test")
	InitDatabase()

	const guildID uint32 = 9301
	const operationID uint32 = 7401
	execGuildBossStateSQL(t, "DELETE FROM guild_operation_boss_ranks")
	execGuildBossStateSQL(t, "DELETE FROM guild_operation_boss_states")
	execGuildBossStateSQL(t, "DELETE FROM guilds")
	execGuildBossStateSQL(t, "INSERT INTO guilds (id, policy, faction, name, level, announce, manifesto, exp, member_count, change_faction_cd, kick_leader_cd, capital, tech_id) VALUES ($1, 1, 1, 'GB', 1, '', '', 0, 1, 0, 0, 1000, 1)", int64(guildID))

	if err := UpsertGuildOperationBossState(GuildOperationBossState{
		GuildID:     guildID,
		OperationID: operationID,
		BossID:      101,
		Damage:      250,
		HP:          8000,
	}); err != nil {
		t.Fatalf("upsert boss state failed: %v", err)
	}

	state, err := GetGuildOperationBossState(guildID, operationID)
	if err != nil {
		t.Fatalf("get boss state failed: %v", err)
	}
	if state.BossID != 101 || state.Damage != 250 || state.HP != 8000 {
		t.Fatalf("unexpected boss state payload: %+v", state)
	}

	if err := ReplaceGuildOperationBossRanks(guildID, operationID, 101, []GuildOperationBossRank{{UserID: 5, Damage: 300}, {UserID: 4, Damage: 300}, {UserID: 6, Damage: 120}}); err != nil {
		t.Fatalf("replace boss ranks failed: %v", err)
	}

	ranks, err := ListGuildOperationBossRanks(guildID, operationID, 101)
	if err != nil {
		t.Fatalf("list boss ranks failed: %v", err)
	}
	if len(ranks) != 3 {
		t.Fatalf("expected 3 ranks, got %d", len(ranks))
	}
	if ranks[0].UserID != 4 || ranks[1].UserID != 5 || ranks[2].UserID != 6 {
		t.Fatalf("expected stable ordering by damage desc, user id asc")
	}
}

func execGuildBossStateSQL(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := db.DefaultStore.Pool.Exec(context.Background(), query, args...); err != nil {
		t.Fatalf("exec sql failed: %v", err)
	}
}
