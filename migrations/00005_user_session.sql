-- +goose Up
CREATE TABLE user_session (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    token_hash bytea NOT NULL UNIQUE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL DEFAULT now(), ip inet, user_agent text NOT NULL DEFAULT '',
    FOREIGN KEY (org_id, user_id) REFERENCES membership (org_id, user_id) ON DELETE CASCADE
);
CREATE INDEX user_session_user_idx ON user_session (user_id);

-- +goose Down
DROP TABLE user_session;
