-- +goose Up
CREATE TABLE audit_event (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid REFERENCES organization(id) ON DELETE CASCADE,
    actor_user_id uuid,
    actor_kind text NOT NULL CHECK (actor_kind IN ('user', 'gateway', 'system')),
    action text NOT NULL CHECK (action <> ''),
    target_type text NOT NULL DEFAULT '', target_id text NOT NULL DEFAULT '',
    ip inet, data jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_event_org_created_idx ON audit_event (org_id, created_at DESC);

-- +goose Down
DROP TABLE audit_event;
