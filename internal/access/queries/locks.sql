-- name: CreateLock :one
-- Inserts nothing (no row) when the target is not in the organization.
INSERT INTO lock (org_id, target_type, target_id, reason, expires_at, created_by)
SELECT @org_id::uuid, @target_type::text, sqlc.narg(target_id)::uuid, @reason::text,
    CASE WHEN @expires_secs::float8 > 0 THEN now() + make_interval(secs => @expires_secs::float8) END, @created_by::uuid
WHERE CASE @target_type::text
    WHEN 'user' THEN EXISTS (SELECT 1 FROM membership WHERE org_id = @org_id AND user_id = sqlc.narg(target_id))
    WHEN 'team' THEN EXISTS (SELECT 1 FROM team WHERE org_id = @org_id AND id = sqlc.narg(target_id))
    WHEN 'resource' THEN EXISTS (SELECT 1 FROM resource WHERE org_id = @org_id AND id = sqlc.narg(target_id))
    WHEN 'grant' THEN EXISTS (SELECT 1 FROM access_grant WHERE org_id = @org_id AND id = sqlc.narg(target_id))
    WHEN 'cli_session' THEN EXISTS (SELECT 1 FROM cli_session WHERE org_id = @org_id AND id = sqlc.narg(target_id))
    WHEN 'agent' THEN EXISTS (SELECT 1 FROM agent WHERE org_id = @org_id AND id = sqlc.narg(target_id))
    WHEN 'agent_run' THEN EXISTS (SELECT 1 FROM agent_run WHERE org_id = @org_id AND id = sqlc.narg(target_id))
    WHEN 'agents_of_user' THEN EXISTS (SELECT 1 FROM membership WHERE org_id = @org_id AND user_id = sqlc.narg(target_id))
    WHEN 'all_agents' THEN sqlc.narg(target_id)::uuid IS NULL
    WHEN 'mfa_device' THEN EXISTS (SELECT 1 FROM webauthn_credential w JOIN membership m ON m.user_id = w.user_id
        WHERE m.org_id = @org_id AND w.id = sqlc.narg(target_id))
    ELSE false
END
RETURNING id, org_id, target_type, target_id, reason, expires_at, created_by, created_at;

-- name: DeleteLock :one
DELETE FROM lock WHERE id = @id AND org_id = @org_id
RETURNING target_type, target_id;

-- name: ActiveLocks :many
SELECT id, org_id, target_type, target_id, reason, expires_at, created_by, created_at
FROM lock
WHERE org_id = @org_id AND (expires_at IS NULL OR expires_at > now())
ORDER BY id;

-- name: ListLocks :many
-- The active locks, newest first.
SELECT id, org_id, target_type, target_id, reason, expires_at, created_by, created_at
FROM lock
WHERE org_id = @org_id AND (expires_at IS NULL OR expires_at > now())
ORDER BY created_at DESC, id DESC;
