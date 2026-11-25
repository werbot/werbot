-- +goose Up
-- An OIDC identity provider of an organization. The client secret is sealed
-- with secretbox; claims_mapping maps claims to teams and roles.
CREATE TABLE idp_oidc (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 63),
    issuer text NOT NULL,
    client_id text NOT NULL,
    client_secret_ciphertext bytea NOT NULL,
    key_id text NOT NULL,
    scopes text[] NOT NULL DEFAULT '{openid,email,profile}',
    claims_mapping jsonb NOT NULL DEFAULT '[]',
    allow_signup bool NOT NULL DEFAULT false,
    enabled bool NOT NULL DEFAULT true,
    created_by uuid REFERENCES users ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name),
    UNIQUE (id, org_id)
);

-- A member's identity at an IdP of the same organization. The refresh token
-- (sealed) lets idp.recheck ask the IdP whether the user is still allowed.
CREATE TABLE user_idp_link (
    user_id uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    idp_id uuid NOT NULL,
    org_id uuid NOT NULL,
    subject text NOT NULL,
    refresh_ciphertext bytea,
    key_id text,
    last_checked_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (idp_id, subject),
    UNIQUE (idp_id, user_id),
    FOREIGN KEY (idp_id, org_id) REFERENCES idp_oidc(id, org_id) ON DELETE CASCADE,
    FOREIGN KEY (org_id, user_id) REFERENCES membership(org_id, user_id) ON DELETE CASCADE
);
CREATE INDEX user_idp_link_recheck_idx ON user_idp_link (last_checked_at) WHERE refresh_ciphertext IS NOT NULL;

-- +goose Down
DROP TABLE user_idp_link;
DROP TABLE idp_oidc;
