-- +goose Up
-- One row per SSH session, owned by the gateway instance that serves it.
CREATE TABLE session (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    user_id uuid REFERENCES users(id) ON DELETE SET NULL,
    resource_id uuid REFERENCES resource(id) ON DELETE SET NULL,
    gateway_id uuid REFERENCES gateway(id) ON DELETE SET NULL,
    gateway_instance_id uuid REFERENCES gateway_instance(id) ON DELETE SET NULL,
    client_ip inet NOT NULL,
    started_at timestamptz NOT NULL DEFAULT now(),
    ended_at timestamptz,
    end_reason text,
    CHECK ((ended_at IS NULL) = (end_reason IS NULL))
);
CREATE INDEX session_org_started_idx ON session (org_id, started_at DESC, id DESC);
CREATE INDEX session_open_instance_idx ON session (gateway_instance_id) WHERE ended_at IS NULL;

-- +goose Down
DROP TABLE session;
