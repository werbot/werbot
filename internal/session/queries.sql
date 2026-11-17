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
