-- name: InsertAgent :one
-- Inserts nothing (no row) when the sponsor already has an agent of that name.
INSERT INTO agent (org_id, sponsor_user_id, name, limits, created_by)
VALUES (@org_id, @sponsor_user_id, @name, @limits, @created_by)
ON CONFLICT (org_id, sponsor_user_id, name) DO NOTHING
RETURNING *;

-- name: GetAgentByName :one
SELECT * FROM agent
WHERE org_id = @org_id AND sponsor_user_id = @sponsor_user_id AND name = @name;

-- name: AgentGate :one
-- The agent's state for a new run. locked: an active lock of the agent, its
-- sponsor (agents_of_user or user) or all agents (agent_locked, no run).
SELECT a.sponsor_user_id, a.status, a.limits,
    agent_locked(a.org_id, a.id, a.sponsor_user_id, NULL)::bool AS locked
FROM agent a
WHERE a.org_id = @org_id AND a.id = @agent_id;

-- name: InsertRun :one
INSERT INTO agent_run (org_id, agent_id, sponsor_user_id, client, reason, status, max_actions, max_concurrent, expires_at)
VALUES (@org_id, @agent_id, @sponsor_user_id, @client, @reason, 'running', @max_actions, @max_concurrent,
    now() + make_interval(secs => @max_duration_secs::float8))
RETURNING *;

-- name: RunGate :one
-- A run with its agent's status. active: running and not expired. locked: an
-- active lock of the run, the agent, its sponsor (agents_of_user or user) or
-- all agents (agent_locked).
SELECT sqlc.embed(r), a.status AS agent_status,
    (r.status = 'running' AND r.expires_at > now())::bool AS active,
    agent_locked(r.org_id, a.id, a.sponsor_user_id, r.id)::bool AS locked
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

-- name: ActionGate :one
-- The run of a new action, locked: concurrent starts of one run (on any
-- replica) queue here, so the open-session count after it is exact. active and
-- locked are those of RunGate.
SELECT r.sponsor_user_id, r.actions, r.max_actions, r.max_concurrent, a.status AS agent_status,
    (r.status = 'running' AND r.expires_at > now())::bool AS active,
    agent_locked(r.org_id, a.id, a.sponsor_user_id, r.id)::bool AS locked
FROM agent_run r
JOIN agent a ON a.id = r.agent_id AND a.org_id = r.org_id
WHERE r.org_id = @org_id AND r.id = @id
FOR UPDATE OF r;

-- name: OpenRunSessions :one
SELECT count(*) FROM session WHERE org_id = @org_id AND agent_run_id = @run_id AND ended_at IS NULL;

-- name: CountAction :exec
-- An allowed action ends a series of denials.
UPDATE agent_run SET actions = actions + 1, denials_in_row = 0
WHERE org_id = @org_id AND id = @id;

-- name: SetDenials :exec
UPDATE agent_run SET denials_in_row = @denials_in_row, status = @status
WHERE org_id = @org_id AND id = @id;

-- name: ResourceIDByName :one
SELECT id FROM resource WHERE org_id = @org_id AND name = @name;

-- name: AlertNames :one
-- The agent name and sponsor login of a run, for agent_alert.
SELECT a.name AS agent, u.login AS sponsor
FROM agent_run r
JOIN agent a ON a.id = r.agent_id AND a.org_id = r.org_id
JOIN users u ON u.id = r.sponsor_user_id
WHERE r.org_id = @org_id AND r.id = @id;

-- name: AlertRecipients :many
-- The emails of the sponsor and of every owner and admin, each once.
SELECT DISTINCT u.email
FROM membership m
JOIN users u ON u.id = m.user_id
WHERE m.org_id = @org_id AND (m.user_id = @sponsor_user_id OR m.role IN ('owner', 'admin'))
ORDER BY u.email;

-- name: ToolCallGate :one
-- The run of a new tool call, locked: concurrent calls of one run queue here,
-- so the call count after it is exact. active: running and not expired.
SELECT (status = 'running' AND expires_at > now())::bool AS active
FROM agent_run WHERE org_id = @org_id AND id = @id
FOR UPDATE;

-- name: CountToolCalls :one
SELECT count(*) FROM agent_tool_call WHERE org_id = @org_id AND run_id = @run_id;

-- name: InsertToolCall :one
INSERT INTO agent_tool_call (org_id, run_id, tool, arguments, traceparent, status)
VALUES (@org_id, @run_id, @tool, @arguments, @traceparent, 'running')
RETURNING id;

