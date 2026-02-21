package orm

import (
	"context"
	"strings"
	"time"

	"github.com/ggmolly/belfast/internal/db"
)

const guildSearchLimit = 20

type GuildJoinRequest struct {
	GuildID     uint32
	Applicant   CommanderSocialProfile
	Content     string
	RequestedAt time.Time
}

type GuildDirectoryEntry struct {
	Guild    Guild
	Leader   CommanderSocialProfile
	TechSeat uint32
}

func UpsertGuildJoinRequest(guildID uint32, applicantCommanderID uint32, content string, requestedAt time.Time) error {
	ctx := context.Background()
	_, err := db.DefaultStore.Pool.Exec(ctx, `
INSERT INTO guild_join_requests (guild_id, applicant_commander_id, content, requested_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (guild_id, applicant_commander_id)
DO UPDATE SET
	content = EXCLUDED.content,
	requested_at = EXCLUDED.requested_at
`, int64(guildID), int64(applicantCommanderID), content, requestedAt.UTC())
	return err
}

func DeleteGuildJoinRequest(guildID uint32, applicantCommanderID uint32) (bool, error) {
	ctx := context.Background()
	result, err := db.DefaultStore.Pool.Exec(ctx, `
DELETE FROM guild_join_requests
WHERE guild_id = $1
	AND applicant_commander_id = $2
`, int64(guildID), int64(applicantCommanderID))
	if err != nil {
		return false, err
	}
	return result.RowsAffected() > 0, nil
}

func HasGuildJoinRequest(guildID uint32, applicantCommanderID uint32) (bool, error) {
	ctx := context.Background()
	var exists bool
	err := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT EXISTS(
	SELECT 1
	FROM guild_join_requests
	WHERE guild_id = $1
		AND applicant_commander_id = $2
)
`, int64(guildID), int64(applicantCommanderID)).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func CountGuildJoinRequestsByApplicant(applicantCommanderID uint32) (uint32, error) {
	ctx := context.Background()
	var count uint32
	err := db.DefaultStore.Pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM guild_join_requests
WHERE applicant_commander_id = $1
`, int64(applicantCommanderID)).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func ListGuildJoinRequests(guildID uint32) ([]GuildJoinRequest, error) {
	ctx := context.Background()
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT
	gr.guild_id,
	gr.applicant_commander_id,
	gr.content,
	gr.requested_at,
	c.name,
	c.level,
	c.manifesto,
	EXTRACT(EPOCH FROM c.last_login)::bigint,
	c.display_icon_id,
	c.display_skin_id,
	c.selected_icon_frame_id,
	c.selected_chat_frame_id,
	c.display_icon_theme_id
FROM guild_join_requests gr
JOIN commanders c ON c.commander_id = gr.applicant_commander_id
WHERE gr.guild_id = $1
ORDER BY gr.requested_at ASC, gr.applicant_commander_id ASC
`, int64(guildID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	requests := make([]GuildJoinRequest, 0)
	for rows.Next() {
		var request GuildJoinRequest
		var lastLoginUnix int64
		if err := rows.Scan(
			&request.GuildID,
			&request.Applicant.CommanderID,
			&request.Content,
			&request.RequestedAt,
			&request.Applicant.Name,
			&request.Applicant.Level,
			&request.Applicant.Manifesto,
			&lastLoginUnix,
			&request.Applicant.DisplayIconID,
			&request.Applicant.DisplaySkinID,
			&request.Applicant.SelectedIconFrameID,
			&request.Applicant.SelectedChatFrameID,
			&request.Applicant.DisplayIconThemeID,
		); err != nil {
			return nil, err
		}
		if lastLoginUnix > 0 {
			request.Applicant.LastLoginUnix = uint32(lastLoginUnix)
		}
		requests = append(requests, request)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return requests, nil
}

func SearchGuildDirectoryByID(guildID uint32) ([]GuildDirectoryEntry, error) {
	if guildID == 0 {
		return []GuildDirectoryEntry{}, nil
	}
	return queryGuildDirectory(`g.id = $1`, int64(guildID))
}

func SearchGuildDirectoryByName(keyword string) ([]GuildDirectoryEntry, error) {
	trimmed := strings.TrimSpace(keyword)
	if trimmed == "" {
		return []GuildDirectoryEntry{}, nil
	}
	return queryGuildDirectory(`LOWER(g.name) = LOWER($1)`, trimmed)
}

func queryGuildDirectory(whereClause string, args ...any) ([]GuildDirectoryEntry, error) {
	ctx := context.Background()
	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, int64(GuildDutyCommander), int64(guildSearchLimit))
	rows, err := db.DefaultStore.Pool.Query(ctx, `
SELECT
	g.id,
	g.policy,
	g.faction,
	g.name,
	g.level,
	g.announce,
	g.manifesto,
	g.exp,
	g.member_count,
	g.change_faction_cd,
	g.kick_leader_cd,
	g.capital,
	g.tech_id,
	COALESCE(c.commander_id, 0),
	COALESCE(c.name, ''),
	COALESCE(c.level, 0),
	COALESCE(c.manifesto, ''),
	COALESCE(EXTRACT(EPOCH FROM c.last_login)::bigint, 0),
	COALESCE(c.display_icon_id, 0),
	COALESCE(c.display_skin_id, 0),
	COALESCE(c.selected_icon_frame_id, 0),
	COALESCE(c.selected_chat_frame_id, 0),
	COALESCE(c.display_icon_theme_id, 0)
FROM guilds g
LEFT JOIN guild_members gm
	ON gm.guild_id = g.id
	AND gm.duty = $`+itoaArg(len(args)+1)+`
LEFT JOIN commanders c
	ON c.commander_id = gm.commander_id
	AND c.deleted_at IS NULL
WHERE g.deleted_at IS NULL
	AND `+whereClause+`
ORDER BY g.id ASC
LIMIT $`+itoaArg(len(args)+2), queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]GuildDirectoryEntry, 0)
	for rows.Next() {
		var entry GuildDirectoryEntry
		var lastLoginUnix int64
		if err := rows.Scan(
			&entry.Guild.ID,
			&entry.Guild.Policy,
			&entry.Guild.Faction,
			&entry.Guild.Name,
			&entry.Guild.Level,
			&entry.Guild.Announce,
			&entry.Guild.Manifesto,
			&entry.Guild.Exp,
			&entry.Guild.MemberCount,
			&entry.Guild.ChangeFactionCD,
			&entry.Guild.KickLeaderCD,
			&entry.Guild.Capital,
			&entry.Guild.TechID,
			&entry.Leader.CommanderID,
			&entry.Leader.Name,
			&entry.Leader.Level,
			&entry.Leader.Manifesto,
			&lastLoginUnix,
			&entry.Leader.DisplayIconID,
			&entry.Leader.DisplaySkinID,
			&entry.Leader.SelectedIconFrameID,
			&entry.Leader.SelectedChatFrameID,
			&entry.Leader.DisplayIconThemeID,
		); err != nil {
			return nil, err
		}
		if lastLoginUnix > 0 {
			entry.Leader.LastLoginUnix = uint32(lastLoginUnix)
		}
		entry.TechSeat = 0
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func itoaArg(n int) string {
	if n == 0 {
		return "0"
	}
	digits := [20]byte{}
	idx := len(digits)
	for n > 0 {
		idx--
		digits[idx] = byte('0' + (n % 10))
		n /= 10
	}
	return string(digits[idx:])
}
