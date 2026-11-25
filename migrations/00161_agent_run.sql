-- +goose Up
-- One bounded working period of an agent: action budget, concurrency cap and a hard expiry.
CREATE TABLE agent_run (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    agent_id uuid NOT NULL,
    sponsor_user_id uuid NOT NULL,
    client text NOT NULL,
    reason text NOT NULL DEFAULT '',
    status text NOT NULL CHECK (status IN ('running', 'paused', 'ended')),
    actions int NOT NULL DEFAULT 0,
    denials_in_row int NOT NULL DEFAULT 0,
    max_actions int NOT NULL,
    max_concurrent int NOT NULL,
    expires_at timestamptz NOT NULL,
    started_at timestamptz NOT NULL DEFAULT now(),
    ended_at timestamptz,
    end_reason text,
    CHECK ((status = 'ended') = (ended_at IS NOT NULL)),
    UNIQUE (id, org_id),
    FOREIGN KEY (agent_id, org_id) REFERENCES agent(id, org_id) ON DELETE CASCADE
);
CREATE INDEX agent_run_agent_idx ON agent_run (org_id, agent_id, started_at DESC);
CREATE INDEX agent_run_live_idx ON agent_run (expires_at) WHERE status <> 'ended';
-- A new run does not change the snapshot; only a status change does.
CREATE TRIGGER agent_run_status_changed AFTER UPDATE ON agent_run
    FOR EACH ROW WHEN (OLD.status IS DISTINCT FROM NEW.status) EXECUTE FUNCTION werbot_notify_org_changed();

-- An active lock of the run, the agent, its sponsor (agents_of_user or user)
-- or all agents; run_id NULL checks the agent only. The targets mirror
-- policy.agentLockTerms: change both together.
-- +goose StatementBegin
CREATE FUNCTION agent_locked(org_id uuid, agent_id uuid, sponsor_id uuid, run_id uuid) RETURNS bool
LANGUAGE sql STABLE AS $$
    SELECT EXISTS (
        SELECT 1 FROM lock l
        WHERE l.org_id = agent_locked.org_id AND (l.expires_at IS NULL OR l.expires_at > now())
          AND ((l.target_type = 'agent_run' AND l.target_id = agent_locked.run_id)
            OR (l.target_type = 'agent' AND l.target_id = agent_locked.agent_id)
            OR (l.target_type IN ('agents_of_user', 'user') AND l.target_id = agent_locked.sponsor_id)
            OR l.target_type = 'all_agents'))
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION agent_locked(uuid, uuid, uuid, uuid);
DROP TABLE agent_run;
