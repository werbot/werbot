-- name: GetSigningKey :one
SELECT ciphertext, key_id FROM oauth_signing_key WHERE id = 1;

-- name: InsertSigningKey :exec
INSERT INTO oauth_signing_key (id, ciphertext, key_id, public_jwk)
VALUES (1, @ciphertext, @key_id, @public_jwk)
ON CONFLICT (id) DO NOTHING;

-- name: CreateAuthRequest :one
-- Also drops requests that ended over an hour ago (no cleanup job).
WITH gc AS (DELETE FROM oauth_auth_request WHERE expires_at < now() - interval '1 hour')
INSERT INTO oauth_auth_request (client_id, redirect_uri, code_challenge, scopes, state, client_hostname, client_ip, expires_at)
VALUES (@client_id, @redirect_uri, @code_challenge, @scopes, @state, @client_hostname, @client_ip, now() + interval '10 minutes')
RETURNING id;

-- name: GetAuthRequest :one
-- No expiry filter: op needs redirect_uri to report an ended request.
SELECT id, redirect_uri, code_challenge, scopes, state, user_id, org_id,
       (user_id IS NOT NULL AND expires_at > now())::bool AS done
FROM oauth_auth_request WHERE id = @id;

-- name: GetAuthRequestByCode :one
SELECT id, redirect_uri, code_challenge, scopes, state, user_id, org_id,
       (user_id IS NOT NULL AND expires_at > now())::bool AS done
FROM oauth_auth_request WHERE code_hash = @code_hash AND expires_at > now();

-- name: SaveAuthCode :execrows
UPDATE oauth_auth_request SET code_hash = @code_hash
WHERE id = @id AND user_id IS NOT NULL AND expires_at > now();

-- name: DeleteAuthRequest :exec
DELETE FROM oauth_auth_request WHERE id = @id;

-- name: GetPendingAuthRequest :one
SELECT client_id, client_hostname, client_ip, scopes FROM oauth_auth_request
WHERE id = @id AND user_id IS NULL AND expires_at > now();

-- name: ApproveAuthRequest :execrows
UPDATE oauth_auth_request SET user_id = @user_id, org_id = @org_id
WHERE id = @id AND user_id IS NULL AND expires_at > now();

-- name: DenyAuthRequest :execrows
-- The callback then sees an unfinished request: interaction_required.
UPDATE oauth_auth_request SET expires_at = now()
WHERE id = @id AND user_id IS NULL AND expires_at > now();

-- name: ConsumeAuthRequest :one
-- Of concurrent exchanges of one code exactly one deletes the row.
DELETE FROM oauth_auth_request
WHERE id = @id AND code_hash = @code_hash AND user_id IS NOT NULL AND expires_at > now()
RETURNING user_id, org_id, client_id, client_hostname, client_ip;

-- name: CreateCLISession :one
-- Lifetimes come from the organization's settings; no row, no session.
INSERT INTO cli_session (org_id, user_id, client_id, hostname, ip, refresh_hash, expires_at, absolute_expires_at)
SELECT s.org_id, @user_id, @client_id, @hostname, @ip, @refresh_hash, now() + s.cli_session_ttl, now() + s.cli_session_max
FROM org_setting s WHERE s.org_id = @org_id
RETURNING id;

-- name: CreateDeviceCode :exec
-- Also drops codes that ended over an hour ago (no cleanup job).
WITH gc AS (DELETE FROM oauth_device_code WHERE expires_at < now() - interval '1 hour')
INSERT INTO oauth_device_code (device_code_hash, user_code, client_id, scopes, client_hostname, client_ip, expires_at)
VALUES (@device_code_hash, @user_code, @client_id, @scopes, @client_hostname, @client_ip, @expires_at);

-- name: PollDeviceCode :one
-- prev_poll_at is the poll before this one (slow_down under 5 s).
WITH prev AS (
    SELECT c.id, c.last_poll_at FROM oauth_device_code c
    WHERE c.device_code_hash = @device_code_hash AND c.client_id = @client_id
)
UPDATE oauth_device_code d SET last_poll_at = now()
FROM prev WHERE d.id = prev.id
RETURNING d.id, d.status, d.scopes, d.user_id, d.org_id, d.expires_at, (d.expires_at > now())::bool AS live,
    d.last_poll_at, prev.last_poll_at AS prev_poll_at;

-- name: ConsumeDeviceCode :one
-- Of concurrent polls of one approved code exactly one marks it used.
-- Same columns as ConsumeAuthRequest.
UPDATE oauth_device_code SET status = 'used'
WHERE id = @id AND status = 'approved' AND expires_at > now()
RETURNING user_id, org_id, client_id, client_hostname, client_ip;

-- name: GetPendingDeviceCode :one
SELECT client_id, client_hostname, client_ip, scopes FROM oauth_device_code
WHERE user_code = @user_code AND status = 'pending' AND expires_at > now();

-- name: ApproveDeviceCode :execrows
UPDATE oauth_device_code SET status = 'approved', user_id = @user_id, org_id = @org_id
WHERE user_code = @user_code AND status = 'pending' AND expires_at > now();

-- name: DenyDeviceCode :execrows
UPDATE oauth_device_code SET status = 'denied'
WHERE user_code = @user_code AND status = 'pending' AND expires_at > now();

