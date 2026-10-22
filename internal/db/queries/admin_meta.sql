-- name: AdminMetaShipTemplates :many
-- the ship templates a commander owns, to tell which META ships they have
SELECT DISTINCT template_id FROM ships WHERE commander_id = ?;

-- name: AdminMetaClaimAward :exec
INSERT INTO meta_pt_awards (commander_id, group_id, target) VALUES (?, ?, ?)
ON CONFLICT (commander_id, group_id, target) DO NOTHING;

-- name: AdminMetaUnclaimAwards :exec
-- a group's rewards from a target on (MetaPTData reads the claimed steps as
-- a prefix of the targets)
DELETE FROM meta_pt_awards WHERE commander_id = ? AND group_id = ? AND target >= ?;
