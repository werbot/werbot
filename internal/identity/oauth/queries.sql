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
