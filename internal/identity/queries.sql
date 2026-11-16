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
