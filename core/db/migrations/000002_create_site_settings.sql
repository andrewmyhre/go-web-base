-- +goose Up

-- One row per site setting that has been changed from its default. A setting
-- with no row is at its default, so an existing app that upgrades core keeps
-- today's behaviour with no data to backfill. The value is JSON so a later
-- setting is not limited to text; core validates it against the setting's
-- definition before it is written.
CREATE TABLE site_setting (
    key        text        PRIMARY KEY,
    value      jsonb       NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by text        NOT NULL DEFAULT ''
);

-- An invite lets one email address make its first sign-in while the site is
-- invite-only. email is stored lower case. At most one invite per address is
-- pending (neither revoked nor consumed), so an admin cannot stack them; a
-- consumed or revoked invite stays as the record.
CREATE TABLE site_invite (
    id          bigserial   PRIMARY KEY,
    email       text        NOT NULL,
    created_by  text        NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now(),
    revoked_at  timestamptz,
    consumed_at timestamptz,
    consumed_by text,
    CONSTRAINT site_invite_email_lower CHECK (email = lower(email)),
    CONSTRAINT site_invite_one_outcome CHECK (revoked_at IS NULL OR consumed_at IS NULL)
);

CREATE UNIQUE INDEX site_invite_pending_email_idx ON site_invite (email)
    WHERE revoked_at IS NULL AND consumed_at IS NULL;

-- +goose Down

DROP TABLE site_invite;
DROP TABLE site_setting;
