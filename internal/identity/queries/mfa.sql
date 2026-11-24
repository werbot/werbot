-- name: DeleteOldMFAChallenges :execrows
DELETE FROM mfa_challenge WHERE created_at < now() - make_interval(secs => @older_secs::float8);

-- name: DeleteOldMFAVerifications :execrows
DELETE FROM mfa_verification WHERE verified_at < now() - make_interval(secs => @older_secs::float8);

-- name: UpsertTOTPSecret :execrows
-- A new enrollment replaces an unconfirmed one; a confirmed secret stays (0 rows).
INSERT INTO totp_secret (user_id, ciphertext, key_id) VALUES (@user_id, @ciphertext, @key_id)
ON CONFLICT (user_id) DO UPDATE SET ciphertext = excluded.ciphertext, key_id = excluded.key_id, last_step = 0
WHERE totp_secret.confirmed_at IS NULL;

-- name: ConfirmTOTPSecret :execrows
UPDATE totp_secret SET confirmed_at = now() WHERE user_id = @user_id AND confirmed_at IS NULL;

-- name: GetTOTPSecret :one
SELECT ciphertext, key_id FROM totp_secret WHERE user_id = @user_id AND confirmed_at IS NOT NULL;

-- name: UseTOTPStep :execrows
-- Spends step: a code of last_step or an older step is never accepted again.
UPDATE totp_secret SET last_step = @step WHERE user_id = @user_id AND confirmed_at IS NOT NULL AND last_step < @step;

-- name: DeleteTOTPSecret :execrows
DELETE FROM totp_secret WHERE user_id = @user_id;

-- name: DeleteRecoveryCodes :execrows
DELETE FROM mfa_recovery_code WHERE user_id = @user_id;

-- name: InsertRecoveryCodes :exec
INSERT INTO mfa_recovery_code (user_id, code_hash) SELECT @user_id::uuid, h FROM unnest(@code_hashes::bytea[]) AS h;

-- name: UseRecoveryCode :execrows
UPDATE mfa_recovery_code SET used_at = now() WHERE user_id = @user_id AND code_hash = @code_hash AND used_at IS NULL;

-- name: DeleteWebAuthnCredential :execrows
DELETE FROM webauthn_credential WHERE id = @id AND user_id = @user_id;

-- name: HasFactor :one
-- A confirmed TOTP secret or a passkey; recovery codes alone are no factor.
SELECT (EXISTS (SELECT 1 FROM totp_secret t WHERE t.user_id = @user_id::uuid AND t.confirmed_at IS NOT NULL)
    OR EXISTS (SELECT 1 FROM webauthn_credential w WHERE w.user_id = @user_id::uuid))::bool AS has;

-- name: GetTOTPFactor :one
SELECT confirmed_at::timestamptz AS confirmed_at FROM totp_secret WHERE user_id = @user_id AND confirmed_at IS NOT NULL;

-- name: GetRecoveryFactor :one
SELECT count(*) AS total, count(*) FILTER (WHERE used_at IS NULL) AS remaining
FROM mfa_recovery_code WHERE user_id = @user_id;

-- name: ListWebAuthnFactors :many
SELECT id, name, created_at, last_used_at FROM webauthn_credential WHERE user_id = @user_id ORDER BY created_at, id;

-- name: GetRequireMFA :one
SELECT require_mfa FROM org_setting WHERE org_id = @org_id;

-- name: ListWebAuthnCredentials :many
SELECT id, credential_id, public_key, sign_count, aaguid, transports, backup_eligible FROM webauthn_credential
WHERE user_id = @user_id ORDER BY created_at, id;

-- name: CreateWebAuthnCredential :one
INSERT INTO webauthn_credential (user_id, name, credential_id, public_key, sign_count, aaguid, transports, backup_eligible)
VALUES (@user_id, @name, @credential_id, @public_key, @sign_count, @aaguid, @transports, @backup_eligible)
RETURNING id, created_at;

-- name: UseWebAuthnCredential :execrows
-- Stores the sign count of an assertion: it must grow, unless the
-- authenticator keeps no counter (0 before and now).
UPDATE webauthn_credential SET sign_count = @sign_count, last_used_at = now()
WHERE id = @id AND (sign_count < @sign_count OR sign_count = 0 AND @sign_count = 0);

-- name: MFADeviceLocked :one
SELECT EXISTS (SELECT 1 FROM lock WHERE org_id = @org_id AND target_type = 'mfa_device' AND target_id = @id::uuid
    AND (expires_at IS NULL OR expires_at > now()))::bool AS locked;

