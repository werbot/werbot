-- +goose Up
-- Locks deny access to a user, team, resource, grant, agent, CLI session or MFA device.
CREATE TABLE lock (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    target_type text NOT NULL CHECK (target_type IN
        ('user', 'team', 'resource', 'grant', 'agent', 'agent_run', 'agents_of_user', 'all_agents', 'cli_session', 'mfa_device')),
    target_id uuid CHECK ((target_type = 'all_agents') = (target_id IS NULL)),
    reason text NOT NULL CHECK (reason <> ''),
    expires_at timestamptz,
    created_by uuid REFERENCES users ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX lock_target_idx ON lock (org_id, target_type, target_id);
CREATE TRIGGER lock_org_changed AFTER INSERT OR UPDATE OR DELETE ON lock FOR EACH ROW EXECUTE FUNCTION werbot_notify_org_changed();

-- +goose Down
DROP TABLE lock;
