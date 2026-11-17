-- name: GetSiteSetting :one
SELECT key, value, updated_at, updated_by
FROM site_setting
WHERE key = sqlc.arg(key);

-- name: ListSiteSettings :many
SELECT key, value, updated_at, updated_by
FROM site_setting
ORDER BY key;

-- name: UpsertSiteSetting :exec
INSERT INTO site_setting (key, value, updated_at, updated_by)
VALUES (sqlc.arg(key), sqlc.arg(value), now(), sqlc.arg(updated_by))
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value, updated_at = now(), updated_by = EXCLUDED.updated_by;

-- name: CreateSiteInvite :one
INSERT INTO site_invite (email, created_by)
VALUES (sqlc.arg(email), sqlc.arg(created_by))
RETURNING id, email, created_by, created_at, revoked_at, consumed_at, consumed_by;

-- name: ListSiteInvites :many
SELECT id, email, created_by, created_at, revoked_at, consumed_at, consumed_by
FROM site_invite
ORDER BY id DESC;

-- name: RevokeSiteInvite :one
-- Only a pending invite can be revoked: a consumed one already did its job.
UPDATE site_invite
SET revoked_at = now()
WHERE id = sqlc.arg(id) AND revoked_at IS NULL AND consumed_at IS NULL
RETURNING id, email, created_by, created_at, revoked_at, consumed_at, consumed_by;

-- name: HasPendingSiteInvite :one
SELECT EXISTS (
    SELECT 1 FROM site_invite
    WHERE email = sqlc.arg(email) AND revoked_at IS NULL AND consumed_at IS NULL
);

-- name: ConsumeSiteInvite :execrows
-- The single UPDATE is what makes consumption happen once: a second caller
-- finds no pending row and changes nothing.
UPDATE site_invite
SET consumed_at = now(), consumed_by = sqlc.arg(consumed_by)
WHERE email = sqlc.arg(email) AND revoked_at IS NULL AND consumed_at IS NULL;
