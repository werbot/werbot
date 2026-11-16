-- +goose Up
CREATE TABLE organization (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    name       text NOT NULL CHECK (length(name) BETWEEN 1 AND 128),
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE organization;
