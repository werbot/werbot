-- +goose Up
-- One row per server replica, the only installation-level table (no org_id).
-- A replica upserts its own row every 15s; online is last_seen_at under 45s.
CREATE TABLE server_instance (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    hostname text NOT NULL,
    internal_url text NOT NULL,
    version text NOT NULL DEFAULT '',
    started_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE server_instance;
