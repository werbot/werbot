-- +goose Up
-- An agent profile is a cli_session bound to one agent; it goes with the agent (never SET NULL: the row would become a person's session).
ALTER TABLE cli_session
    ADD COLUMN agent_id uuid,
    ADD FOREIGN KEY (agent_id, org_id) REFERENCES agent(id, org_id) ON DELETE CASCADE;
CREATE INDEX cli_session_agent_idx ON cli_session (org_id, agent_id) WHERE agent_id IS NOT NULL AND revoked_at IS NULL;

-- +goose Down
DROP INDEX cli_session_agent_idx;
ALTER TABLE cli_session DROP COLUMN agent_id;
