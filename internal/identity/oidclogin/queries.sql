-- name: GetIdP :one
-- Public login paths: the IdP of any organization of the installation.
SELECT * FROM idp_oidc WHERE id = @id;

-- name: GetOrgIdP :one
SELECT * FROM idp_oidc WHERE id = @id AND org_id = @org_id;

-- name: ListIdPs :many
SELECT * FROM idp_oidc WHERE org_id = @org_id ORDER BY name, id;

-- name: CountOrgTeams :one
SELECT count(*) FROM team WHERE org_id = @org_id AND id = ANY(@ids::uuid[]);

-- name: CreateIdP :one
INSERT INTO idp_oidc (org_id, name, issuer, client_id, client_secret_ciphertext, key_id, scopes, claims_mapping, allow_signup, enabled, created_by)
VALUES (@org_id, @name, @issuer, @client_id, @client_secret_ciphertext, @key_id, @scopes, @claims_mapping, @allow_signup, @enabled, @created_by)
RETURNING *;

-- name: UpdateIdP :one
-- A NULL secret keeps the stored one.
UPDATE idp_oidc SET name = @name, issuer = @issuer, client_id = @client_id,
    client_secret_ciphertext = COALESCE(sqlc.narg('client_secret_ciphertext'), client_secret_ciphertext),
    key_id = COALESCE(sqlc.narg('key_id'), key_id),
    scopes = @scopes, claims_mapping = @claims_mapping, allow_signup = @allow_signup, enabled = @enabled
WHERE id = @id AND org_id = @org_id
RETURNING *;

-- name: DeleteIdP :one
DELETE FROM idp_oidc WHERE id = @id AND org_id = @org_id RETURNING name;

-- name: StaleLinks :many
-- Links with a refresh token not checked for older_secs, oldest first.
SELECT * FROM user_idp_link
WHERE refresh_ciphertext IS NOT NULL AND last_checked_at < now() - make_interval(secs => @older_secs::float8)
ORDER BY last_checked_at
LIMIT @lim;

-- name: UserStaleLinks :many
-- The links of a member with a refresh token not checked for older_secs.
SELECT * FROM user_idp_link
WHERE org_id = @org_id AND user_id = @user_id AND refresh_ciphertext IS NOT NULL
    AND last_checked_at < now() - make_interval(secs => @older_secs::float8);

-- name: UpdateLink :exec
-- keep_refresh leaves the stored refresh token as it is.
UPDATE user_idp_link SET last_checked_at = @checked_at,
    refresh_ciphertext = CASE WHEN @keep_refresh::bool THEN refresh_ciphertext ELSE sqlc.narg('refresh_ciphertext') END,
    key_id = CASE WHEN @keep_refresh::bool THEN key_id ELSE sqlc.narg('key_id') END
WHERE idp_id = @idp_id AND subject = @subject AND org_id = @org_id;
