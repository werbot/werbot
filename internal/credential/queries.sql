-- name: UpsertCredential :execrows
-- The DO UPDATE only touches a row of the same organization.
INSERT INTO credential (org_id, resource_id, kind, ciphertext, key_id, slot)
VALUES (@org_id, @resource_id, @kind, @ciphertext, @key_id, @slot)
ON CONFLICT (resource_id, slot) DO UPDATE
SET kind = EXCLUDED.kind, ciphertext = EXCLUDED.ciphertext, key_id = EXCLUDED.key_id, updated_at = now()
WHERE credential.org_id = EXCLUDED.org_id;

-- name: DeleteCredential :execrows
DELETE FROM credential WHERE resource_id = @resource_id AND org_id = @org_id AND slot = @slot;

-- name: GetCredential :one
SELECT kind, ciphertext, key_id, updated_at FROM credential WHERE resource_id = @resource_id AND org_id = @org_id AND slot = @slot;

-- name: ListCredentialKinds :many
SELECT resource_id, kind FROM credential WHERE org_id = @org_id AND slot = @slot AND resource_id = ANY(@resource_ids::uuid[]);

-- name: SetAgentReadonlyStatus :exec
-- The verification state of the agent_readonly slot; resource.updated_at is
-- part of the check version and stays. The resource trigger notifies the snapshot.
UPDATE resource SET agent_readonly_status = @status, agent_readonly_detail = ''
WHERE id = @resource_id AND org_id = @org_id;
