-- +goose Up
-- Last probe result of a resource from one gateway instance.
CREATE TABLE resource_health (
    resource_id uuid NOT NULL,
    gateway_instance_id uuid NOT NULL REFERENCES gateway_instance(id) ON DELETE CASCADE,
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    ok bool NOT NULL,
    error text NOT NULL DEFAULT '',
    checked_at timestamptz NOT NULL,
    PRIMARY KEY (resource_id, gateway_instance_id),
    FOREIGN KEY (resource_id, org_id) REFERENCES resource(id, org_id) ON DELETE CASCADE
);
CREATE INDEX resource_health_org_id_idx ON resource_health (org_id);
CREATE INDEX resource_health_instance_idx ON resource_health (gateway_instance_id);

-- +goose Down
DROP TABLE resource_health;
