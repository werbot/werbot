-- +goose Up
-- An AI agent acts for its sponsor inside one organization; deleting the sponsor's membership deletes the agent.
CREATE TABLE agent (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    sponsor_user_id uuid NOT NULL,
    name text NOT NULL CHECK (name ~ '^[a-z0-9][a-z0-9_-]{0,62}$'),
    kind text NOT NULL DEFAULT 'interactive' CHECK (kind IN ('interactive')),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'frozen')),
    limits jsonb NOT NULL DEFAULT '{}',
    created_by uuid REFERENCES users ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, sponsor_user_id, name),
    UNIQUE (id, org_id),
    FOREIGN KEY (org_id, sponsor_user_id) REFERENCES membership(org_id, user_id) ON DELETE CASCADE
);
CREATE TRIGGER agent_org_changed AFTER INSERT OR DELETE ON agent
    FOR EACH ROW EXECUTE FUNCTION werbot_notify_org_changed();
CREATE TRIGGER agent_status_changed AFTER UPDATE ON agent
    FOR EACH ROW WHEN (OLD.status IS DISTINCT FROM NEW.status) EXECUTE FUNCTION werbot_notify_org_changed();

-- +goose Down
DROP TABLE agent;
