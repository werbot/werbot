-- name: LockSetup :exec
SELECT pg_advisory_xact_lock(@key::bigint);

-- name: HasUsers :one
SELECT EXISTS (SELECT 1 FROM users);

-- name: CreateOrganization :one
INSERT INTO organization (name) VALUES (@name) RETURNING id;

-- name: CreateUser :one
INSERT INTO users (email, login, name, password_hash)
VALUES (@email, @login, @name, @password_hash)
RETURNING id;

-- name: CreateMembership :exec
INSERT INTO membership (org_id, user_id, role) VALUES (@org_id, @user_id, @role);

-- name: GetLoginUser :one
-- The user's first membership (by created_at) is the organization of the session.
SELECT u.id, u.password_hash, m.org_id
FROM users u LEFT JOIN membership m ON m.user_id = u.id
WHERE lower(u.email) = @email
ORDER BY m.created_at
LIMIT 1;

-- name: CreateSession :exec
INSERT INTO user_session (token_hash, user_id, org_id, expires_at, ip, user_agent)
VALUES (@token_hash, @user_id, @org_id, now() + interval '7 days', @ip, @user_agent);

-- name: GetSessionByToken :one
SELECT s.id, s.user_id, s.org_id, m.role, s.last_seen_at < now() - interval '1 minute' AS stale
FROM user_session s
JOIN membership m ON m.org_id = s.org_id AND m.user_id = s.user_id
WHERE s.token_hash = @token_hash AND s.expires_at > now();

-- name: ExtendSession :exec
UPDATE user_session SET last_seen_at = now(), expires_at = now() + interval '7 days'
WHERE id = @id AND org_id = @org_id;

-- name: DeleteSession :exec
DELETE FROM user_session WHERE id = @id AND org_id = @org_id;

-- name: GetSessionInfo :one
SELECT u.id, u.email, u.login, u.name, u.created_at, o.name AS org_name, m.role
FROM membership m
JOIN users u ON u.id = m.user_id
JOIN organization o ON o.id = m.org_id
WHERE m.org_id = @org_id AND m.user_id = @user_id;

-- name: CreateOneTimeToken :exec
INSERT INTO one_time_token (token_hash, kind, org_id, user_id, email, role, expires_at)
VALUES (@token_hash, @kind, @org_id, @user_id, @email, @role, now() + make_interval(secs => @ttl_seconds::float8));

-- name: ConsumeOneTimeToken :one
UPDATE one_time_token SET used_at = now()
WHERE token_hash = @token_hash AND kind = @kind AND used_at IS NULL AND expires_at > now()
RETURNING org_id, user_id, email, role;

-- name: GetResetUser :one
-- The audit event of a reset goes to the user's first organization, if any.
SELECT u.id, u.name, m.org_id
FROM users u LEFT JOIN membership m ON m.user_id = u.id
WHERE lower(u.email) = @email
ORDER BY m.created_at
LIMIT 1;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = @password_hash, updated_at = now() WHERE id = @id;

-- name: GetUserOrg :one
SELECT org_id FROM membership WHERE user_id = @user_id ORDER BY created_at LIMIT 1;

-- name: DeleteUserSessions :exec
DELETE FROM user_session WHERE user_id = @user_id;

-- name: GetProfile :one
SELECT id, email, login, name, created_at FROM users WHERE id = @id;

-- name: GetCertSubject :one
-- The login a user certificate names and whether an active lock holds its CLI session.
SELECT u.login, EXISTS (SELECT 1 FROM lock l WHERE l.org_id = @org_id AND l.target_type = 'cli_session'
    AND l.target_id = @cli_session_id::uuid AND (l.expires_at IS NULL OR l.expires_at > now()))::bool AS locked
FROM users u WHERE u.id = @user_id;

-- name: UpdateProfile :exec
-- A NULL email keeps the current one.
UPDATE users SET name = @name, email = coalesce(sqlc.narg(email), email), updated_at = now() WHERE id = @id;

