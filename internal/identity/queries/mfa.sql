-- name: DeleteOldMFAChallenges :execrows
DELETE FROM mfa_challenge WHERE created_at < now() - make_interval(secs => @older_secs::float8);

-- name: DeleteOldMFAVerifications :execrows
DELETE FROM mfa_verification WHERE verified_at < now() - make_interval(secs => @older_secs::float8);
