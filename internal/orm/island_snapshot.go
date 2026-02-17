package orm

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandSnapshot struct {
	CommanderID    uint32
	Name           string
	Level          uint32
	Exp            uint32
	StorageLevel   uint32
	Prosperity     uint32
	AgoraLevel     uint32
	MapID          uint32
	PositionX      float32
	PositionY      float32
	PositionZ      float32
	RotationX      float32
	RotationY      float32
	RotationZ      float32
	OpenFlag       uint32
	InviteCode     string
	DailyTimestamp uint32
	FollowShips    []uint32
}

func (IslandSnapshot) TableName() string {
	return "island_snapshots"
}

func GetIslandSnapshot(commanderID uint32) (*IslandSnapshot, error) {
	var (
		commanderIDRaw    int64
		levelRaw          int64
		expRaw            int64
		storageLevelRaw   int64
		prosperityRaw     int64
		agoraLevelRaw     int64
		mapIDRaw          int64
		openFlagRaw       int64
		dailyTimestampRaw int64
		followShipsJSON   []byte
		snapshot          IslandSnapshot
	)
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT commander_id, name, level, exp, storage_level, prosperity, agora_level,
       map_id, position_x, position_y, position_z, rotation_x, rotation_y, rotation_z,
       open_flag, invite_code, daily_timestamp, follow_ships
FROM island_snapshots
WHERE commander_id = $1
`, int64(commanderID)).Scan(
		&commanderIDRaw,
		&snapshot.Name,
		&levelRaw,
		&expRaw,
		&storageLevelRaw,
		&prosperityRaw,
		&agoraLevelRaw,
		&mapIDRaw,
		&snapshot.PositionX,
		&snapshot.PositionY,
		&snapshot.PositionZ,
		&snapshot.RotationX,
		&snapshot.RotationY,
		&snapshot.RotationZ,
		&openFlagRaw,
		&snapshot.InviteCode,
		&dailyTimestampRaw,
		&followShipsJSON,
	)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(followShipsJSON, &snapshot.FollowShips); err != nil {
		return nil, err
	}
	if snapshot.FollowShips == nil {
		snapshot.FollowShips = []uint32{}
	}
	snapshot.CommanderID = uint32(commanderIDRaw)
	snapshot.Level = uint32(levelRaw)
	snapshot.Exp = uint32(expRaw)
	snapshot.StorageLevel = uint32(storageLevelRaw)
	snapshot.Prosperity = uint32(prosperityRaw)
	snapshot.AgoraLevel = uint32(agoraLevelRaw)
	snapshot.MapID = uint32(mapIDRaw)
	snapshot.OpenFlag = uint32(openFlagRaw)
	snapshot.DailyTimestamp = uint32(dailyTimestampRaw)
	return &snapshot, nil
}

func UpsertIslandSnapshot(snapshot *IslandSnapshot) error {
	followShips := snapshot.FollowShips
	if followShips == nil {
		followShips = []uint32{}
	}
	followShipsJSON, err := json.Marshal(followShips)
	if err != nil {
		return err
	}
	_, err = db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO island_snapshots (
	commander_id, name, level, exp, storage_level, prosperity, agora_level,
	map_id, position_x, position_y, position_z, rotation_x, rotation_y, rotation_z,
	open_flag, invite_code, daily_timestamp, follow_ships
)
VALUES (
	$1, $2, $3, $4, $5, $6, $7,
	$8, $9, $10, $11, $12, $13, $14,
	$15, $16, $17, $18
)
ON CONFLICT (commander_id)
DO UPDATE SET
	name = EXCLUDED.name,
	level = EXCLUDED.level,
	exp = EXCLUDED.exp,
	storage_level = EXCLUDED.storage_level,
	prosperity = EXCLUDED.prosperity,
	agora_level = EXCLUDED.agora_level,
	map_id = EXCLUDED.map_id,
	position_x = EXCLUDED.position_x,
	position_y = EXCLUDED.position_y,
	position_z = EXCLUDED.position_z,
	rotation_x = EXCLUDED.rotation_x,
	rotation_y = EXCLUDED.rotation_y,
	rotation_z = EXCLUDED.rotation_z,
	open_flag = EXCLUDED.open_flag,
	invite_code = EXCLUDED.invite_code,
	daily_timestamp = EXCLUDED.daily_timestamp,
	follow_ships = EXCLUDED.follow_ships,
	updated_at = CURRENT_TIMESTAMP
`,
		int64(snapshot.CommanderID),
		snapshot.Name,
		int64(snapshot.Level),
		int64(snapshot.Exp),
		int64(snapshot.StorageLevel),
		int64(snapshot.Prosperity),
		int64(snapshot.AgoraLevel),
		int64(snapshot.MapID),
		snapshot.PositionX,
		snapshot.PositionY,
		snapshot.PositionZ,
		snapshot.RotationX,
		snapshot.RotationY,
		snapshot.RotationZ,
		int64(snapshot.OpenFlag),
		snapshot.InviteCode,
		int64(snapshot.DailyTimestamp),
		followShipsJSON,
	)
	return err
}

