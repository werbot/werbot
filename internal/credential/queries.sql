-- name: UpsertCredential :execrows
-- The DO UPDATE only touches a row of the same organization.
INSERT INTO credential (org_id, resource_id, kind, ciphertext, key_id)
VALUES (@org_id, @resource_id, @kind, @ciphertext, @key_id)
ON CONFLICT (resource_id) DO UPDATE
SET kind = EXCLUDED.kind, ciphertext = EXCLUDED.ciphertext, key_id = EXCLUDED.key_id, updated_at = now()
WHERE credential.org_id = EXCLUDED.org_id;

-- name: DeleteCredential :execrows
DELETE FROM credential WHERE resource_id = @resource_id AND org_id = @org_id;

-- name: GetCredential :one
SELECT kind, ciphertext, key_id FROM credential WHERE resource_id = @resource_id AND org_id = @org_id;

-- name: ListCredentialKinds :many
SELECT resource_id, kind FROM credential WHERE org_id = @org_id AND resource_id = ANY(@resource_ids::uuid[]);
