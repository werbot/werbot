-- +goose Up
-- A resource preset names a tcp or udp base type with default ports, a
-- category and a launch hint. A resource created from a preset keeps the base
-- type and the preset id; the pool snapshot never carries presets.
CREATE TABLE resource_preset (
    id            uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id        uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    name          text NOT NULL CHECK (name ~ '^[a-z][a-z0-9-]{0,31}$'),
    base_type     text NOT NULL CHECK (base_type IN ('tcp', 'udp')),
    display_name  text NOT NULL,
    icon          text NOT NULL DEFAULT '',
    default_ports jsonb NOT NULL,
    category      text NOT NULL,
    launch_hint   text NOT NULL DEFAULT '',
    created_by    uuid NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name),
    UNIQUE (id, org_id)
);
ALTER TABLE resource ADD COLUMN preset_id uuid NULL,
    ADD CONSTRAINT resource_preset_fk FOREIGN KEY (preset_id, org_id) REFERENCES resource_preset(id, org_id) ON DELETE RESTRICT;

-- +goose Down
ALTER TABLE resource DROP COLUMN preset_id;
DROP TABLE resource_preset;