-- name: LockToolCall :one
SELECT status FROM agent_tool_call
WHERE org_id = @org_id AND run_id = @run_id AND id = @id
FOR UPDATE;

-- name: FinishToolCall :exec
UPDATE agent_tool_call SET status = @status, ended_at = now()
WHERE org_id = @org_id AND id = @id;

-- name: ListAgents :many
-- The agents of an organization with their sponsor's login and live runs, by
-- sponsor login and name; sponsor_user_id and id narrow the list.
SELECT sqlc.embed(a), u.login AS sponsor_login,
    (SELECT count(*) FROM agent_run r
     WHERE r.org_id = a.org_id AND r.agent_id = a.id AND r.status = 'running' AND r.expires_at > now())::int AS active_runs
FROM agent a
JOIN users u ON u.id = a.sponsor_user_id
WHERE a.org_id = @org_id
  AND (sqlc.narg(sponsor_user_id)::uuid IS NULL OR a.sponsor_user_id = sqlc.narg(sponsor_user_id)::uuid)
  AND (sqlc.narg(id)::uuid IS NULL OR a.id = sqlc.narg(id)::uuid)
ORDER BY u.login, a.name;

-- name: LockAgent :one
SELECT * FROM agent WHERE org_id = @org_id AND id = @id FOR UPDATE;

-- name: UpdateAgent :exec
UPDATE agent SET limits = @limits, status = @status WHERE org_id = @org_id AND id = @id;

-- name: DeleteAgent :exec
DELETE FROM agent WHERE org_id = @org_id AND id = @id;

-- name: ListRuns :many
-- Runs with their agent's name, newest first; id, sponsor_user_id, agent_id
-- and status (empty: any) narrow the list.
SELECT sqlc.embed(r), a.name AS agent_name
FROM agent_run r
JOIN agent a ON a.id = r.agent_id AND a.org_id = r.org_id
WHERE r.org_id = @org_id
  AND (sqlc.narg(id)::uuid IS NULL OR r.id = sqlc.narg(id)::uuid)
  AND (sqlc.narg(sponsor_user_id)::uuid IS NULL OR r.sponsor_user_id = sqlc.narg(sponsor_user_id)::uuid)
  AND (sqlc.narg(agent_id)::uuid IS NULL OR r.agent_id = sqlc.narg(agent_id)::uuid)
  AND (@status::text = '' OR r.status = @status::text)
ORDER BY r.started_at DESC, r.id DESC
LIMIT @row_limit::int OFFSET @row_offset::int;

-- name: RunToolCalls :many
SELECT id, tool, arguments, traceparent, status, started_at, ended_at
FROM agent_tool_call
WHERE org_id = @org_id AND run_id = @run_id
ORDER BY started_at, id
LIMIT 1000;

-- name: RunSessions :many
SELECT s.id, coalesce(r.name, '')::text AS resource_name, s.tool_call_id, coalesce(s.command, '')::text AS command,
    coalesce(s.classes, '{}')::text[] AS classes, s.exit_code, coalesce(s.end_reason, '')::text AS end_reason,
    s.output_sha256, s.started_at, s.ended_at
FROM session s
LEFT JOIN resource r ON r.id = s.resource_id AND r.org_id = s.org_id
WHERE s.org_id = @org_id AND s.agent_run_id = @run_id
ORDER BY s.started_at, s.id
LIMIT 1000;

-- name: RunApprovals :many
SELECT a.id, coalesce(r.name, '')::text AS resource_name, coalesce(a.command, '')::text AS command,
    coalesce(a.classes, '{}')::text[] AS classes, a.status, coalesce(u.login, '')::text AS decided_by_login,
    a.created_at, a.decided_at
FROM approval a
LEFT JOIN resource r ON r.id = a.resource_id AND r.org_id = a.org_id
LEFT JOIN users u ON u.id = a.decided_by
WHERE a.org_id = @org_id AND a.agent_run_id = @run_id
ORDER BY a.created_at, a.id
LIMIT 1000;

-- name: RunDenials :many
-- The session.deny events of a run: the resource by name when it still
-- exists in the organization, else as recorded.
SELECT coalesce(r.name, e.target_id)::text AS resource, e.data, e.created_at
FROM audit_event e
LEFT JOIN resource r ON r.org_id = e.org_id AND r.id::text = e.target_id
WHERE e.org_id = @org_id AND e.action = 'session.deny' AND e.data ? 'agent_run_id'
  AND e.data->>'agent_run_id' = @run_id::text
ORDER BY e.created_at, e.id
LIMIT 1000;
