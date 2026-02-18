package orm

import (
	"context"
	"encoding/json"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandCurDress struct {
	Type uint32 `json:"type"`
	ID   uint32 `json:"id"`
}

type IslandCapState struct {
	DressID uint32 `json:"dress_id"`
	CapID   uint32 `json:"cap_id"`
}

type IslandCommanderDressProfile struct {
	CommanderID uint32
	IslandID    uint32
	CurDress    []IslandCurDress
	CapList     []IslandCapState
}

func GetIslandCommanderDressProfile(commanderID uint32) (*IslandCommanderDressProfile, error) {
	var (
		commanderIDRaw int64
		islandIDRaw    int64
		curDressJSON   []byte
		capJSON        []byte
		profile        IslandCommanderDressProfile
	)
	err := db.DefaultStore.Pool.QueryRow(context.Background(), `
SELECT commander_id, island_id, cur_dress, cap_list
FROM island_commander_dress_profiles
WHERE commander_id = $1
`, int64(commanderID)).Scan(&commanderIDRaw, &islandIDRaw, &curDressJSON, &capJSON)
	err = db.MapNotFound(err)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(curDressJSON, &profile.CurDress); err != nil {
		return nil, err
	}
	if profile.CurDress == nil {
		profile.CurDress = []IslandCurDress{}
	}
	if err := json.Unmarshal(capJSON, &profile.CapList); err != nil {
		return nil, err
	}
	if profile.CapList == nil {
		profile.CapList = []IslandCapState{}
	}
	profile.CommanderID = uint32(commanderIDRaw)
	profile.IslandID = uint32(islandIDRaw)
	return &profile, nil
}

func UpsertIslandCommanderDressProfile(profile *IslandCommanderDressProfile) error {
	curDress := profile.CurDress
	if curDress == nil {
		curDress = []IslandCurDress{}
	}
	capList := profile.CapList
	if capList == nil {
		capList = []IslandCapState{}
	}
	curDressJSON, err := json.Marshal(curDress)
	if err != nil {
		return err
	}
	capJSON, err := json.Marshal(capList)
	if err != nil {
		return err
	}
	_, err = db.DefaultStore.Pool.Exec(context.Background(), `
INSERT INTO island_commander_dress_profiles (commander_id, island_id, cur_dress, cap_list)
VALUES ($1, $2, $3, $4)
ON CONFLICT (commander_id)
DO UPDATE SET
	island_id = EXCLUDED.island_id,
	cur_dress = EXCLUDED.cur_dress,
	cap_list = EXCLUDED.cap_list,
	updated_at = CURRENT_TIMESTAMP
`, int64(profile.CommanderID), int64(profile.IslandID), curDressJSON, capJSON)
	return err
}