-- name: GetPasswordHash :one
SELECT password_hash FROM users WHERE id = @id;

-- name: DeleteOtherUserSessions :exec
DELETE FROM user_session WHERE user_id = @user_id AND id <> @keep_id;

-- name: VoidPasswordResetTokens :exec
UPDATE one_time_token SET used_at = now()
WHERE user_id = @user_id::uuid AND kind = 'password_reset' AND used_at IS NULL;

-- name: LockOrgOwners :many
-- Fixed lock order (user_id) keeps concurrent owner changes deadlock-free.
SELECT user_id FROM membership WHERE org_id = @org_id AND role = 'owner' ORDER BY user_id FOR UPDATE;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = @id;

-- name: ListSSHKeys :many
SELECT id, name, public_key, fingerprint_sha256, created_at FROM user_ssh_key WHERE user_id = @user_id ORDER BY created_at, id;

-- name: CreateSSHKey :one
INSERT INTO user_ssh_key (user_id, name, public_key, fingerprint_sha256)
VALUES (@user_id, @name, @public_key, @fingerprint_sha256)
RETURNING id, name, public_key, fingerprint_sha256, created_at;

-- name: UpdateSSHKey :one
UPDATE user_ssh_key SET name = @name WHERE id = @id AND user_id = @user_id
RETURNING id, name, public_key, fingerprint_sha256, created_at;

-- name: DeleteSSHKey :one
DELETE FROM user_ssh_key WHERE id = @id AND user_id = @user_id RETURNING fingerprint_sha256;

