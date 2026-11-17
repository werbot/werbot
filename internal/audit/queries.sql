-- name: ListAuditEvents :many
-- The LEFT JOIN keeps events of deleted users. Events without an org_id never match.
SELECT e.id, e.actor_user_id, coalesce(u.login, '')::text AS actor_login, e.actor_kind, e.action,
       e.target_type, e.target_id, e.ip, e.data, e.created_at
FROM audit_event e
LEFT JOIN users u ON u.id = e.actor_user_id
WHERE e.org_id = @org_id
  AND (sqlc.narg(id)::uuid IS NULL OR e.id = sqlc.narg(id))
  AND (@action::text = '' OR e.action = @action OR starts_with(e.action, @action || '.'))
  AND (sqlc.narg(actor_user_id)::uuid IS NULL OR e.actor_user_id = sqlc.narg(actor_user_id))
  AND (@target_type::text = '' OR e.target_type = @target_type)
  AND (@target_id::text = '' OR e.target_id = @target_id)
  AND (sqlc.narg(from_ts)::timestamptz IS NULL OR e.created_at >= sqlc.narg(from_ts))
  AND (sqlc.narg(to_ts)::timestamptz IS NULL OR e.created_at < sqlc.narg(to_ts))
ORDER BY e.created_at DESC, e.id DESC
LIMIT @row_limit::int OFFSET @row_offset::int;