func GetIslandSnapshotForUpdateTx(ctx context.Context, tx pgx.Tx, commanderID uint32) (*IslandSnapshot, error) {
	var (
		commanderIDRaw    int64
		levelRaw          int64
		expRaw            int64
		storageLevelRaw   int64
		prosperityRaw     int64
		agoraLevelRaw     int64
		mapIDRaw          int64
		openFlagRaw       int64
		dailyTimestampRaw int64
		followShipsJSON   []byte
		snapshot          IslandSnapshot
	)
	err := tx.QueryRow(ctx, `
SELECT commander_id, name, level, exp, storage_level, prosperity, agora_level,
       map_id, position_x, position_y, position_z, rotation_x, rotation_y, rotation_z,
       open_flag, invite_code, daily_timestamp, follow_ships
FROM island_snapshots
WHERE commander_id = $1
FOR UPDATE
`, int64(commanderID)).Scan(
		&commanderIDRaw,
		&snapshot.Name,
		&levelRaw,
		&expRaw,
		&storageLevelRaw,
		&prosperityRaw,
		&agoraLevelRaw,
		&mapIDRaw,
		&snapshot.PositionX,
		&snapshot.PositionY,
		&snapshot.PositionZ,
		&snapshot.RotationX,
		&snapshot.RotationY,
		&snapshot.RotationZ,
		&openFlagRaw,
		&snapshot.InviteCode,
		&dailyTimestampRaw,
		&followShipsJSON,
	)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(followShipsJSON, &snapshot.FollowShips); err != nil {
		return nil, err
	}
	if snapshot.FollowShips == nil {
		snapshot.FollowShips = []uint32{}
	}
	snapshot.CommanderID = uint32(commanderIDRaw)
	snapshot.Level = uint32(levelRaw)
	snapshot.Exp = uint32(expRaw)
	snapshot.StorageLevel = uint32(storageLevelRaw)
	snapshot.Prosperity = uint32(prosperityRaw)
	snapshot.AgoraLevel = uint32(agoraLevelRaw)
	snapshot.MapID = uint32(mapIDRaw)
	snapshot.OpenFlag = uint32(openFlagRaw)
	snapshot.DailyTimestamp = uint32(dailyTimestampRaw)
	return &snapshot, nil
}

func UpsertIslandSnapshotTx(ctx context.Context, tx pgx.Tx, snapshot *IslandSnapshot) error {
	followShips := snapshot.FollowShips
	if followShips == nil {
		followShips = []uint32{}
	}
	followShipsJSON, err := json.Marshal(followShips)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO island_snapshots (
	commander_id, name, level, exp, storage_level, prosperity, agora_level,
	map_id, position_x, position_y, position_z, rotation_x, rotation_y, rotation_z,
	open_flag, invite_code, daily_timestamp, follow_ships
)
VALUES (
	$1, $2, $3, $4, $5, $6, $7,
	$8, $9, $10, $11, $12, $13, $14,
	$15, $16, $17, $18
)
ON CONFLICT (commander_id)
DO UPDATE SET
	name = EXCLUDED.name,
	level = EXCLUDED.level,
	exp = EXCLUDED.exp,
	storage_level = EXCLUDED.storage_level,
	prosperity = EXCLUDED.prosperity,
	agora_level = EXCLUDED.agora_level,
	map_id = EXCLUDED.map_id,
	position_x = EXCLUDED.position_x,
	position_y = EXCLUDED.position_y,
	position_z = EXCLUDED.position_z,
	rotation_x = EXCLUDED.rotation_x,
	rotation_y = EXCLUDED.rotation_y,
	rotation_z = EXCLUDED.rotation_z,
	open_flag = EXCLUDED.open_flag,
	invite_code = EXCLUDED.invite_code,
	daily_timestamp = EXCLUDED.daily_timestamp,
	follow_ships = EXCLUDED.follow_ships,
	updated_at = CURRENT_TIMESTAMP
`,
		int64(snapshot.CommanderID),
		snapshot.Name,
		int64(snapshot.Level),
		int64(snapshot.Exp),
		int64(snapshot.StorageLevel),
		int64(snapshot.Prosperity),
		int64(snapshot.AgoraLevel),
		int64(snapshot.MapID),
		snapshot.PositionX,
		snapshot.PositionY,
		snapshot.PositionZ,
		snapshot.RotationX,
		snapshot.RotationY,
		snapshot.RotationZ,
		int64(snapshot.OpenFlag),
		snapshot.InviteCode,
		int64(snapshot.DailyTimestamp),
		followShipsJSON,
	)
	return err
}
