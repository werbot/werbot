-- name: CreateBinding :one
-- Inserts nothing (no row) when the subject is not in the organization.
INSERT INTO access_binding (org_id, subject_type, subject_id, resource_id, label_selector, created_by)
SELECT @org_id::uuid, @subject_type::text, @subject_id::uuid, sqlc.narg(resource_id)::uuid, sqlc.narg(label_selector)::jsonb, @created_by::uuid
WHERE CASE @subject_type::text
    WHEN 'user' THEN EXISTS (SELECT 1 FROM membership WHERE org_id = @org_id AND user_id = @subject_id)
    ELSE EXISTS (SELECT 1 FROM team WHERE org_id = @org_id AND id = @subject_id)
END
RETURNING id, org_id, subject_type, subject_id, resource_id, label_selector, created_by, created_at;

-- name: DeleteBinding :execrows
DELETE FROM access_binding WHERE id = @id AND org_id = @org_id;

-- name: ListBindings :many
-- A set resource_id keeps the bindings to it and every selector binding.
SELECT id, org_id, subject_type, subject_id, resource_id, label_selector, created_by, created_at
FROM access_binding
WHERE org_id = @org_id
  AND (sqlc.narg(resource_id)::uuid IS NULL OR resource_id = sqlc.narg(resource_id)::uuid OR label_selector IS NOT NULL)
ORDER BY created_at, id;
