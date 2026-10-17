package db

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// IDList is a list of ids stored in one column, as a JSON array.
type IDList []uint32

// Value implements driver.Valuer.
func (list IDList) Value() (driver.Value, error) {
	if list == nil {
		return "[]", nil
	}
	data, err := json.Marshal([]uint32(list))
	return string(data), err
}

// Scan implements sql.Scanner.
func (list *IDList) Scan(src any) error {
	var data []byte
	switch src := src.(type) {
	case string:
		data = []byte(src)
	case []byte:
		data = src
	default:
		return fmt.Errorf("IDList from %T", src)
	}
	*list = nil
	return json.Unmarshal(data, (*[]uint32)(list))
}
