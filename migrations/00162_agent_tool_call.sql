-- +goose Up
-- One MCP tool call of a run.
CREATE TABLE agent_tool_call (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    org_id uuid NOT NULL REFERENCES organization(id) ON DELETE CASCADE,
    run_id uuid NOT NULL,
    tool text NOT NULL,
    arguments jsonb NOT NULL DEFAULT '{}',
    traceparent text NOT NULL DEFAULT '',
    status text NOT NULL CHECK (status IN ('running', 'ok', 'error', 'pending_approval', 'denied')),
    started_at timestamptz NOT NULL DEFAULT now(),
    ended_at timestamptz,
    UNIQUE (id, org_id),
    FOREIGN KEY (run_id, org_id) REFERENCES agent_run(id, org_id) ON DELETE CASCADE
);
CREATE INDEX agent_tool_call_run_idx ON agent_tool_call (run_id, started_at);

-- +goose Down
DROP TABLE agent_tool_call;
