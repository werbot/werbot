-- name: CreateResource :one
INSERT INTO resource (org_id, name, type, category, gateway_id, config, idle_timeout, max_duration)
VALUES (@org_id, @name, @type, @category, @gateway_id, @config,
        make_interval(secs => sqlc.narg(idle_secs)::bigint), make_interval(secs => sqlc.narg(max_secs)::bigint))
RETURNING id;

-- name: UpdateResource :execrows
UPDATE resource
SET name = @name, gateway_id = @gateway_id, config = coalesce(sqlc.narg(config)::jsonb, config),
    idle_timeout = make_interval(secs => sqlc.narg(idle_secs)::bigint),
    max_duration = make_interval(secs => sqlc.narg(max_secs)::bigint), updated_at = now()
WHERE id = @id AND org_id = @org_id;

-- name: DeleteResource :execrows
DELETE FROM resource WHERE id = @id AND org_id = @org_id;

-- name: GetResource :one
-- idle_secs and max_secs are 0 when unset (the valid minimum is 60);
-- health_checked_at is the epoch while the health is unknown.
SELECT r.id, r.org_id, r.name, r.type, r.category, r.gateway_id, r.config, r.created_at, r.updated_at,
       coalesce((SELECT jsonb_object_agg(l.key, l.value) FROM resource_label l WHERE l.resource_id = r.id), '{}')::jsonb AS labels,
       h.key_type, h.public_key, h.fingerprint_sha256, h.created_at AS host_key_created_at,
       coalesce(extract(epoch FROM r.idle_timeout), 0)::bigint AS idle_secs,
       coalesce(extract(epoch FROM r.max_duration), 0)::bigint AS max_secs,
       CASE WHEN hs.n = 0 THEN 'unknown' WHEN hs.any_ok THEN 'healthy' ELSE 'unhealthy' END::text AS health_state,
       coalesce(hs.checked_at, 'epoch'::timestamptz)::timestamptz AS health_checked_at,
       coalesce(hs.last_error, '')::text AS health_error
FROM resource r
LEFT JOIN resource_host_key h ON h.resource_id = r.id
-- Fresh (3 min) probes of the online instances of the resource's pool.
LEFT JOIN LATERAL (
    SELECT count(*) AS n, coalesce(bool_or(rh.ok), false) AS any_ok, max(rh.checked_at) AS checked_at,
           (array_agg(rh.error ORDER BY rh.checked_at DESC) FILTER (WHERE NOT rh.ok))[1] AS last_error
    FROM resource_health rh
    JOIN gateway_instance gi ON gi.id = rh.gateway_instance_id AND gi.org_id = r.org_id AND gi.gateway_id = r.gateway_id
    WHERE rh.resource_id = r.id AND rh.org_id = r.org_id
      AND gi.last_seen_at > now() - interval '45 seconds'
      AND rh.checked_at > now() - interval '3 minutes'
) hs ON true
WHERE r.id = @id AND r.org_id = @org_id;

-- name: ListResources :many
-- An empty label_filter matches everything; a NULL gateway_id means any gateway.
SELECT r.id, r.org_id, r.name, r.type, r.category, r.gateway_id, r.config, r.created_at, r.updated_at,
       coalesce((SELECT jsonb_object_agg(l.key, l.value) FROM resource_label l WHERE l.resource_id = r.id), '{}')::jsonb AS labels,
       h.key_type, h.public_key, h.fingerprint_sha256, h.created_at AS host_key_created_at,
       coalesce(extract(epoch FROM r.idle_timeout), 0)::bigint AS idle_secs,
       coalesce(extract(epoch FROM r.max_duration), 0)::bigint AS max_secs,
       CASE WHEN hs.n = 0 THEN 'unknown' WHEN hs.any_ok THEN 'healthy' ELSE 'unhealthy' END::text AS health_state,
       coalesce(hs.checked_at, 'epoch'::timestamptz)::timestamptz AS health_checked_at,
       coalesce(hs.last_error, '')::text AS health_error
