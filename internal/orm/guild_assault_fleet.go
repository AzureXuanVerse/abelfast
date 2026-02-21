package orm

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

const GuildAssaultRecommendationLimit = uint32(9)

type GuildAssaultFleetSlot struct {
	GuildID     uint32
	CommanderID uint32
	Pos         uint32
	ShipID      uint32
	LastTime    uint32
}

type GuildAssaultRecommendation struct {
	GuildID     uint32
	CommanderID uint32
	ShipID      uint32
}

type GuildBossMissionFleet struct {
	GuildID     uint32
	OperationID uint32
	FleetID     uint32
	Ships       []GuildBossMissionShip
	Commanders  []GuildBossMissionCommander
}

type GuildBossMissionShip struct {
	UserID uint32 `json:"user_id"`
	ShipID uint32 `json:"ship_id"`
}

type GuildBossMissionCommander struct {
	Pos uint32 `json:"pos"`
	ID  uint32 `json:"id"`
}

func ListGuildAssaultFleetSlotsByCommander(guildID uint32, commanderID uint32) ([]GuildAssaultFleetSlot, error) {
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT guild_id, commander_id, pos, ship_id, last_time
FROM guild_assault_fleet_slots
WHERE guild_id = $1
  AND commander_id = $2
ORDER BY pos ASC
`, int64(guildID), int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]GuildAssaultFleetSlot, 0)
	for rows.Next() {
		var slot GuildAssaultFleetSlot
		if err := rows.Scan(&slot.GuildID, &slot.CommanderID, &slot.Pos, &slot.ShipID, &slot.LastTime); err != nil {
			return nil, err
		}
		result = append(result, slot)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func ListGuildAssaultFleetSlotsByGuild(guildID uint32) ([]GuildAssaultFleetSlot, error) {
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT guild_id, commander_id, pos, ship_id, last_time
FROM guild_assault_fleet_slots
WHERE guild_id = $1
ORDER BY commander_id ASC, pos ASC
`, int64(guildID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]GuildAssaultFleetSlot, 0)
	for rows.Next() {
		var slot GuildAssaultFleetSlot
		if err := rows.Scan(&slot.GuildID, &slot.CommanderID, &slot.Pos, &slot.ShipID, &slot.LastTime); err != nil {
			return nil, err
		}
		result = append(result, slot)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func UpsertGuildAssaultFleetSlots(guildID uint32, commanderID uint32, slots []GuildAssaultFleetSlot, now uint32) error {
	ctx := context.Background()
	return WithPGXTx(ctx, func(tx pgx.Tx) error {
		for _, slot := range slots {
			if _, err := tx.Exec(ctx, `
INSERT INTO guild_assault_fleet_slots (guild_id, commander_id, pos, ship_id, last_time, updated_at)
VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
ON CONFLICT (guild_id, commander_id, pos)
DO UPDATE SET ship_id = EXCLUDED.ship_id,
  last_time = EXCLUDED.last_time,
  updated_at = CURRENT_TIMESTAMP
`, int64(guildID), int64(commanderID), int64(slot.Pos), int64(slot.ShipID), int64(now)); err != nil {
				return err
			}
		}
		return nil
	})
}

func ListGuildAssaultRecommendations(guildID uint32) ([]GuildAssaultRecommendation, error) {
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT guild_id, commander_id, ship_id
FROM guild_assault_recommendations
WHERE guild_id = $1
ORDER BY commander_id ASC, ship_id ASC
`, int64(guildID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]GuildAssaultRecommendation, 0)
	for rows.Next() {
		var recommendation GuildAssaultRecommendation
		if err := rows.Scan(&recommendation.GuildID, &recommendation.CommanderID, &recommendation.ShipID); err != nil {
			return nil, err
		}
		result = append(result, recommendation)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func SetGuildAssaultRecommendation(guildID uint32, commanderID uint32, shipID uint32, recommended bool) error {
	ctx := context.Background()
	return WithPGXTx(ctx, func(tx pgx.Tx) error {
		if recommended {
			var lockedGuildID int64
			if err := tx.QueryRow(ctx, `
SELECT id
FROM guilds
WHERE id = $1
FOR UPDATE
`, int64(guildID)).Scan(&lockedGuildID); err != nil {
				return err
			}

			var exists bool
			if err := tx.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM guild_assault_recommendations
  WHERE guild_id = $1
    AND commander_id = $2
    AND ship_id = $3
)
`, int64(guildID), int64(commanderID), int64(shipID)).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				var count uint32
				if err := tx.QueryRow(ctx, `
SELECT COUNT(*)
FROM guild_assault_recommendations
WHERE guild_id = $1
`, int64(guildID)).Scan(&count); err != nil {
					return err
				}
				if count >= GuildAssaultRecommendationLimit {
					return ErrGuildPermission
				}
			}
			_, err := tx.Exec(ctx, `
INSERT INTO guild_assault_recommendations (guild_id, commander_id, ship_id)
VALUES ($1, $2, $3)
ON CONFLICT (guild_id, commander_id, ship_id) DO NOTHING
`, int64(guildID), int64(commanderID), int64(shipID))
			return err
		}

		_, err := tx.Exec(ctx, `
DELETE FROM guild_assault_recommendations
WHERE guild_id = $1
  AND commander_id = $2
  AND ship_id = $3
`, int64(guildID), int64(commanderID), int64(shipID))
		return err
	})
}

func UpsertGuildBossMissionFleets(guildID uint32, operationID uint32, fleets []GuildBossMissionFleet) error {
	ctx := context.Background()
	return WithPGXTx(ctx, func(tx pgx.Tx) error {
		for _, fleet := range fleets {
			ships, err := json.Marshal(fleet.Ships)
			if err != nil {
				return err
			}
			commanders, err := json.Marshal(fleet.Commanders)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
INSERT INTO guild_boss_mission_fleets (guild_id, operation_id, fleet_id, ships, commanders, updated_at)
VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
ON CONFLICT (guild_id, operation_id, fleet_id)
DO UPDATE SET ships = EXCLUDED.ships,
  commanders = EXCLUDED.commanders,
  updated_at = CURRENT_TIMESTAMP
`, int64(guildID), int64(operationID), int64(fleet.FleetID), ships, commanders); err != nil {
				return err
			}
		}
		return nil
	})
}

func ListGuildBossMissionFleets(guildID uint32, operationID uint32) ([]GuildBossMissionFleet, error) {
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT guild_id, operation_id, fleet_id, ships, commanders
FROM guild_boss_mission_fleets
WHERE guild_id = $1
  AND operation_id = $2
ORDER BY fleet_id ASC
`, int64(guildID), int64(operationID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]GuildBossMissionFleet, 0)
	for rows.Next() {
		var fleet GuildBossMissionFleet
		var shipsRaw []byte
		var commandersRaw []byte
		if err := rows.Scan(&fleet.GuildID, &fleet.OperationID, &fleet.FleetID, &shipsRaw, &commandersRaw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(shipsRaw, &fleet.Ships); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(commandersRaw, &fleet.Commanders); err != nil {
			return nil, err
		}
		result = append(result, fleet)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].FleetID < result[j].FleetID
	})
	return result, nil
}

func HasGuildBossMissionFleet(guildID uint32, operationID uint32) (bool, error) {
	ctx := context.Background()
	var exists bool
	err := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM guild_boss_mission_fleets
  WHERE guild_id = $1
    AND operation_id = $2
)
`, int64(guildID), int64(operationID)).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}
