package game

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

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

// readConfigFile reads a config table's file, an error when it is missing or
// has no rows.
func readConfigFile(name string) ([]byte, error) {
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
	return bytes.TrimSpace(raw), nil
}

// readConfigTable decodes every row of a config table into T, keyed by id.
// Rows that don't fit T are skipped; a missing or empty table is an error.
func readConfigTable[T any](name string) (map[uint32]T, error) {
	raw, err := readConfigFile(name)
	if err != nil {
		return nil, err
	}
	// the whole table at once; row by row only when a row doesn't fit T
	if raw[0] == '[' {
		var rows []T
		var ids []struct {
			ID uint32 `json:"id"`
		}
		if json.Unmarshal(raw, &rows) == nil && json.Unmarshal(raw, &ids) == nil {
			table := make(map[uint32]T, len(rows))
			for i, row := range rows {
				table[ids[i].ID] = row
			}
			return table, nil
		}
	} else {
		var table map[uint32]T
		if json.Unmarshal(raw, &table) == nil {
			return table, nil
		}
	}
	table, err := readConfigRows[T](raw)
	if err != nil {
		return nil, fmt.Errorf("config table %s: %w", name, err)
	}
	return table, nil
}

// readConfigRows decodes a table row by row, skipping the rows that don't
// fit T (and, in the object layout, the keys that aren't ids).
func readConfigRows[T any](raw []byte) (map[uint32]T, error) {
	table := map[uint32]T{}
	add := func(id uint32, row json.RawMessage) {
		var value T
		if json.Unmarshal(row, &value) == nil {
			table[id] = value
		}
	}
	if raw[0] == '[' {
		var list []json.RawMessage
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, err
		}
		for _, row := range list {
			var key struct {
				ID uint32 `json:"id"`
			}
			if json.Unmarshal(row, &key) == nil {
				add(key.ID, row)
			}
		}
		return table, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	for key, row := range object {
		if id, err := strconv.ParseUint(key, 10, 32); err == nil {
			add(uint32(id), row)
		}
	}
	return table, nil
}

// gamesetConfig is a gameset row, keyed by the setting's name: key_value,
// or description for the settings that are lists.
type gamesetConfig struct {
	KeyValue    uint32          `json:"key_value"`
	Description json.RawMessage `json:"description"`
}

// readGameset reads the gameset settings.
func readGameset() (map[string]gamesetConfig, error) {
	raw, err := readConfigFile("gameset")
	if err != nil {
		return nil, err
	}
	var gameset map[string]gamesetConfig
	if err := json.Unmarshal(raw, &gameset); err != nil {
		return nil, fmt.Errorf("config table gameset: %w", err)
	}
	return gameset, nil
}

// loadTable reads a config table into *table as is.
func loadTable[T any](name string, table *map[uint32]T) func() error {
	return func() (err error) {
		*table, err = readConfigTable[T](name)
		return err
	}
}

// loadAll runs independent loads concurrently.
func loadAll(loads ...func() error) error {
	errs := make([]error, len(loads))
	var wg sync.WaitGroup
	for i, load := range loads {
		wg.Go(func() { errs[i] = load() })
	}
	wg.Wait()
	return errors.Join(errs...)
}

// loadGameData reads every config table the handlers use, once at startup:
// as is (loadTable) or into what the handlers index (ships, construction,
// the campaign, tasks...).
func loadGameData() error {
	return loadAll(
		loadShipData,
		loadChapterData,
		loadSignInData,
		loadTaskData,
		loadJuusData,
		loadBuildTimes,
		loadTable("constellation_challenge_month", &limitChallengeMonths),
		loadArenaShop,
		loadCommissionTeams,
		loadTable("honormedal_goods_list", &medalGoods),
		loadEducateData,
	)
}
