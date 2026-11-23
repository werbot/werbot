-- name: CreateMFAChallenge :one
-- Creates a pending challenge of a member of org_id; none for a non-member.
INSERT INTO mfa_challenge (org_id, user_id, code, ssh_session_id, resource_id, client_ip, binding, strength,
    gateway_instance_id, expires_at)
SELECT m.org_id, m.user_id, @code, sqlc.narg(ssh_session_id), sqlc.narg(resource_id), sqlc.narg(client_ip), @binding, @strength,
    sqlc.narg(gateway_instance_id), now() + make_interval(secs => @ttl_secs::float8)
FROM membership m WHERE m.org_id = @org_id AND m.user_id = @user_id
RETURNING id, expires_at;

-- name: GetUserMFAChallenge :one
-- A challenge of the user with its instance's hostname and resource name;
-- live is false once it expired.
SELECT c.code, c.status, c.strength, c.client_ip, c.expires_at, c.expires_at > now() AS live,
    coalesce(i.hostname, '')::text AS host, coalesce(r.name, '')::text AS resource
FROM mfa_challenge c
LEFT JOIN gateway_instance i ON i.id = c.gateway_instance_id AND i.org_id = c.org_id
LEFT JOIN resource r ON r.id = c.resource_id AND r.org_id = c.org_id
WHERE c.id = @id AND c.org_id = @org_id AND c.user_id = @user_id;

-- name: GetInstanceMFAChallenge :one
-- A challenge of the gateway instance; live is false once it expired.
SELECT user_id, strength, status, expires_at > now() AS live FROM mfa_challenge
WHERE id = @id AND org_id = @org_id AND gateway_instance_id = @instance_id::uuid;

-- name: ConfirmMFAChallenge :one
-- Spends a pending, unexpired challenge of the user (and of the instance,
-- when given): the only transition to confirmed.
UPDATE mfa_challenge SET status = 'confirmed', confirmed_at = now()
WHERE id = @id AND org_id = @org_id AND user_id = @user_id AND status = 'pending' AND expires_at > now()
    AND (sqlc.narg(instance_id)::uuid IS NULL OR gateway_instance_id = sqlc.narg(instance_id))
RETURNING binding, gateway_instance_id;

-- name: DenyMFAChallenge :exec
UPDATE mfa_challenge SET status = 'denied' WHERE id = @id AND org_id = @org_id AND status = 'pending';

-- name: MFAChallengeStatus :one
-- The status of an instance's challenge; a pending one past its expiry
-- becomes expired.
WITH e AS (
    UPDATE mfa_challenge SET status = 'expired'
    WHERE id = @id AND org_id = @org_id AND gateway_instance_id = @instance_id::uuid AND status = 'pending' AND expires_at <= now()
    RETURNING status
)
SELECT coalesce((SELECT e.status FROM e), c.status)::text AS status FROM mfa_challenge c
WHERE c.id = @id AND c.org_id = @org_id AND c.gateway_instance_id = @instance_id::uuid;

-- name: ListConfirmedMFAChallenges :many
SELECT id FROM mfa_challenge
WHERE org_id = @org_id AND gateway_instance_id = @instance_id::uuid AND status = 'confirmed' AND expires_at > now()
ORDER BY confirmed_at, id;

-- name: NotifyMFAConfirmed :exec
SELECT pg_notify('werbot_mfa_confirmed', @payload::text);

-- name: LastMFAVerification :one
-- The newest second factor of the user for binding and its age.
SELECT method, extract(epoch FROM now() - verified_at)::float8 AS age_secs FROM mfa_verification
WHERE org_id = @org_id AND user_id = @user_id AND binding = @binding
ORDER BY verified_at DESC LIMIT 1;
