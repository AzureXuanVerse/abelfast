// Package gamedata reads the client's config tables, at runtime and never from
// the repository: <data dir>/<region>/{sharecfgdata,ShareCfg}/<table>.json, as
// written by belfast-data's bump.sh (a list of rows with their id) or in the
// AzurLaneData layout (an object keyed by id, what the al-data skill's
// table.sh writes under .cache/al-data/<commit>). The data dir is ./data
// unless $BELFAST_DATA says otherwise. Content the official server sends that
// no table has lives next to the tables, in <data dir>/<region>/server.
package gamedata

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/ggmolly/belfast/internal/region"
)

// Dir is the data dir.
func Dir() string {
	if dir := os.Getenv("BELFAST_DATA"); dir != "" {
		return dir
	}
	return "data"
}

// RegionDir is the current region's part of the data dir.
func RegionDir() string {
	return filepath.Join(Dir(), region.Current())
}

// Missing is the error for a table lacking a row or value the server needs.
func Missing(table, format string, args ...any) error {
	return fmt.Errorf("config table %s in %s: no %s", table, RegionDir(), fmt.Sprintf(format, args...))
}

// ServerFile reads server content (<region>/server/<name>), nil when the data
// dir has none.
func ServerFile(name string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(RegionDir(), "server", name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

// file reads a table's file, an error when it is missing or has no rows.
func file(name string) ([]byte, error) {
	var raw []byte
	var err error
	for _, sub := range []string{"sharecfgdata", "ShareCfg"} {
		raw, err = os.ReadFile(filepath.Join(RegionDir(), sub, name+".json"))
		if err == nil && len(bytes.TrimSpace(raw)) > 2 {
			break
		}
	}
	if err == nil && len(bytes.TrimSpace(raw)) <= 2 {
		err = errors.New("no rows")
	}
	if err != nil {
		return nil, fmt.Errorf("config table %s in %s: %w", name, RegionDir(), err)
	}
	return bytes.TrimSpace(raw), nil
}

// Table decodes every row of a table into T, keyed by id. Rows that don't fit
// T are skipped; a missing or empty table is an error.
func Table[T any](name string) (map[uint32]T, error) {
	raw, err := file(name)
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
	table, err := rows[T](raw)
	if err != nil {
		return nil, fmt.Errorf("config table %s: %w", name, err)
	}
	return table, nil
}

// rows decodes a table row by row, skipping the rows that don't fit T (and,
// in the object layout, the keys that aren't ids).
func rows[T any](raw []byte) (map[uint32]T, error) {
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

// Gameset reads the gameset settings' key_value, by setting name.
func Gameset() (map[string]uint32, error) {
	raw, err := file("gameset")
	if err != nil {
		return nil, err
	}
	var rows map[string]struct {
		KeyValue uint32 `json:"key_value"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("config table gameset: %w", err)
	}
	gameset := make(map[string]uint32, len(rows))
	for key, row := range rows {
		gameset[key] = row.KeyValue
	}
	return gameset, nil
}

// Load reads a table into *table as is.
func Load[T any](name string, table *map[uint32]T) func() error {
	return func() (err error) {
		*table, err = Table[T](name)
		return err
	}
}

// LoadAll runs independent loads concurrently.
func LoadAll(loads ...func() error) error {
	errs := make([]error, len(loads))
	var wg sync.WaitGroup
	for i, load := range loads {
		wg.Go(func() { errs[i] = load() })
	}
	wg.Wait()
	return errors.Join(errs...)
}
