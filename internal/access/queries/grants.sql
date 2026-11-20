-- name: ActiveGrants :many
SELECT id, org_id, user_id, resource_id, approval_id, expires_at, break_glass, needs_review
FROM access_grant
WHERE org_id = @org_id AND revoked_at IS NULL AND expires_at > now()
ORDER BY id;

-- name: ListGrants :many
-- Newest first; NULL filters match every grant.
-- ponytail: no paging; add it when an organization keeps many thousands of grants.
SELECT g.id, g.org_id, g.user_id, g.resource_id, g.approval_id, g.expires_at, g.break_glass, g.needs_review,
    g.revoked_at, g.created_by, g.created_at, u.login AS user_login, r.name AS resource_name
FROM access_grant g
JOIN users u ON u.id = g.user_id
JOIN resource r ON r.id = g.resource_id AND r.org_id = g.org_id
WHERE g.org_id = @org_id
  AND (sqlc.narg(user_id)::uuid IS NULL OR g.user_id = sqlc.narg(user_id)::uuid)
  AND (sqlc.narg(resource_id)::uuid IS NULL OR g.resource_id = sqlc.narg(resource_id)::uuid)
  AND (NOT @active::bool OR (g.revoked_at IS NULL AND g.expires_at > now()))
ORDER BY g.created_at DESC, g.id DESC;

-- name: GrantForUpdate :one
-- The grant, row-locked until the transaction ends.
SELECT user_id, resource_id, (revoked_at IS NOT NULL)::bool AS revoked, break_glass, needs_review
FROM access_grant WHERE id = @id AND org_id = @org_id
FOR UPDATE;

-- name: RevokeGrant :exec
UPDATE access_grant SET revoked_at = now() WHERE id = @id AND org_id = @org_id;

-- name: ReviewGrant :exec
UPDATE access_grant SET needs_review = false WHERE id = @id AND org_id = @org_id;
