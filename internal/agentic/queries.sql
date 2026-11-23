-- name: InsertAgent :one
-- Inserts nothing (no row) when the sponsor already has an agent of that name.
INSERT INTO agent (org_id, sponsor_user_id, name, created_by)
VALUES (@org_id, @sponsor_user_id, @name, @sponsor_user_id)
ON CONFLICT (org_id, sponsor_user_id, name) DO NOTHING
RETURNING *;

-- name: GetAgentByName :one
SELECT * FROM agent
WHERE org_id = @org_id AND sponsor_user_id = @sponsor_user_id AND name = @name;

-- name: AgentGate :one
-- The agent's state for a new run. locked: an active lock of the agent, its
-- sponsor (agents_of_user or user) or all agents; RunGate checks the same targets.
SELECT a.sponsor_user_id, a.status, a.limits,
    EXISTS (
        SELECT 1 FROM lock l
        WHERE l.org_id = a.org_id AND (l.expires_at IS NULL OR l.expires_at > now())
          AND ((l.target_type = 'agent' AND l.target_id = a.id)
            OR (l.target_type IN ('agents_of_user', 'user') AND l.target_id = a.sponsor_user_id)
            OR l.target_type = 'all_agents')
    ) AS locked
FROM agent a
WHERE a.org_id = @org_id AND a.id = @agent_id;

-- name: InsertRun :one
INSERT INTO agent_run (org_id, agent_id, sponsor_user_id, client, reason, status, max_actions, max_concurrent, expires_at)
VALUES (@org_id, @agent_id, @sponsor_user_id, @client, @reason, 'running', @max_actions, @max_concurrent,
    now() + make_interval(secs => @max_duration_secs::float8))
RETURNING *;

-- name: RunGate :one
-- A run with its agent's status. active: running and not expired. locked: an
-- active lock of the run, the agent, its sponsor (agents_of_user or user) or all agents.
SELECT sqlc.embed(r), a.status AS agent_status,
    (r.status = 'running' AND r.expires_at > now())::bool AS active,
    EXISTS (
        SELECT 1 FROM lock l
        WHERE l.org_id = r.org_id AND (l.expires_at IS NULL OR l.expires_at > now())
          AND ((l.target_type = 'agent_run' AND l.target_id = r.id)
            OR (l.target_type = 'agent' AND l.target_id = a.id)
            OR (l.target_type IN ('agents_of_user', 'user') AND l.target_id = a.sponsor_user_id)
            OR l.target_type = 'all_agents')
    ) AS locked
FROM agent_run r
JOIN agent a ON a.id = r.agent_id AND a.org_id = r.org_id
WHERE r.org_id = @org_id AND r.id = @id;

-- name: LockRun :one
SELECT * FROM agent_run WHERE org_id = @org_id AND id = @id FOR UPDATE;

-- name: EndRun :exec
UPDATE agent_run SET status = 'ended', ended_at = now(), end_reason = @end_reason
WHERE org_id = @org_id AND id = @id;

-- name: ResumeRun :exec
UPDATE agent_run SET status = 'running', denials_in_row = 0
WHERE org_id = @org_id AND id = @id;

-- name: ExpireRuns :many
-- Ends every running or paused run past expires_at, in all organizations.
UPDATE agent_run SET status = 'ended', ended_at = now(), end_reason = 'expired'
WHERE status <> 'ended' AND expires_at < now()
RETURNING org_id, id;

-- name: SnapshotAgents :many
SELECT id, sponsor_user_id, status FROM agent WHERE org_id = @org_id ORDER BY id;

-- name: BlockedRuns :many
-- Paused runs and runs ended within the last hour.
SELECT id FROM agent_run
WHERE org_id = @org_id AND (status = 'paused' OR (status = 'ended' AND ended_at > now() - interval '1 hour'))
ORDER BY id;
