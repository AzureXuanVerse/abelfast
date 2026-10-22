-- name: AdminClearMetaRepairs :exec
DELETE FROM ship_meta_repairs WHERE ship_id = ?;
