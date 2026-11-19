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
UPDATE oauth_device_code SET last_poll_at = now()
WHERE device_code_hash = @device_code_hash AND client_id = @client_id
RETURNING id, status, scopes, user_id, org_id, expires_at, (expires_at > now())::bool AS live;

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
SELECT c.id, c.org_id, c.user_id, c.client_id FROM cli_session c
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
