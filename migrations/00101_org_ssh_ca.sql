-- +goose Up
-- The organization's user CA: signs the short-lived SSH certificates of CLI users.
CREATE TABLE org_ssh_ca (
    org_id uuid PRIMARY KEY REFERENCES organization(id) ON DELETE CASCADE,
    ciphertext bytea NOT NULL,
    key_id text NOT NULL,
    public_key text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE org_ssh_ca;
