-- +goose Up
-- A gateway is a pool: every gateway process is one gateway_instance row and
-- all instances share the pool host key.
CREATE TABLE gateway (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 63),
    host_key_ciphertext bytea NOT NULL,
    host_key_key_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name),
    UNIQUE (id, org_id)
);

CREATE TABLE gateway_instance (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    gateway_id uuid NOT NULL,
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    hostname text NOT NULL,
    cert_serial text NOT NULL,
    cert_expires_at timestamptz NOT NULL,
    version text NOT NULL DEFAULT '',
    connectors text[] NOT NULL DEFAULT '{}',
    last_seen_at timestamptz,
    client_ip inet,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (gateway_id, org_id) REFERENCES gateway(id, org_id) ON DELETE CASCADE
);
CREATE INDEX gateway_instance_gateway_idx ON gateway_instance (gateway_id);

CREATE TABLE gateway_join_token (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    token_hash bytea NOT NULL UNIQUE,
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    gateway_id uuid NOT NULL,
    max_uses int NOT NULL CHECK (max_uses BETWEEN 1 AND 1000),
    uses int NOT NULL DEFAULT 0,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_by uuid REFERENCES users(id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (gateway_id, org_id) REFERENCES gateway(id, org_id) ON DELETE CASCADE
);
CREATE INDEX gateway_join_token_gateway_idx ON gateway_join_token (gateway_id);

-- There were no pools before this migration. The composite key keeps a
-- resource on a pool of its own organization.
UPDATE resource SET gateway_id = NULL;
ALTER TABLE resource ADD CONSTRAINT resource_gateway_fk
    FOREIGN KEY (gateway_id, org_id) REFERENCES gateway(id, org_id) ON DELETE RESTRICT;

-- Only deletions change snapshots: heartbeats update gateway_instance every 15s.
CREATE TRIGGER gateway_org_changed AFTER DELETE ON gateway FOR EACH ROW EXECUTE FUNCTION werbot_notify_org_changed();
CREATE TRIGGER gateway_instance_org_changed AFTER DELETE ON gateway_instance FOR EACH ROW EXECUTE FUNCTION werbot_notify_org_changed();

-- +goose Down
DROP TRIGGER gateway_instance_org_changed ON gateway_instance;
DROP TRIGGER gateway_org_changed ON gateway;
ALTER TABLE resource DROP CONSTRAINT resource_gateway_fk;
DROP TABLE gateway_join_token;
DROP TABLE gateway_instance;
DROP TABLE gateway;
