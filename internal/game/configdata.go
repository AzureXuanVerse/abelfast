package game

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ggmolly/belfast/internal/region"
)

// Game data comes from the client's config tables at runtime, never from the
// repository: <data dir>/<region>/{sharecfgdata,ShareCfg}/<table>.json, as
// written by belfast-data's bump.sh (a list of rows with their id) or in the
// AzurLaneData layout (an object keyed by id, what the al-data skill's
// table.sh writes under .cache/al-data/<commit>). The data dir is ./data
// unless $BELFAST_DATA says otherwise. Content the official server sends that
// no table has lives next to the tables, in <data dir>/<region>/server.
func configDataDir() string {
	if dir := os.Getenv("BELFAST_DATA"); dir != "" {
		return dir
	}
	return "data"
}

// regionDataDir is the current region's part of the data dir.
func regionDataDir() string {
	return filepath.Join(configDataDir(), region.Current())
}

// readConfigTable decodes every row of a config table into T, keyed by id.
// Rows that don't fit T are skipped; a missing or empty table is an error.
func readConfigTable[T any](name string) (map[uint32]T, error) {
	var raw []byte
	var err error
	for _, sub := range []string{"sharecfgdata", "ShareCfg"} {
		raw, err = os.ReadFile(filepath.Join(regionDataDir(), sub, name+".json"))
		if err == nil && len(bytes.TrimSpace(raw)) > 2 {
			break
		}
	}
	if err == nil && len(bytes.TrimSpace(raw)) <= 2 {
		err = errors.New("no rows")
	}
	if err != nil {
		return nil, fmt.Errorf("config table %s in %s: %w", name, regionDataDir(), err)
	}
	rows := map[uint32]json.RawMessage{}
	switch bytes.TrimSpace(raw)[0] {
	case '[':
		var list []json.RawMessage
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, fmt.Errorf("config table %s: %w", name, err)
		}
		for _, row := range list {
			var key struct {
				ID uint32 `json:"id"`
			}
			if json.Unmarshal(row, &key) == nil {
				rows[key.ID] = row
			}
		}
	default:
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil {
			return nil, fmt.Errorf("config table %s: %w", name, err)
		}
		for key, row := range object {
			if id, err := strconv.ParseUint(key, 10, 32); err == nil {
				rows[uint32(id)] = row
			}
		}
	}
	table := make(map[uint32]T, len(rows))
	for id, row := range rows {
		var value T
		if json.Unmarshal(row, &value) == nil {
			table[id] = value
		}
	}
	return table, nil
}

// loadTable reads a config table into *table as is.
func loadTable[T any](name string, table *map[uint32]T) func() error {
	return func() (err error) {
		*table, err = readConfigTable[T](name)
		return err
	}
}

// loadGameData reads every config table the handlers use, once at startup:
// as is (loadTable) or into what the handlers index (ships, construction,
// the campaign, tasks...).
func loadGameData() error {
	for _, load := range []func() error{
		loadShipData,
		loadChapterData,
		loadSignInData,
		loadTaskData,
		loadJuusData,
		loadBuildTimes,
		loadTable("constellation_challenge_month", &limitChallengeMonths),
		loadArenaShop,
	} {
		if err := load(); err != nil {
			return err
		}
	}
	return nil
}
