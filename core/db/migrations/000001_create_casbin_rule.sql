-- +goose Up

CREATE TABLE casbin_rule (
    id    bigserial PRIMARY KEY,
    ptype text      NOT NULL,
    v0    text,
    v1    text,
    v2    text,
    v3    text,
    v4    text,
    v5    text
);

-- A rule is its ptype and its values, with trailing columns NULL when the
-- rule is shorter than six. Postgres treats NULLs as distinct in a UNIQUE
-- constraint, so the trailing NULLs are folded to '' and a unique index
-- keeps the same rule from being stored twice.
CREATE UNIQUE INDEX casbin_rule_unique_idx ON casbin_rule (
    ptype,
    coalesce(v0, ''),
    coalesce(v1, ''),
    coalesce(v2, ''),
    coalesce(v3, ''),
    coalesce(v4, ''),
    coalesce(v5, '')
);

-- +goose Down

DROP TABLE casbin_rule;
