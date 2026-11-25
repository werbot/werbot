-- +goose Up
-- A passed second factor of a member, bound to a key, certificate, web or CLI
-- session (binding). mfa.gc deletes rows older than 24h.
CREATE TABLE mfa_verification (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL,
    user_id uuid NOT NULL,
    binding text NOT NULL,
    method text NOT NULL CHECK (method IN ('webauthn', 'totp')),
    verified_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (org_id, user_id) REFERENCES membership(org_id, user_id) ON DELETE CASCADE
);
CREATE INDEX mfa_verification_user_idx ON mfa_verification (user_id, binding, verified_at DESC);

-- An MFA challenge a gateway instance (NULL for a step-up) waits on. Every
-- reference stays inside the challenge's organization.
CREATE TABLE mfa_challenge (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL,
    user_id uuid NOT NULL,
    code text NOT NULL,
    ssh_session_id bytea,
    resource_id uuid,
    client_ip inet,
    binding text NOT NULL,
    strength text NOT NULL CHECK (strength IN ('any', 'webauthn')),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'confirmed', 'denied', 'expired')),
    expires_at timestamptz NOT NULL,
    confirmed_at timestamptz,
    gateway_instance_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (org_id, user_id) REFERENCES membership(org_id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (gateway_instance_id, org_id) REFERENCES gateway_instance(id, org_id) ON DELETE CASCADE,
    FOREIGN KEY (resource_id, org_id) REFERENCES resource(id, org_id) ON DELETE SET NULL (resource_id)
);
CREATE INDEX mfa_challenge_confirmed_idx ON mfa_challenge (gateway_instance_id, expires_at) WHERE status = 'confirmed';

-- +goose Down
DROP TABLE mfa_challenge;
DROP TABLE mfa_verification;
