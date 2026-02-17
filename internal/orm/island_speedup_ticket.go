package orm

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandSpeedupTicket struct {
	CommanderID uint32 `gorm:"primaryKey;column:commander_id"`
	SpeedID     uint32 `gorm:"primaryKey;column:speed_id"`
	EndTime     uint32 `gorm:"primaryKey;column:end_time"`
	Count       uint32 `gorm:"column:count"`
}

func (IslandSpeedupTicket) TableName() string {
	return "island_speedup_tickets"
}

type IslandSpeedupTicketKey struct {
	SpeedID uint32
	EndTime uint32
}

type IslandSpeedupTicketConsume struct {
	SpeedID uint32
	EndTime uint32
	Count   uint32
}

func UpsertIslandSpeedupTicket(commanderID uint32, speedID uint32, endTime uint32, count uint32) error {
	_, err := db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO island_speedup_tickets (commander_id, speed_id, end_time, count)
VALUES ($1, $2, $3, $4)
ON CONFLICT (commander_id, speed_id, end_time)
DO UPDATE SET count = EXCLUDED.count
`, int64(commanderID), int64(speedID), int64(endTime), int64(count))
	return err
}

func ListIslandSpeedupTickets(commanderID uint32) ([]IslandSpeedupTicket, error) {
	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT commander_id, speed_id, end_time, count
FROM island_speedup_tickets
WHERE commander_id = $1
ORDER BY speed_id, end_time
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tickets := make([]IslandSpeedupTicket, 0)
	for rows.Next() {
		var commanderIDRaw int64
		var speedIDRaw int64
		var endTimeRaw int64
		var countRaw int64
		if err := rows.Scan(&commanderIDRaw, &speedIDRaw, &endTimeRaw, &countRaw); err != nil {
			return nil, err
		}
		tickets = append(tickets, IslandSpeedupTicket{
			CommanderID: uint32(commanderIDRaw),
			SpeedID:     uint32(speedIDRaw),
			EndTime:     uint32(endTimeRaw),
			Count:       uint32(countRaw),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tickets, nil
}

func DeleteIslandSpeedupTicketKeysTx(ctx context.Context, tx pgx.Tx, commanderID uint32, keys []IslandSpeedupTicketKey) error {
	for i := range keys {
		if _, err := tx.Exec(ctx, `
DELETE FROM island_speedup_tickets
WHERE commander_id = $1 AND speed_id = $2 AND end_time = $3
`, int64(commanderID), int64(keys[i].SpeedID), int64(keys[i].EndTime)); err != nil {
			return err
		}
	}
	return nil
}

func ConsumeIslandSpeedupTicketsTx(ctx context.Context, tx pgx.Tx, commanderID uint32, requests []IslandSpeedupTicketConsume) error {
	for i := range requests {
		result, err := tx.Exec(ctx, `
UPDATE island_speedup_tickets
SET count = count - $4
WHERE commander_id = $1 AND speed_id = $2 AND end_time = $3 AND count >= $4
`, int64(commanderID), int64(requests[i].SpeedID), int64(requests[i].EndTime), int64(requests[i].Count))
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return db.ErrNotFound
		}
		if _, err := tx.Exec(ctx, `
DELETE FROM island_speedup_tickets
WHERE commander_id = $1 AND speed_id = $2 AND end_time = $3 AND count = 0
`, int64(commanderID), int64(requests[i].SpeedID), int64(requests[i].EndTime)); err != nil {
			return err
		}
	}
	return nil
}
