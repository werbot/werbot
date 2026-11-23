-- +goose Up
-- Second factors belong to the user, not to an organization (like user_ssh_key).
CREATE TABLE webauthn_credential (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 64),
    credential_id bytea NOT NULL UNIQUE,
    public_key bytea NOT NULL,
    sign_count bigint NOT NULL DEFAULT 0,
    aaguid bytea NOT NULL,
    transports text[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz
);
CREATE INDEX webauthn_credential_user_idx ON webauthn_credential (user_id);

-- The TOTP secret is sealed with secretbox; last_step is the newest accepted
-- 30-second step, so a code of that step or an older one is never accepted again.
CREATE TABLE totp_secret (
    user_id uuid PRIMARY KEY REFERENCES users ON DELETE CASCADE,
    ciphertext bytea NOT NULL,
    key_id text NOT NULL,
    confirmed_at timestamptz,
    last_step bigint NOT NULL DEFAULT 0
);

-- Recovery codes are stored as SHA-256 only and are single use.
CREATE TABLE mfa_recovery_code (
    user_id uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    code_hash bytea NOT NULL,
    used_at timestamptz,
    PRIMARY KEY (user_id, code_hash)
);

-- +goose Down
DROP TABLE mfa_recovery_code;
DROP TABLE totp_secret;
DROP TABLE webauthn_credential;
