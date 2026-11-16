-- +goose Up
CREATE TABLE user_ssh_key (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (length(name) BETWEEN 1 AND 128),
    public_key text NOT NULL,
    fingerprint_sha256 text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX user_ssh_key_user_idx ON user_ssh_key (user_id);

-- +goose Down
DROP TABLE user_ssh_key;
