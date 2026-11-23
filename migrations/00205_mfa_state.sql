-- +goose Up
-- A WebAuthn ceremony is single use, bound to its user, kind and ref, and
-- lives 5 minutes.
CREATE TABLE webauthn_ceremony (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL,
    user_id uuid NOT NULL,
    kind text NOT NULL CHECK (kind IN ('register', 'login', 'stepup', 'challenge')),
    ref text NOT NULL DEFAULT '',
    session_data jsonb NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (org_id, user_id) REFERENCES membership(org_id, user_id) ON DELETE CASCADE
);
CREATE INDEX webauthn_ceremony_open_idx ON webauthn_ceremony (user_id, kind, ref) WHERE used_at IS NULL;

-- mfa_login is the token between the password and the second factor.
ALTER TABLE one_time_token DROP CONSTRAINT one_time_token_kind_check,
    ADD CONSTRAINT one_time_token_kind_check CHECK (kind IN ('invite', 'password_reset', 'account_deletion', 'mfa_login'));

-- The web session that opened a stream (set only from the caller's own session).
ALTER TABLE stream_ticket ADD COLUMN user_session_id uuid REFERENCES user_session ON DELETE CASCADE;

-- +goose Down
ALTER TABLE stream_ticket DROP COLUMN user_session_id;
DELETE FROM one_time_token WHERE kind = 'mfa_login';
ALTER TABLE one_time_token DROP CONSTRAINT one_time_token_kind_check,
    ADD CONSTRAINT one_time_token_kind_check CHECK (kind IN ('invite', 'password_reset', 'account_deletion'));
DROP TABLE webauthn_ceremony;