-- name: LookupSSHKeyUser :one
-- team_ids is NULL when the member is in no team, as UserTeams returns no rows.
-- ponytail: the team sub-select runs with werbot.user_id unset (uuid.Nil: the
-- tunnel's ReadInOrg has no user); stage 42 RLS on team_member must allow it.
SELECT u.id, u.login, m.role,
       (SELECT array_agg(tm.team_id ORDER BY tm.team_id) FROM team_member tm
        WHERE tm.org_id = m.org_id AND tm.user_id = u.id)::uuid[] AS team_ids
FROM user_ssh_key k
JOIN users u ON u.id = k.user_id
JOIN membership m ON m.user_id = u.id AND m.org_id = @org_id
WHERE k.fingerprint_sha256 = @fingerprint_sha256;

-- name: LookupLoginUser :one
-- ponytail: the team sub-select runs with werbot.user_id unset (uuid.Nil: the
-- tunnel's ReadInOrg has no user); stage 42 RLS on team_member must allow it.
SELECT u.id, u.login, m.role,
       (SELECT array_agg(tm.team_id ORDER BY tm.team_id) FROM team_member tm
        WHERE tm.org_id = m.org_id AND tm.user_id = u.id)::uuid[] AS team_ids
FROM users u
JOIN membership m ON m.user_id = u.id AND m.org_id = @org_id
WHERE u.login = @login;

-- name: ListMembers :many
SELECT u.id, u.email, u.login, u.name, u.created_at, m.role, m.created_at AS joined_at
FROM membership m JOIN users u ON u.id = m.user_id
WHERE m.org_id = @org_id
ORDER BY m.created_at, u.id;

-- name: ListInvites :many
SELECT email::text, role::text, expires_at FROM one_time_token
WHERE org_id = @org_id::uuid AND kind = 'invite' AND used_at IS NULL AND expires_at > now()
ORDER BY created_at;

-- name: IsMemberEmail :one
SELECT EXISTS (
    SELECT 1 FROM membership m JOIN users u ON u.id = m.user_id
    WHERE m.org_id = @org_id AND lower(u.email) = @email
);

-- name: LockMemberRole :one
-- Callers lock the org's owners first (LockOrgOwners): one lock order for all role changes.
SELECT role FROM membership WHERE org_id = @org_id AND user_id = @user_id FOR UPDATE;

-- name: UpdateMemberRole :exec
UPDATE membership SET role = @role WHERE org_id = @org_id AND user_id = @user_id;

-- name: DeleteMembership :exec
DELETE FROM membership WHERE org_id = @org_id AND user_id = @user_id;

-- name: LockUserOwnedOrgs :many
-- Owners of every organization the user owns, locked in (org_id, user_id) order.
-- ponytail: reads other orgs inside the caller's InOrg tx; stage 42 RLS must allow it.
SELECT m.org_id, m.user_id FROM membership m
WHERE m.role = 'owner' AND m.org_id IN (SELECT o.org_id FROM membership o WHERE o.user_id = @user_id AND o.role = 'owner')
ORDER BY m.org_id, m.user_id FOR UPDATE OF m;

-- name: ListTeams :many
SELECT t.id, t.name, t.created_at,
    COALESCE(array_agg(tm.user_id ORDER BY tm.user_id) FILTER (WHERE tm.user_id IS NOT NULL), '{}')::uuid[] AS member_user_ids
FROM team t LEFT JOIN team_member tm ON tm.team_id = t.id
WHERE t.org_id = @org_id
GROUP BY t.id
ORDER BY t.created_at, t.id;

-- name: TeamMemberIDs :many
SELECT user_id FROM team_member WHERE team_id = @team_id AND org_id = @org_id ORDER BY user_id;

-- name: CreateTeam :one
INSERT INTO team (org_id, name) VALUES (@org_id, @name) RETURNING id, created_at;

-- name: RenameTeam :one
UPDATE team SET name = @name WHERE id = @id AND org_id = @org_id RETURNING id, name, created_at;

-- name: DeleteTeam :one
DELETE FROM team WHERE id = @id AND org_id = @org_id RETURNING name;

-- name: AddTeamMember :exec
INSERT INTO team_member (team_id, org_id, user_id) VALUES (@team_id, @org_id, @user_id);

-- name: RemoveTeamMember :execrows
DELETE FROM team_member WHERE team_id = @team_id AND org_id = @org_id AND user_id = @user_id;

-- name: UserTeams :many
SELECT team_id FROM team_member WHERE org_id = @org_id AND user_id = @user_id ORDER BY team_id;

-- name: ListOrgMembers :many
-- uuid order equals the order of the lowercase text form.
SELECT m.user_id,
       coalesce(array_agg(tm.team_id ORDER BY tm.team_id) FILTER (WHERE tm.team_id IS NOT NULL), '{}')::uuid[] AS team_ids
FROM membership m
LEFT JOIN team_member tm ON tm.org_id = m.org_id AND tm.user_id = m.user_id
WHERE m.org_id = @org_id
GROUP BY m.user_id
ORDER BY m.user_id;

-- name: CreateOrgSetting :exec
INSERT INTO org_setting (org_id) VALUES (@org_id);

-- name: GetOrgSettings :one
SELECT extract(epoch FROM cli_session_ttl)::bigint AS cli_session_ttl_secs,
       extract(epoch FROM cli_session_max)::bigint AS cli_session_max_secs,
       extract(epoch FROM grant_default_ttl)::bigint AS grant_default_ttl_secs,
       extract(epoch FROM grant_max_ttl)::bigint AS grant_max_ttl_secs,
       extract(epoch FROM request_pending_ttl)::bigint AS request_pending_ttl_secs,
       extract(epoch FROM command_approval_ttl)::bigint AS command_approval_ttl_secs,
       lock_mode
FROM org_setting WHERE org_id = @org_id;

-- name: LockOrgSettings :one
SELECT extract(epoch FROM cli_session_ttl)::bigint AS cli_session_ttl_secs,
       extract(epoch FROM cli_session_max)::bigint AS cli_session_max_secs,
       extract(epoch FROM grant_default_ttl)::bigint AS grant_default_ttl_secs,
       extract(epoch FROM grant_max_ttl)::bigint AS grant_max_ttl_secs,
       extract(epoch FROM request_pending_ttl)::bigint AS request_pending_ttl_secs,
       extract(epoch FROM command_approval_ttl)::bigint AS command_approval_ttl_secs,
       lock_mode
FROM org_setting WHERE org_id = @org_id FOR UPDATE;

-- name: UpdateOrgSettings :exec
UPDATE org_setting SET
    cli_session_ttl = make_interval(secs => @cli_session_ttl_secs::float8),
    cli_session_max = make_interval(secs => @cli_session_max_secs::float8),
    grant_default_ttl = make_interval(secs => @grant_default_ttl_secs::float8),
    grant_max_ttl = make_interval(secs => @grant_max_ttl_secs::float8),
    request_pending_ttl = make_interval(secs => @request_pending_ttl_secs::float8),
    command_approval_ttl = make_interval(secs => @command_approval_ttl_secs::float8),
    lock_mode = @lock_mode,
    updated_at = now()
WHERE org_id = @org_id;

-- name: GetMemberRole :one
SELECT role FROM membership WHERE org_id = @org_id AND user_id = @user_id;

-- name: GetCLISessionRole :one
-- A Bearer request: the live cli_session of the token and the member's role.
SELECT m.role FROM cli_session c
JOIN membership m ON m.org_id = c.org_id AND m.user_id = c.user_id
WHERE c.id = @id AND c.org_id = @org_id AND c.user_id = @user_id AND c.revoked_at IS NULL
  AND c.expires_at > now() AND c.absolute_expires_at > now();

-- name: ListCLISessions :many
SELECT id, user_id, client_id, hostname, ip, created_at, last_used_at, expires_at, absolute_expires_at
FROM cli_session
WHERE org_id = @org_id AND user_id = @user_id AND revoked_at IS NULL
  AND expires_at > now() AND absolute_expires_at > now()
ORDER BY created_at DESC;

-- name: RevokeCLISession :execrows
-- A manager revokes any session of the organization, others only their own.
UPDATE cli_session SET revoked_at = now(), revoke_reason = @reason::text
WHERE id = @id AND org_id = @org_id AND revoked_at IS NULL AND (@manager::bool OR user_id = @user_id);

-- name: RevokeUserCLISessions :exec
-- A new password ends the user's CLI sessions in every organization but keep_id.
UPDATE cli_session SET revoked_at = now(), revoke_reason = 'password_reset'
WHERE user_id = @user_id AND id <> @keep_id AND revoked_at IS NULL;

-- name: GetUserCA :one
SELECT ciphertext, key_id FROM org_ssh_ca WHERE org_id = @org_id;

-- name: InsertUserCA :exec
-- Concurrent first uses race here: the first insert wins, the rest re-read.
INSERT INTO org_ssh_ca (org_id, ciphertext, key_id, public_key)
VALUES (@org_id, @ciphertext, @key_id, @public_key)
ON CONFLICT (org_id) DO NOTHING;

-- name: LookupCertUser :one
-- An SSH certificate: the live cli_session it was issued for, the member and
-- the login of its principal. ponytail: team sub-select as LookupLoginUser.
SELECT u.id, u.login, m.role,
       (SELECT array_agg(tm.team_id ORDER BY tm.team_id) FROM team_member tm
        WHERE tm.org_id = m.org_id AND tm.user_id = u.id)::uuid[] AS team_ids
FROM cli_session c
JOIN membership m ON m.org_id = c.org_id AND m.user_id = c.user_id
JOIN users u ON u.id = c.user_id
WHERE c.id = @id AND c.org_id = @org_id AND c.user_id = @user_id AND u.login = @login
  AND c.revoked_at IS NULL AND c.expires_at > now() AND c.absolute_expires_at > now();
