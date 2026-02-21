package orm

import (
	"context"
	"sort"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type GuildOperationBossState struct {
	GuildID     uint32
	OperationID uint32
	BossID      uint32
	Damage      uint32
	HP          uint32
}

type GuildOperationBossRank struct {
	UserID uint32
	Damage uint32
}

func GetGuildOperationBossState(guildID uint32, operationID uint32) (*GuildOperationBossState, error) {
	ctx := context.Background()
	row := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT guild_id, operation_id, boss_id, damage, hp
FROM guild_operation_boss_states
WHERE guild_id = $1
  AND operation_id = $2
`, int64(guildID), int64(operationID))
	state := &GuildOperationBossState{}
	if err := row.Scan(&state.GuildID, &state.OperationID, &state.BossID, &state.Damage, &state.HP); err != nil {
		return nil, db.MapNotFound(err)
	}
	return state, nil
}

func UpsertGuildOperationBossState(state GuildOperationBossState) error {
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
INSERT INTO guild_operation_boss_states (guild_id, operation_id, boss_id, damage, hp, updated_at)
VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
ON CONFLICT (guild_id, operation_id)
DO UPDATE SET boss_id = EXCLUDED.boss_id,
	damage = EXCLUDED.damage,
	hp = EXCLUDED.hp,
	updated_at = CURRENT_TIMESTAMP
`, int64(state.GuildID), int64(state.OperationID), int64(state.BossID), int64(state.Damage), int64(state.HP))
	return err
}

func ListGuildOperationBossRanks(guildID uint32, operationID uint32, bossID uint32) ([]GuildOperationBossRank, error) {
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT user_id, damage
FROM guild_operation_boss_ranks
WHERE guild_id = $1
  AND operation_id = $2
  AND boss_id = $3
ORDER BY damage DESC, user_id ASC
`, int64(guildID), int64(operationID), int64(bossID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ranks := make([]GuildOperationBossRank, 0)
	for rows.Next() {
		var rank GuildOperationBossRank
		if err := rows.Scan(&rank.UserID, &rank.Damage); err != nil {
			return nil, err
		}
		ranks = append(ranks, rank)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ranks, nil
}

func ReplaceGuildOperationBossRanks(guildID uint32, operationID uint32, bossID uint32, ranks []GuildOperationBossRank) error {
	ctx := context.Background()
	return WithPGXTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
DELETE FROM guild_operation_boss_ranks
WHERE guild_id = $1
  AND operation_id = $2
  AND boss_id = $3
`, int64(guildID), int64(operationID), int64(bossID)); err != nil {
			return err
		}
		ordered := append([]GuildOperationBossRank(nil), ranks...)
		sort.Slice(ordered, func(i, j int) bool {
			if ordered[i].Damage == ordered[j].Damage {
				return ordered[i].UserID < ordered[j].UserID
			}
			return ordered[i].Damage > ordered[j].Damage
		})
		for _, rank := range ordered {
			if _, err := tx.Exec(ctx, `
INSERT INTO guild_operation_boss_ranks (guild_id, operation_id, boss_id, user_id, damage)
VALUES ($1, $2, $3, $4, $5)
`, int64(guildID), int64(operationID), int64(bossID), int64(rank.UserID), int64(rank.Damage)); err != nil {
				return err
			}
		}
		return nil
	})
}
