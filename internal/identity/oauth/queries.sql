-- name: GetSigningKey :one
SELECT ciphertext, key_id FROM oauth_signing_key WHERE id = 1;

-- name: InsertSigningKey :exec
INSERT INTO oauth_signing_key (id, ciphertext, key_id, public_jwk)
VALUES (1, @ciphertext, @key_id, @public_jwk)
ON CONFLICT (id) DO NOTHING;
