-- +goose Up
-- Install-level request counters in fixed windows, shared by every replica.
-- rate_limit.gc deletes windows older than 2h.
CREATE TABLE rate_limit (
    key text NOT NULL,
    window_start timestamptz NOT NULL,
    count int NOT NULL,
    PRIMARY KEY (key, window_start)
);

-- mfa_at is when the web session last passed a second factor.
ALTER TABLE user_session ADD COLUMN mfa_at timestamptz;
ALTER TABLE org_setting ADD COLUMN require_mfa text NOT NULL DEFAULT 'off' CHECK (require_mfa IN ('off', 'admins', 'all'));

-- +goose Down
ALTER TABLE org_setting DROP COLUMN require_mfa;
ALTER TABLE user_session DROP COLUMN mfa_at;
DROP TABLE rate_limit;
