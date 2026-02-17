package orm

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/ggmolly/belfast/internal/db"
)

type IslandAgoraTheme struct {
	CommanderID uint32 `json:"commander_id"`
	ThemeSlotID uint32 `json:"theme_slot_id"`
	Name        string `json:"name"`
	PlacedData  []byte `json:"placed_data"`
}

func UpsertIslandAgoraThemeTx(ctx context.Context, tx pgx.Tx, commanderID uint32, slotID uint32, name string, placedData []byte) error {
	_, err := tx.Exec(ctx, `
INSERT INTO island_agora_themes (
  commander_id,
  theme_slot_id,
  name,
  placed_data
) VALUES (
  $1, $2, $3, $4
)
ON CONFLICT (commander_id, theme_slot_id)
DO UPDATE SET
  name = EXCLUDED.name,
  placed_data = EXCLUDED.placed_data
`, int64(commanderID), int64(slotID), name, placedData)
	return err
}

func DeleteIslandAgoraThemeTx(ctx context.Context, tx pgx.Tx, commanderID uint32, slotID uint32) error {
	_, err := tx.Exec(ctx, `DELETE FROM island_agora_themes WHERE commander_id = $1 AND theme_slot_id = $2`, int64(commanderID), int64(slotID))
	return err
}

func ListIslandAgoraThemes(commanderID uint32) ([]IslandAgoraTheme, error) {
	if db.DefaultStore == nil {
		return nil, errors.New("db not initialized")
	}
	rows, err := db.DefaultStore.Pool.Query(context.Background(), `
SELECT commander_id, theme_slot_id, name, placed_data
FROM island_agora_themes
WHERE commander_id = $1
ORDER BY theme_slot_id ASC
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	themes := make([]IslandAgoraTheme, 0)
	for rows.Next() {
		var theme IslandAgoraTheme
		if err := rows.Scan(&theme.CommanderID, &theme.ThemeSlotID, &theme.Name, &theme.PlacedData); err != nil {
			return nil, err
		}
		themes = append(themes, theme)
	}

	return themes, rows.Err()
}
