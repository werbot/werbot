-- name: ListMembers :many
-- Every member of the organization with role, email, login and teams.
SELECT m.user_id, m.role, u.email, u.login,
    coalesce(array_agg(tm.team_id ORDER BY tm.team_id) FILTER (WHERE tm.team_id IS NOT NULL), '{}')::uuid[] AS team_ids
FROM membership m
JOIN users u ON u.id = m.user_id
LEFT JOIN team_member tm ON tm.org_id = m.org_id AND tm.user_id = m.user_id
WHERE m.org_id = @org_id
GROUP BY m.user_id, m.role, u.email, u.login
ORDER BY m.user_id;

-- name: ListTargets :many
-- Every resource of the organization with its labels.
SELECT r.id, r.name,
    coalesce(jsonb_object_agg(l.key, l.value) FILTER (WHERE l.key IS NOT NULL), '{}')::jsonb AS labels
FROM resource r
LEFT JOIN resource_label l ON l.resource_id = r.id AND l.org_id = r.org_id
WHERE r.org_id = @org_id
GROUP BY r.id
ORDER BY r.id;

-- name: CreateApproval :one
-- A status other than pending is decided now.
INSERT INTO approval (org_id, kind, requester_user_id, agent_run_id, resource_id, label_selector, duration,
    command, command_sha256, classes, reason, policy_id, break_glass, status, decided_at, expires_at, gateway_instance_id)
VALUES (@org_id, @kind, @requester_user_id, sqlc.narg(agent_run_id), sqlc.narg(resource_id), sqlc.narg(label_selector),
    make_interval(secs => sqlc.narg(duration_secs)::float8), sqlc.narg(command), sqlc.narg(command_sha256), sqlc.narg(classes),
    @reason, sqlc.narg(policy_id), @break_glass, @status::text, CASE WHEN @status::text = 'pending' THEN NULL ELSE now() END,
    now() + make_interval(secs => @expires_secs::float8), sqlc.narg(gateway_instance_id))
RETURNING *;

-- name: GetApproval :one
SELECT * FROM approval WHERE id = @id AND org_id = @org_id;

-- name: DecideApproval :one
-- Only a pending approval before its deadline changes: one of concurrent decisions wins.
UPDATE approval SET status = @status, decided_by = @decided_by, decided_at = now(), comment = @comment
WHERE id = @id AND org_id = @org_id AND status = 'pending' AND expires_at > now()
RETURNING *;

-- name: GetApprovalState :one
SELECT status, expires_at <= now() AS overdue FROM approval WHERE id = @id AND org_id = @org_id;

-- name: CancelApproval :one
UPDATE approval SET status = 'canceled', decided_at = now()
WHERE id = @id AND org_id = @org_id AND requester_user_id = @requester_user_id AND kind = 'access' AND status = 'pending'
RETURNING id;

-- name: CreateGrants :many
-- One grant per resource; a break-glass grant needs a review.
INSERT INTO access_grant (org_id, approval_id, user_id, resource_id, expires_at, break_glass, needs_review, created_by)
SELECT @org_id::uuid, @approval_id::uuid, @user_id::uuid, r, now() + make_interval(secs => @duration_secs::float8),
    @break_glass::bool, @break_glass::bool, sqlc.narg(created_by)::uuid
FROM unnest(@resource_ids::uuid[]) AS r
RETURNING id, resource_id, expires_at;

-- name: NotifyApprovalDecided :exec
SELECT pg_notify('werbot_approval_decided', @payload::text);

-- name: ListApprovals :many
-- Newest first; a NULL row_limit lists every row.
SELECT sqlc.embed(a), u.login AS requester_login, coalesce(r.name, '')::text AS resource_name
FROM approval a
JOIN users u ON u.id = a.requester_user_id
LEFT JOIN resource r ON r.id = a.resource_id AND r.org_id = a.org_id
WHERE a.org_id = @org_id
  AND (sqlc.narg(id)::uuid IS NULL OR a.id = sqlc.narg(id)::uuid)
  AND (sqlc.narg(requester_user_id)::uuid IS NULL OR a.requester_user_id = sqlc.narg(requester_user_id)::uuid)
  AND (@status::text = '' OR a.status = @status::text)
ORDER BY a.created_at DESC, a.id DESC
LIMIT sqlc.narg(row_limit)::int OFFSET @row_offset::int;

-- name: DecidedCommands :many
-- Command decisions of one gateway instance after since, oldest first.
SELECT * FROM approval
WHERE org_id = @org_id AND kind = 'command' AND gateway_instance_id = @gateway_instance_id::uuid
  AND status IN ('approved', 'denied') AND decided_at > @since::timestamptz
ORDER BY decided_at, id;
