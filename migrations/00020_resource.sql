-- +goose Up
CREATE TABLE resource (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (name ~ '^[a-z0-9-]{1,63}$'),
    type text NOT NULL,
    category text NOT NULL,
    gateway_id uuid NULL,
    config jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name),
    UNIQUE (id, org_id)
);

CREATE TABLE resource_label (
    resource_id uuid NOT NULL,
    org_id uuid NOT NULL,
    key text NOT NULL,
    value text NOT NULL,
    PRIMARY KEY (resource_id, key),
    FOREIGN KEY (resource_id, org_id) REFERENCES resource (id, org_id) ON DELETE CASCADE
);
CREATE INDEX resource_label_selector_idx ON resource_label (org_id, key, value);

-- +goose Down
DROP TABLE resource_label;
DROP TABLE resource;
