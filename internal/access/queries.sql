-- name: CreateBinding :one
-- Inserts nothing (no row) when the subject is not in the organization.
-- Ports travel as two parallel arrays (end_port 0: the single port), are
-- stored as closed int4range values and read back as lower and upper - 1.
INSERT INTO access_binding (org_id, subject_type, subject_id, resource_id, label_selector, created_by, ports)
SELECT @org_id::uuid, @subject_type::text, @subject_id::uuid, sqlc.narg(resource_id)::uuid, sqlc.narg(label_selector)::jsonb, @created_by::uuid,
    ARRAY(SELECT int4range((@ports::int4[])[i], coalesce(nullif((@end_ports::int4[])[i], 0), (@ports::int4[])[i]), '[]')
          FROM generate_subscripts(@ports::int4[], 1) AS i ORDER BY i)
WHERE CASE @subject_type::text
    WHEN 'user' THEN EXISTS (SELECT 1 FROM membership WHERE org_id = @org_id AND user_id = @subject_id)
    ELSE EXISTS (SELECT 1 FROM team WHERE org_id = @org_id AND id = @subject_id)
END
RETURNING id, org_id, subject_type, subject_id, resource_id, label_selector, created_by, created_at,
    ARRAY(SELECT lower(r) FROM unnest(ports) WITH ORDINALITY AS u(r, o) ORDER BY o)::int4[] AS ports,
    ARRAY(SELECT upper(r) - 1 FROM unnest(ports) WITH ORDINALITY AS u(r, o) ORDER BY o)::int4[] AS end_ports;

-- name: DeleteBinding :one
DELETE FROM access_binding WHERE id = @id AND org_id = @org_id
RETURNING subject_type, subject_id, resource_id, label_selector;

-- name: ListBindings :many
-- A set resource_id keeps the bindings to it and every selector binding.
SELECT id, org_id, subject_type, subject_id, resource_id, label_selector, created_by, created_at,
    ARRAY(SELECT lower(r) FROM unnest(ports) WITH ORDINALITY AS u(r, o) ORDER BY o)::int4[] AS ports,
    ARRAY(SELECT upper(r) - 1 FROM unnest(ports) WITH ORDINALITY AS u(r, o) ORDER BY o)::int4[] AS end_ports
FROM access_binding
WHERE org_id = @org_id
  AND (sqlc.narg(resource_id)::uuid IS NULL OR resource_id = sqlc.narg(resource_id)::uuid OR label_selector IS NOT NULL)
ORDER BY created_at, id;