-- name: DeleteStaleCeremonies :exec
DELETE FROM webauthn_ceremony WHERE user_id = @user_id AND (expires_at <= now() OR used_at IS NOT NULL);

-- name: CreateCeremony :exec
INSERT INTO webauthn_ceremony (org_id, user_id, kind, ref, session_data, expires_at)
VALUES (@org_id, @user_id, @kind, @ref, @session_data, now() + make_interval(secs => @ttl_secs::float8));

-- name: UseCeremony :one
-- Spends the newest open ceremony of the user, kind and ref.
UPDATE webauthn_ceremony SET used_at = now()
WHERE id = (SELECT c.id FROM webauthn_ceremony c
        WHERE c.org_id = @org_id AND c.user_id = @user_id AND c.kind = @kind AND c.ref = @ref
            AND c.used_at IS NULL AND c.expires_at > now()
        ORDER BY c.created_at DESC, c.id DESC LIMIT 1 FOR UPDATE)
    AND used_at IS NULL
RETURNING session_data;

-- name: GetSessionByToken :one
-- A cookie request: the live session, the member's role and the user's MFA
-- state (the session's last second factor, the organization's requirement,
-- whether the user has a factor; recovery codes alone are no factor).
SELECT s.id, s.user_id, s.org_id, m.role, s.last_seen_at < now() - interval '1 minute' AS stale, s.mfa_at, o.require_mfa,
    (EXISTS (SELECT 1 FROM totp_secret t WHERE t.user_id = s.user_id AND t.confirmed_at IS NOT NULL)
        OR EXISTS (SELECT 1 FROM webauthn_credential w WHERE w.user_id = s.user_id))::bool AS has_factor
FROM user_session s
JOIN membership m ON m.org_id = s.org_id AND m.user_id = s.user_id
JOIN org_setting o ON o.org_id = s.org_id
WHERE s.token_hash = @token_hash AND s.expires_at > now();

-- name: GetCLISessionRole :one
-- A Bearer request: the live cli_session of the token, its agent, the
-- member's role and the user's MFA state as for a cookie, with the newest
-- second factor verified for this cli_session.
SELECT m.role, c.agent_id, o.require_mfa,
    (EXISTS (SELECT 1 FROM totp_secret t WHERE t.user_id = c.user_id AND t.confirmed_at IS NOT NULL)
        OR EXISTS (SELECT 1 FROM webauthn_credential w WHERE w.user_id = c.user_id))::bool AS has_factor,
    v.verified_at AS mfa_at
FROM cli_session c
JOIN membership m ON m.org_id = c.org_id AND m.user_id = c.user_id
JOIN org_setting o ON o.org_id = c.org_id
LEFT JOIN mfa_verification v ON v.user_id = c.user_id AND v.org_id = c.org_id AND v.binding = 'cli:' || c.id::text
WHERE c.id = @id AND c.org_id = @org_id AND c.user_id = @user_id AND c.revoked_at IS NULL
  AND c.expires_at > now() AND c.absolute_expires_at > now()
ORDER BY v.verified_at DESC NULLS LAST LIMIT 1;

-- name: GetLoginFactors :one
-- What a login can be finished with; recovery codes count while one is unused.
SELECT EXISTS (SELECT 1 FROM totp_secret WHERE user_id = @user_id::uuid AND confirmed_at IS NOT NULL)::bool AS has_totp,
    EXISTS (SELECT 1 FROM webauthn_credential WHERE user_id = @user_id::uuid)::bool AS has_passkey,
    EXISTS (SELECT 1 FROM mfa_recovery_code WHERE user_id = @user_id::uuid AND used_at IS NULL)::bool AS has_recovery;

-- name: GetMFALoginToken :one
-- Reads an open mfa_token without spending it (consumeToken spends it).
SELECT org_id::uuid AS org_id, user_id::uuid AS user_id FROM one_time_token
WHERE token_hash = @token_hash AND kind = 'mfa_login' AND used_at IS NULL AND expires_at > now()
    AND org_id IS NOT NULL AND user_id IS NOT NULL;

-- name: SetSessionMFA :one
-- Records that the session of token_hash has just passed a second factor.
UPDATE user_session SET mfa_at = now() WHERE token_hash = @token_hash RETURNING id;

-- name: SetSessionMFAByID :exec
-- Records that the caller's session has just passed a step-up.
UPDATE user_session SET mfa_at = now() WHERE id = @id AND org_id = @org_id AND user_id = @user_id;

-- name: CreateMFAVerification :exec
INSERT INTO mfa_verification (org_id, user_id, binding, method) VALUES (@org_id, @user_id, @binding, @method);
