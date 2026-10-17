package db

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// Column types sqlc maps lists to (sqlc.yaml overrides), stored as JSON.

// IDList is a list of ids.
type IDList []uint32

// Cells are chapter cells, {row, column}.
type Cells [][2]uint32

// Drops are rewards, {DROP_TYPE_*, id, number}.
type Drops [][3]uint32

func (list IDList) Value() (driver.Value, error) { return jsonValue([]uint32(list)) }
func (list *IDList) Scan(src any) error          { return jsonScan(src, (*[]uint32)(list)) }
func (cells Cells) Value() (driver.Value, error) { return jsonValue([][2]uint32(cells)) }
func (cells *Cells) Scan(src any) error          { return jsonScan(src, (*[][2]uint32)(cells)) }
func (drops Drops) Value() (driver.Value, error) { return jsonValue([][3]uint32(drops)) }
func (drops *Drops) Scan(src any) error          { return jsonScan(src, (*[][3]uint32)(drops)) }

func jsonValue[T any](list []T) (driver.Value, error) {
	if list == nil {
		return "[]", nil
	}
	data, err := json.Marshal(list)
	return string(data), err
}

// jsonScan decodes a JSON array column into *list; an empty column is an
// empty list.
func jsonScan[T any](src any, list *[]T) error {
	var data []byte
	switch src := src.(type) {
	case string:
		data = []byte(src)
	case []byte:
		data = src
	default:
		return fmt.Errorf("list column from %T", src)
	}
	*list = nil
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, list)
}