-- name: GetLiveCLISession :one
-- The session of a refresh token, if live under the current settings.
-- agent_id: an agent profile.
SELECT c.id, c.org_id, c.user_id, c.client_id, c.agent_id FROM cli_session c
JOIN org_setting s ON s.org_id = c.org_id
WHERE c.refresh_hash = @refresh_hash AND c.revoked_at IS NULL AND c.expires_at > now()
  AND c.absolute_expires_at > now() AND c.created_at + s.cli_session_max > now();

-- name: RevokeReusedCLISession :one
-- The refresh token before the last rotation came back: end the session.
UPDATE cli_session SET revoked_at = now(), revoke_reason = 'refresh_reuse'
WHERE prev_refresh_hash = @prev_refresh_hash AND revoked_at IS NULL
RETURNING id, org_id;

-- name: RotateCLISession :execrows
-- Of concurrent rotations of one token exactly one matches refresh_hash.
UPDATE cli_session c SET prev_refresh_hash = c.refresh_hash, refresh_hash = @new_hash, last_used_at = now(),
    expires_at = least(now() + s.cli_session_ttl, c.absolute_expires_at, c.created_at + s.cli_session_max)
FROM org_setting s
WHERE c.id = @id AND s.org_id = c.org_id AND c.refresh_hash = @refresh_hash AND c.revoked_at IS NULL;

-- name: GetCLISessionByRefresh :one
SELECT id, user_id FROM cli_session WHERE refresh_hash = @refresh_hash AND client_id = @client_id;

-- name: LogoutCLISession :one
UPDATE cli_session SET revoked_at = now(), revoke_reason = 'logout'
WHERE id = @id AND user_id = @user_id AND revoked_at IS NULL
RETURNING org_id;

-- name: LiveCLISessionMember :one
-- A token exchange subject: its cli_session is live and its user a member;
-- an agent profile also names its agent.
SELECT c.id, c.agent_id, a.name AS agent_name FROM cli_session c
JOIN membership m ON m.org_id = c.org_id AND m.user_id = c.user_id
LEFT JOIN agent a ON a.id = c.agent_id AND a.org_id = c.org_id
WHERE c.id = @id AND c.org_id = @org_id AND c.user_id = @user_id AND c.revoked_at IS NULL
  AND c.expires_at > now() AND c.absolute_expires_at > now();

-- name: LockProfileAgent :one
-- An agent profile is created under the agent's row lock: its sponsor and
-- status and the organization's cli_session_max.
SELECT a.name, a.sponsor_user_id, a.status, extract(epoch FROM s.cli_session_max)::bigint AS max_secs
FROM agent a JOIN org_setting s ON s.org_id = a.org_id
WHERE a.id = @id AND a.org_id = @org_id
FOR UPDATE OF a;

-- name: CountAgentProfiles :one
-- Live profiles of an agent; after LockProfileAgent, so this statement's
-- snapshot sees every profile committed before the lock.
-- ponytail: the live cli_session predicate is repeated in GetLiveCLISession,
-- here and ListAgentProfiles; a live_cli_session view would hold it once.
SELECT count(*) FROM cli_session c
JOIN org_setting s ON s.org_id = c.org_id
WHERE c.org_id = @org_id AND c.agent_id = @agent_id::uuid AND c.revoked_at IS NULL AND c.expires_at > now()
  AND c.absolute_expires_at > now() AND c.created_at + s.cli_session_max > now();

-- name: CreateAgentProfile :one
-- Idle past cli_session_ttl or older than ttl, the profile ends.
INSERT INTO cli_session (org_id, user_id, client_id, hostname, ip, refresh_hash, agent_id, expires_at, absolute_expires_at)
SELECT s.org_id, @user_id, 'werbot-cli', @hostname, @ip, @refresh_hash, @agent_id::uuid,
       least(now() + s.cli_session_ttl, now() + make_interval(secs => @ttl_secs::float8)),
       now() + make_interval(secs => @ttl_secs::float8)
FROM org_setting s WHERE s.org_id = @org_id
RETURNING id, created_at, last_used_at, expires_at, absolute_expires_at;

-- name: ListAgentProfiles :many
-- Live agent profiles, of one sponsor and one agent when given.
SELECT c.id, a.id AS agent_id, a.name AS agent_name, c.user_id, c.hostname, c.created_at, c.last_used_at, c.expires_at, c.absolute_expires_at
FROM cli_session c
JOIN agent a ON a.id = c.agent_id AND a.org_id = c.org_id
JOIN org_setting s ON s.org_id = c.org_id
WHERE c.org_id = @org_id AND c.revoked_at IS NULL AND c.expires_at > now()
  AND c.absolute_expires_at > now() AND c.created_at + s.cli_session_max > now()
  AND (sqlc.narg(sponsor_user_id)::uuid IS NULL OR c.user_id = sqlc.narg(sponsor_user_id))
  AND (sqlc.narg(agent_id)::uuid IS NULL OR c.agent_id = sqlc.narg(agent_id))
ORDER BY c.created_at DESC;

-- name: RevokeAgentProfile :one
-- A manager revokes any profile of the organization, others only their own.
UPDATE cli_session SET revoked_at = now(), revoke_reason = 'revoked'
WHERE id = @id AND org_id = @org_id AND agent_id IS NOT NULL AND revoked_at IS NULL
  AND (@manager::bool OR user_id = @user_id)
RETURNING agent_id;

-- name: RevokeProfilesOfAgent :many
UPDATE cli_session SET revoked_at = now(), revoke_reason = @reason::text
WHERE org_id = @org_id AND agent_id = @agent_id::uuid AND revoked_at IS NULL
RETURNING id;
