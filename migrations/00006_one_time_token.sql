-- +goose Up
CREATE TABLE one_time_token (
    token_hash bytea PRIMARY KEY,
    kind text NOT NULL CHECK (kind IN ('invite', 'password_reset', 'account_deletion')),
    org_id uuid REFERENCES organization(id) ON DELETE CASCADE,
    user_id uuid REFERENCES users(id) ON DELETE CASCADE,
    email text, role text CHECK (role IN ('owner', 'admin', 'member')),
    expires_at timestamptz NOT NULL, used_at timestamptz, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX one_time_token_open_idx ON one_time_token (org_id, kind) WHERE used_at IS NULL;

-- +goose Down
DROP TABLE one_time_token;