FROM resource r
LEFT JOIN resource_host_key h ON h.resource_id = r.id
-- Fresh (3 min) probes of the online instances of the resource's pool.
LEFT JOIN LATERAL (
    SELECT count(*) AS n, coalesce(bool_or(rh.ok), false) AS any_ok, max(rh.checked_at) AS checked_at,
           (array_agg(rh.error ORDER BY rh.checked_at DESC) FILTER (WHERE NOT rh.ok))[1] AS last_error
    FROM resource_health rh
    JOIN gateway_instance gi ON gi.id = rh.gateway_instance_id AND gi.org_id = r.org_id AND gi.gateway_id = r.gateway_id
    WHERE rh.resource_id = r.id AND rh.org_id = r.org_id
      AND gi.last_seen_at > now() - interval '45 seconds'
      AND rh.checked_at > now() - interval '3 minutes'
) hs ON true
WHERE r.org_id = @org_id
  AND (sqlc.narg(gateway_id)::uuid IS NULL OR r.gateway_id = sqlc.narg(gateway_id)::uuid)
  AND NOT EXISTS (
      SELECT 1 FROM jsonb_each_text(@label_filter::jsonb) f
      WHERE NOT EXISTS (
          SELECT 1 FROM resource_label l
          WHERE l.resource_id = r.id AND l.key = f.key AND l.value = f.value))
ORDER BY r.name;

-- name: DeleteLabels :exec
DELETE FROM resource_label WHERE resource_id = @resource_id AND org_id = @org_id;

-- name: InsertLabels :exec
INSERT INTO resource_label (resource_id, org_id, key, value)
SELECT @resource_id::uuid, @org_id::uuid, key, value FROM jsonb_each_text(@labels::jsonb);

-- name: InsertHostKey :execrows
INSERT INTO resource_host_key (resource_id, org_id, key_type, public_key, fingerprint_sha256)
VALUES (@resource_id, @org_id, @key_type, @public_key, @fingerprint_sha256)
ON CONFLICT (resource_id) DO NOTHING;

-- name: GetHostKeyFingerprint :one
SELECT fingerprint_sha256 FROM resource_host_key WHERE resource_id = @resource_id AND org_id = @org_id;

-- name: DeleteHostKey :execrows
DELETE FROM resource_host_key WHERE resource_id = @resource_id AND org_id = @org_id;

-- name: OrgGatewayIDs :many
-- Two rows are enough to tell "none", "the only one" and "several".
SELECT id FROM gateway WHERE org_id = @org_id ORDER BY created_at, id LIMIT 2;

-- name: LockOrgPools :exec
-- Held from the creation of the first pool to commit, so a resource saved
-- meanwhile either sees the pool or is assigned to it. The two-key form
-- (namespace WERB) never collides with the bigint keys of db.BootstrapLock.
SELECT pg_advisory_xact_lock(1464160834, hashtext(CAST(@org_id::uuid AS text)));

-- name: LockOrgPoolsShared :exec
SELECT pg_advisory_xact_lock_shared(1464160834, hashtext(CAST(@org_id::uuid AS text)));

-- name: CreateGateway :one
INSERT INTO gateway (org_id, name, host_key_ciphertext, host_key_key_id)
VALUES (@org_id, @name, @host_key_ciphertext, @host_key_key_id)
RETURNING id, created_at;

-- name: CreateGatewayIfAbsent :one
-- No row when the pool already exists.
INSERT INTO gateway (org_id, name, host_key_ciphertext, host_key_key_id)
VALUES (@org_id, @name, @host_key_ciphertext, @host_key_key_id)
ON CONFLICT (org_id, name) DO NOTHING
RETURNING id;

-- name: GetGatewayIDByName :one
SELECT id FROM gateway WHERE org_id = @org_id AND name = @name;

-- name: AssignUnpooledResources :execrows
-- Only while gateway_id is the single pool of the organization.
UPDATE resource r SET gateway_id = @gateway_id::uuid, updated_at = now()
WHERE r.org_id = @org_id AND r.gateway_id IS NULL
  AND (SELECT count(*) FROM gateway g WHERE g.org_id = @org_id) = 1;

-- name: ListGateways :many
SELECT id, org_id, name, created_at FROM gateway WHERE org_id = @org_id ORDER BY name;

-- name: DeleteGateway :execrows
DELETE FROM gateway WHERE id = @id AND org_id = @org_id;

-- name: GetGatewayHostKey :one
SELECT host_key_ciphertext, host_key_key_id FROM gateway WHERE id = @id AND org_id = @org_id;

-- name: ListInstances :many
SELECT id, gateway_id, org_id, hostname, version, cert_serial, connectors, client_ip,
       cert_expires_at, created_at, last_seen_at,
       coalesce(last_seen_at > now() - interval '45 seconds', false)::bool AS online
FROM gateway_instance
WHERE org_id = @org_id
ORDER BY created_at, id;

