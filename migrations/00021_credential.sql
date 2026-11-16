-- +goose Up
CREATE TABLE credential (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL,
    resource_id uuid NOT NULL UNIQUE,
    kind text NOT NULL CHECK (kind IN ('password', 'private_key')),
    ciphertext bytea NOT NULL,
    key_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (resource_id, org_id) REFERENCES resource (id, org_id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE credential;
