-- +goose Up
CREATE TABLE resource_host_key (
    resource_id uuid PRIMARY KEY,
    org_id uuid NOT NULL,
    key_type text NOT NULL,
    public_key text NOT NULL,
    fingerprint_sha256 text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (resource_id, org_id) REFERENCES resource (id, org_id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE resource_host_key;
