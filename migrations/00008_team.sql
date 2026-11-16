-- +goose Up
CREATE TABLE team (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 64),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name),
    UNIQUE (id, org_id)
);

-- +goose Down
DROP TABLE team;
