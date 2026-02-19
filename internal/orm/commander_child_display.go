package orm

import (
	"context"

	"github.com/ggmolly/belfast/internal/db"
)

func UpdateCommanderChildDisplay(commanderID uint32, childDisplay uint32) error {
	_, err := db.DefaultStore.Pool.Exec(context.Background(), `
UPDATE commanders
SET child_display = $2
WHERE commander_id = $1
`, int64(commanderID), int64(childDisplay))
	return err
}
