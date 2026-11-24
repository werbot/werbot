-- name: GetIdPLinkUser :one
-- The member linked to subject at the IdP; the link goes with the membership.
SELECT user_id FROM user_idp_link WHERE idp_id = @idp_id AND subject = @subject AND org_id = @org_id;

-- name: GetMemberByEmail :one
SELECT u.id FROM users u JOIN membership m ON m.user_id = u.id
WHERE m.org_id = @org_id AND lower(u.email) = @email;

-- name: LoginTaken :one
SELECT EXISTS (SELECT 1 FROM users WHERE login = @login);

-- name: UpsertIdPLink :exec
-- A login without a refresh token keeps the stored one.
INSERT INTO user_idp_link (user_id, idp_id, org_id, subject, refresh_ciphertext, key_id, last_checked_at)
VALUES (@user_id, @idp_id, @org_id, @subject, sqlc.narg('refresh_ciphertext'), sqlc.narg('key_id'), now())
ON CONFLICT (idp_id, subject) DO UPDATE SET
    refresh_ciphertext = COALESCE(EXCLUDED.refresh_ciphertext, user_idp_link.refresh_ciphertext),
    key_id = CASE WHEN EXCLUDED.refresh_ciphertext IS NULL THEN user_idp_link.key_id ELSE EXCLUDED.key_id END,
    last_checked_at = now();

-- name: SetExternalRole :one
-- An IdP mapping never changes an owner; the old role, or no row when nothing changed.
UPDATE membership m SET role = @role
FROM membership old
WHERE m.org_id = @org_id AND m.user_id = @user_id AND m.role NOT IN ('owner', @role)
    AND old.org_id = m.org_id AND old.user_id = m.user_id
RETURNING old.role;

-- name: AddExternalTeams :exec
-- Teams deleted since the mapping was written are skipped.
INSERT INTO team_member (team_id, org_id, user_id)
SELECT t.id, t.org_id, @user_id FROM team t
WHERE t.org_id = @org_id AND t.id = ANY(@team_ids::uuid[])
ON CONFLICT DO NOTHING;

-- name: RemoveExternalTeams :exec
DELETE FROM team_member WHERE org_id = @org_id AND user_id = @user_id AND team_id = ANY(@team_ids::uuid[]);

-- name: ListEnabledIdPs :many
-- Every enabled IdP of the installation (Community has one organization).
SELECT id, name FROM idp_oidc WHERE enabled ORDER BY name, id;

-- name: DeleteOrgUserSessions :exec
DELETE FROM user_session WHERE org_id = @org_id AND user_id = @user_id;

-- name: RevokeOrgUserCLISessions :exec
UPDATE cli_session SET revoked_at = now(), revoke_reason = @reason::text
WHERE org_id = @org_id AND user_id = @user_id AND revoked_at IS NULL;
