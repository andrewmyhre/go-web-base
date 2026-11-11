-- name: LoadCasbinRules :many
SELECT ptype, v0, v1, v2, v3, v4, v5
FROM casbin_rule
ORDER BY id;

-- name: LoadCasbinRulesByPtype :many
SELECT ptype, v0, v1, v2, v3, v4, v5
FROM casbin_rule
WHERE ptype = sqlc.arg(ptype)
ORDER BY id;

-- name: InsertCasbinRule :exec
INSERT INTO casbin_rule (ptype, v0, v1, v2, v3, v4, v5)
VALUES (
    sqlc.arg(ptype), sqlc.narg(v0), sqlc.narg(v1), sqlc.narg(v2),
    sqlc.narg(v3), sqlc.narg(v4), sqlc.narg(v5)
);

-- name: DeleteCasbinRule :exec
-- Exact match: rules are NULL-padded, and IS NOT DISTINCT FROM treats NULL =
-- NULL as a match, so a given rule removes only rows with identical values.
DELETE FROM casbin_rule
WHERE ptype = sqlc.arg(ptype)
  AND v0 IS NOT DISTINCT FROM sqlc.narg(v0)
  AND v1 IS NOT DISTINCT FROM sqlc.narg(v1)
  AND v2 IS NOT DISTINCT FROM sqlc.narg(v2)
  AND v3 IS NOT DISTINCT FROM sqlc.narg(v3)
  AND v4 IS NOT DISTINCT FROM sqlc.narg(v4)
  AND v5 IS NOT DISTINCT FROM sqlc.narg(v5);

-- name: DeleteCasbinRulesFiltered :exec
-- A filter value bound as NULL matches any stored value, including a NULL
-- one; the adapter binds NULL for every column outside the filter and for
-- every empty filter value, which Casbin treats as a wildcard. A non-NULL
-- filter value matches only that stored value, never a NULL column. The
-- ::text casts let Postgres type a parameter whose first use is `IS NULL`
-- (without them pgx fails with SQLSTATE 42P08).
DELETE FROM casbin_rule
WHERE ptype = sqlc.arg(ptype)
  AND (sqlc.narg(f0)::text IS NULL OR v0 = sqlc.narg(f0))
  AND (sqlc.narg(f1)::text IS NULL OR v1 = sqlc.narg(f1))
  AND (sqlc.narg(f2)::text IS NULL OR v2 = sqlc.narg(f2))
  AND (sqlc.narg(f3)::text IS NULL OR v3 = sqlc.narg(f3))
  AND (sqlc.narg(f4)::text IS NULL OR v4 = sqlc.narg(f4))
  AND (sqlc.narg(f5)::text IS NULL OR v5 = sqlc.narg(f5));

-- name: DeleteAllCasbinRules :exec
-- SavePolicy replaces the whole policy inside one transaction.
DELETE FROM casbin_rule;