-- name: AuthenticateInstance :one
-- The instance id comes from a verified certificate: no org filter. The previous
-- serial matches until the current one is first used; that use clears it.
WITH first_use AS (
    UPDATE gateway_instance u SET prev_cert_serial = NULL
    WHERE u.id = @id AND u.gateway_id = @gateway_id AND u.cert_serial = @cert_serial AND u.prev_cert_serial IS NOT NULL
)
SELECT i.id, i.gateway_id, i.org_id, i.hostname, i.version, i.cert_serial, i.connectors, i.client_ip,
       i.cert_expires_at, i.created_at, i.last_seen_at,
       coalesce(i.last_seen_at > now() - interval '45 seconds', false)::bool AS online
FROM gateway_instance i
WHERE i.id = @id AND i.gateway_id = @gateway_id AND @cert_serial IN (i.cert_serial, i.prev_cert_serial);

-- name: CreateInstance :exec
INSERT INTO gateway_instance (id, gateway_id, org_id, hostname, cert_serial, cert_expires_at, version, connectors, client_ip)
VALUES (@id, @gateway_id, @org_id, @hostname, @cert_serial, @cert_expires_at, @version, @connectors, @client_ip);

-- name: GetInstanceGatewayID :one
SELECT gateway_id FROM gateway_instance WHERE id = @id FOR UPDATE;

-- name: RenewInstanceCert :exec
-- A set prev_cert_serial means the current serial was never used: the instance
-- still holds the previous one, so it stays valid.
UPDATE gateway_instance
SET prev_cert_serial = coalesce(prev_cert_serial, cert_serial), cert_serial = @cert_serial, cert_expires_at = @cert_expires_at
WHERE id = @id;

-- name: HeartbeatInstance :execrows
UPDATE gateway_instance SET last_seen_at = now(), version = @version, connectors = @connectors WHERE id = @id;

-- name: DeleteInstance :one
DELETE FROM gateway_instance WHERE id = @id AND org_id = @org_id RETURNING gateway_id;

-- name: DeleteExpiredInstances :execrows
DELETE FROM gateway_instance WHERE cert_expires_at < now() - interval '24 hours';

-- name: CreateJoinToken :one
INSERT INTO gateway_join_token (token_hash, org_id, gateway_id, max_uses, expires_at, created_by)
VALUES (@token_hash, @org_id, @gateway_id, @max_uses, now() + make_interval(secs => @ttl_seconds::float8), @created_by)
RETURNING id, expires_at;

-- name: ListJoinTokens :many
SELECT id, max_uses, uses, expires_at, revoked_at, created_by, created_at
FROM gateway_join_token
WHERE gateway_id = @gateway_id AND org_id = @org_id
ORDER BY created_at, id;

-- name: RevokeJoinToken :one
-- revoked_now: this transaction set revoked_at (an earlier revoke has an earlier time).
UPDATE gateway_join_token SET revoked_at = coalesce(revoked_at, now())
WHERE id = @id AND org_id = @org_id
RETURNING gateway_id, (revoked_at = now())::bool AS revoked_now;

-- name: SpendJoinToken :one
UPDATE gateway_join_token SET uses = uses + 1
WHERE token_hash = @token_hash AND uses < max_uses AND expires_at > now() AND revoked_at IS NULL
RETURNING id, org_id, gateway_id;

-- name: DeleteExpiredJoinTokens :execrows
DELETE FROM gateway_join_token WHERE expires_at < now() - interval '7 days';

-- name: UpsertResourceHealth :exec
-- Rows of resources outside the instance's pool or organization are skipped;
-- the newest of repeated resource ids wins.
INSERT INTO resource_health (resource_id, gateway_instance_id, org_id, ok, error, checked_at)
SELECT DISTINCT ON (r.id) r.id, i.id, r.org_id, h.ok, h.error, h.checked_at
FROM (SELECT unnest(@resource_ids::uuid[]) AS resource_id, unnest(@oks::bool[]) AS ok,
             unnest(@errors::text[]) AS error, unnest(@checked_ats::timestamptz[]) AS checked_at) h
JOIN resource r ON r.id = h.resource_id AND r.org_id = @org_id
JOIN gateway_instance i ON i.id = @instance_id AND i.org_id = @org_id AND i.gateway_id = r.gateway_id
ORDER BY r.id, h.checked_at DESC
ON CONFLICT (resource_id, gateway_instance_id)
DO UPDATE SET ok = EXCLUDED.ok, error = EXCLUDED.error, checked_at = EXCLUDED.checked_at;

-- name: OnlineInstanceIDs :many
SELECT id FROM gateway_instance
WHERE gateway_id = @gateway_id AND last_seen_at > now() - interval '45 seconds'
ORDER BY id;
