-- name: StartSession :one
INSERT INTO session (org_id, user_id, resource_id, gateway_id, gateway_instance_id, client_ip)
VALUES (@org_id, @user_id, @resource_id, @gateway_id, @gateway_instance_id, @client_ip)
RETURNING id;

-- name: EndSession :one
UPDATE session
SET ended_at = now(), end_reason = @reason::text
WHERE id = @id AND org_id = @org_id AND gateway_instance_id = @instance_id::uuid AND ended_at IS NULL
RETURNING resource_id;

-- name: SessionOfInstanceExists :one
SELECT EXISTS (SELECT 1 FROM session WHERE id = @id AND org_id = @org_id AND gateway_instance_id = @instance_id::uuid);

-- name: EndMissingSessions :many
UPDATE session
SET ended_at = now(), end_reason = @reason::text
WHERE org_id = @org_id AND gateway_instance_id = @instance_id::uuid AND ended_at IS NULL
  AND started_at < now() - interval '1 minute'
  -- a hashed subplan, O(rows + ids) under a generic plan too. NULL closes all.
  AND id NOT IN (SELECT unnest(@active::uuid[]))
RETURNING id, resource_id;

-- name: EndLostSessions :many
-- Sessions of a deleted instance, or of an instance silent for lost_after_secs.
UPDATE session s
SET ended_at = now(), end_reason = 'gateway_lost'
WHERE s.ended_at IS NULL
  AND (s.gateway_instance_id IS NULL OR EXISTS (
        SELECT 1 FROM gateway_instance i
        WHERE i.id = s.gateway_instance_id
          AND coalesce(i.last_seen_at, i.created_at) < now() - make_interval(secs => @lost_after_secs::float8)))
RETURNING s.id, s.org_id, s.resource_id;

-- name: GetSigningKey :one
SELECT ciphertext, key_id, public_key FROM signing_key WHERE id = 1;

-- name: InsertSigningKey :exec
INSERT INTO signing_key (id, ciphertext, key_id, public_key)
VALUES (1, @ciphertext, @key_id, @public_key)
ON CONFLICT (id) DO NOTHING;

-- name: CreateRecording :exec
INSERT INTO recording (session_id, org_id, store)
VALUES (@session_id, @org_id, @store)
ON CONFLICT DO NOTHING;

-- name: LockRecording :one
-- The row lock orders concurrent ingest of one session across server replicas.
SELECT chunks, bytes, last_hash, final_hash
FROM recording
WHERE session_id = @session_id AND org_id = @org_id
FOR UPDATE;

-- name: AppendRecordingChunk :exec
UPDATE recording
SET chunks = chunks + 1, bytes = bytes + @size::bigint, last_hash = @last_hash
WHERE session_id = @session_id AND org_id = @org_id;

-- name: FinishRecording :exec
UPDATE recording
SET final_hash = @final_hash, manifest_signature = @manifest_signature, verified_at = now()
WHERE session_id = @session_id AND org_id = @org_id;

-- name: GetRecording :one
SELECT store, chunks, bytes, final_hash, manifest_signature
FROM recording
WHERE session_id = @session_id AND org_id = @org_id;

-- name: ListSessions :many
-- LEFT JOINs keep the history of deleted users and resources.
SELECT s.id, s.user_id, s.resource_id, s.gateway_id, s.client_ip, s.started_at, s.ended_at,
       coalesce(u.login, '')::text AS user_login,
       coalesce(r.name, '')::text AS resource_name,
       coalesce(s.end_reason, '')::text AS end_reason,
       coalesce(rec.chunks, 0)::int AS chunks,
       coalesce(rec.bytes, 0)::bigint AS bytes,
       (rec.manifest_signature IS NOT NULL)::bool AS finished
FROM session s
LEFT JOIN users u ON u.id = s.user_id
LEFT JOIN resource r ON r.id = s.resource_id AND r.org_id = s.org_id
LEFT JOIN recording rec ON rec.session_id = s.id AND rec.org_id = s.org_id
WHERE s.org_id = @org_id
  AND (sqlc.narg(user_id)::uuid IS NULL OR s.user_id = sqlc.narg(user_id))
  AND (sqlc.narg(resource_id)::uuid IS NULL OR s.resource_id = sqlc.narg(resource_id))
  AND (sqlc.narg(from_ts)::timestamptz IS NULL OR s.started_at >= sqlc.narg(from_ts))
  AND (sqlc.narg(to_ts)::timestamptz IS NULL OR s.started_at < sqlc.narg(to_ts))
  AND (@status::text = '' OR (@status::text = 'active') = (s.ended_at IS NULL))
  AND (sqlc.narg(id)::uuid IS NULL OR s.id = sqlc.narg(id))
ORDER BY s.started_at DESC, s.id DESC
LIMIT @row_limit::int OFFSET @row_offset::int;
