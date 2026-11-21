-- +goose Up
-- OAuth authorization server for the CLI: one signing key per installation,
-- short-lived authorization and device requests, and the CLI sessions they
-- mint. Codes and refresh tokens are stored only as SHA-256 hashes.
CREATE TABLE oauth_signing_key (
    id smallint PRIMARY KEY CHECK (id = 1),
    ciphertext bytea NOT NULL,
    key_id text NOT NULL,
    public_jwk jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE oauth_auth_request (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    client_id text NOT NULL,
    redirect_uri text NOT NULL,
    code_challenge text NOT NULL,
    scopes text[] NOT NULL,
    state text NOT NULL DEFAULT '',
    client_hostname text NOT NULL DEFAULT '',
    client_ip inet,
    user_id uuid,
    org_id uuid REFERENCES organization(id) ON DELETE CASCADE,
    code_hash bytea UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (org_id, user_id) REFERENCES membership(org_id, user_id) ON DELETE CASCADE
);

CREATE TABLE oauth_device_code (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    device_code_hash bytea NOT NULL UNIQUE,
    user_code text NOT NULL UNIQUE,
    client_id text NOT NULL,
    scopes text[] NOT NULL,
    client_hostname text NOT NULL DEFAULT '',
    client_ip inet,
    user_id uuid,
    org_id uuid REFERENCES organization(id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'denied', 'used')),
    last_poll_at timestamptz,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (org_id, user_id) REFERENCES membership(org_id, user_id) ON DELETE CASCADE
);

CREATE TABLE cli_session (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    user_id uuid NOT NULL,
    client_id text NOT NULL,
    hostname text NOT NULL DEFAULT '',
    ip inet,
    refresh_hash bytea NOT NULL UNIQUE,
    -- the hash before the last rotation: presenting it again revokes the session
    prev_refresh_hash bytea,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    absolute_expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    revoke_reason text,
    FOREIGN KEY (org_id, user_id) REFERENCES membership(org_id, user_id) ON DELETE CASCADE
);
CREATE INDEX cli_session_user_idx ON cli_session (org_id, user_id) WHERE revoked_at IS NULL;
CREATE INDEX cli_session_prev_idx ON cli_session (prev_refresh_hash);

-- +goose Down
DROP TABLE cli_session;
DROP TABLE oauth_device_code;
DROP TABLE oauth_auth_request;
DROP TABLE oauth_signing_key;
