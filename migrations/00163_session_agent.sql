-- +goose Up
-- Link sessions and approvals to the agent run and tool call that caused them; history survives their deletion.
ALTER TABLE session
    ADD COLUMN agent_run_id uuid,
    ADD COLUMN tool_call_id uuid,
    ADD COLUMN exit_code int,
    ADD COLUMN output_sha256 bytea CHECK (output_sha256 IS NULL OR length(output_sha256) = 32),
    ADD FOREIGN KEY (agent_run_id, org_id) REFERENCES agent_run(id, org_id) ON DELETE SET NULL (agent_run_id),
    ADD FOREIGN KEY (tool_call_id, org_id) REFERENCES agent_tool_call(id, org_id) ON DELETE SET NULL (tool_call_id);
CREATE INDEX session_agent_run_open_idx ON session (agent_run_id) WHERE ended_at IS NULL AND agent_run_id IS NOT NULL;
CREATE INDEX session_agent_run_idx ON session (org_id, agent_run_id, started_at) WHERE agent_run_id IS NOT NULL;
ALTER TABLE approval
    ADD FOREIGN KEY (agent_run_id, org_id) REFERENCES agent_run(id, org_id) ON DELETE SET NULL (agent_run_id);
CREATE INDEX approval_agent_reuse_idx ON approval (org_id, agent_run_id, resource_id, command_sha256)
    WHERE kind = 'command' AND agent_run_id IS NOT NULL;
CREATE INDEX audit_event_agent_run_idx ON audit_event (org_id, (data->>'agent_run_id'), created_at)
    WHERE action = 'session.deny' AND data ? 'agent_run_id';

-- +goose Down
DROP INDEX audit_event_agent_run_idx;
DROP INDEX approval_agent_reuse_idx;
ALTER TABLE approval DROP CONSTRAINT approval_agent_run_id_org_id_fkey;
DROP INDEX session_agent_run_idx;
DROP INDEX session_agent_run_open_idx;
ALTER TABLE session
    DROP COLUMN tool_call_id,
    DROP COLUMN agent_run_id,
    DROP COLUMN output_sha256,
    DROP COLUMN exit_code;
