package orm

import (
	"context"

	"github.com/ggmolly/belfast/internal/db"
)

type CommanderSocialProfile struct {
	CommanderID         uint32
	Name                string
	Level               uint32
	Manifesto           string
	LastLoginUnix       uint32
	DisplayIconID       uint32
	DisplaySkinID       uint32
	SelectedIconFrameID uint32
	SelectedChatFrameID uint32
	DisplayIconThemeID  uint32
	ShipCount           uint32
	CollectionCount     uint32
	CollectAttackCount  uint32
	PvpAttackCount      uint32
	PvpWinCount         uint32
}

func GetCommanderSocialProfileByID(commanderID uint32) (*CommanderSocialProfile, error) {
	profiles, err := getCommanderSocialProfilesByQuery(`
SELECT
	c.commander_id,
	c.name,
	c.level,
	c.manifesto,
	EXTRACT(EPOCH FROM c.last_login)::bigint,
	c.display_icon_id,
	c.display_skin_id,
	c.selected_icon_frame_id,
	c.selected_chat_frame_id,
	c.display_icon_theme_id,
	COALESCE((SELECT COUNT(*) FROM owned_ships os WHERE os.owner_id = c.commander_id AND os.deleted_at IS NULL), 0),
	COALESCE((SELECT COUNT(*) FROM owned_skins sk WHERE sk.commander_id = c.commander_id), 0),
	c.collect_attack_count,
	0,
	0
FROM commanders c
WHERE c.commander_id = $1
  AND c.deleted_at IS NULL
`, int64(commanderID))
	if err != nil {
		return nil, err
	}
	if len(profiles) == 0 {
		return nil, db.ErrNotFound
	}
	return &profiles[0], nil
}

func GetCommanderSocialProfileByName(name string) (*CommanderSocialProfile, error) {
	profiles, err := getCommanderSocialProfilesByQuery(`
SELECT
	c.commander_id,
	c.name,
	c.level,
	c.manifesto,
	EXTRACT(EPOCH FROM c.last_login)::bigint,
	c.display_icon_id,
	c.display_skin_id,
	c.selected_icon_frame_id,
	c.selected_chat_frame_id,
	c.display_icon_theme_id,
	COALESCE((SELECT COUNT(*) FROM owned_ships os WHERE os.owner_id = c.commander_id AND os.deleted_at IS NULL), 0),
	COALESCE((SELECT COUNT(*) FROM owned_skins sk WHERE sk.commander_id = c.commander_id), 0),
	c.collect_attack_count,
	0,
	0
FROM commanders c
WHERE c.name = $1
  AND c.deleted_at IS NULL
LIMIT 1
`, name)
	if err != nil {
		return nil, err
	}
	if len(profiles) == 0 {
		return nil, db.ErrNotFound
	}
	return &profiles[0], nil
}

func GetCommanderSocialProfilesByIDs(commanderIDs []uint32) (map[uint32]CommanderSocialProfile, error) {
	result := make(map[uint32]CommanderSocialProfile)
	if len(commanderIDs) == 0 {
		return result, nil
	}

	profiles, err := getCommanderSocialProfilesByQuery(`
SELECT
	c.commander_id,
	c.name,
	c.level,
	c.manifesto,
	EXTRACT(EPOCH FROM c.last_login)::bigint,
	c.display_icon_id,
	c.display_skin_id,
	c.selected_icon_frame_id,
	c.selected_chat_frame_id,
	c.display_icon_theme_id,
	COALESCE((SELECT COUNT(*) FROM owned_ships os WHERE os.owner_id = c.commander_id AND os.deleted_at IS NULL), 0),
	COALESCE((SELECT COUNT(*) FROM owned_skins sk WHERE sk.commander_id = c.commander_id), 0),
	c.collect_attack_count,
	0,
	0
FROM commanders c
WHERE c.commander_id = ANY($1)
  AND c.deleted_at IS NULL
`, commanderIDs)
	if err != nil {
		return nil, err
	}
	for _, profile := range profiles {
		result[profile.CommanderID] = profile
	}
	return result, nil
}

func ListCommanderSocialProfilesForRecommendations(excludeCommanderID uint32, limit int) ([]CommanderSocialProfile, error) {
	if limit <= 0 {
		limit = 20
	}
	return getCommanderSocialProfilesByQuery(`
SELECT
	c.commander_id,
	c.name,
	c.level,
	c.manifesto,
	EXTRACT(EPOCH FROM c.last_login)::bigint,
	c.display_icon_id,
	c.display_skin_id,
	c.selected_icon_frame_id,
	c.selected_chat_frame_id,
	c.display_icon_theme_id,
	COALESCE((SELECT COUNT(*) FROM owned_ships os WHERE os.owner_id = c.commander_id AND os.deleted_at IS NULL), 0),
	COALESCE((SELECT COUNT(*) FROM owned_skins sk WHERE sk.commander_id = c.commander_id), 0),
	c.collect_attack_count,
	0,
	0
FROM commanders c
LEFT JOIN commander_friend_relations cfr
  ON cfr.commander_id = $1
 AND cfr.friend_commander_id = c.commander_id
WHERE c.commander_id <> $1
  AND c.deleted_at IS NULL
  AND cfr.friend_commander_id IS NULL
ORDER BY c.last_login DESC
LIMIT $2
`, int64(excludeCommanderID), limit)
}

func getCommanderSocialProfilesByQuery(query string, args ...any) ([]CommanderSocialProfile, error) {
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	profiles := make([]CommanderSocialProfile, 0)
	for rows.Next() {
		var profile CommanderSocialProfile
		var lastLoginUnix int64
		if err := rows.Scan(
			&profile.CommanderID,
			&profile.Name,
			&profile.Level,
			&profile.Manifesto,
			&lastLoginUnix,
			&profile.DisplayIconID,
			&profile.DisplaySkinID,
			&profile.SelectedIconFrameID,
			&profile.SelectedChatFrameID,
			&profile.DisplayIconThemeID,
			&profile.ShipCount,
			&profile.CollectionCount,
			&profile.CollectAttackCount,
			&profile.PvpAttackCount,
			&profile.PvpWinCount,
		); err != nil {
			return nil, err
		}
		if lastLoginUnix > 0 {
			profile.LastLoginUnix = uint32(lastLoginUnix)
		}
		profiles = append(profiles, profile)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return profiles, nil
}
