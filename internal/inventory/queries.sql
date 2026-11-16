-- name: CreateResource :one
INSERT INTO resource (org_id, name, type, category, gateway_id, config)
VALUES (@org_id, @name, @type, @category, @gateway_id, @config)
RETURNING id;

-- name: UpdateResource :execrows
UPDATE resource
SET name = @name, gateway_id = @gateway_id, config = coalesce(sqlc.narg(config)::jsonb, config), updated_at = now()
WHERE id = @id AND org_id = @org_id;

-- name: DeleteResource :execrows
DELETE FROM resource WHERE id = @id AND org_id = @org_id;

-- name: GetResource :one
SELECT r.id, r.org_id, r.name, r.type, r.category, r.gateway_id, r.config, r.created_at, r.updated_at,
       coalesce((SELECT jsonb_object_agg(l.key, l.value) FROM resource_label l WHERE l.resource_id = r.id), '{}')::jsonb AS labels,
       h.key_type, h.public_key, h.fingerprint_sha256, h.created_at AS host_key_created_at
FROM resource r
LEFT JOIN resource_host_key h ON h.resource_id = r.id
WHERE r.id = @id AND r.org_id = @org_id;

-- name: ListResources :many
-- An empty label_filter matches everything; a NULL gateway_id means any gateway.
SELECT r.id, r.org_id, r.name, r.type, r.category, r.gateway_id, r.config, r.created_at, r.updated_at,
       coalesce((SELECT jsonb_object_agg(l.key, l.value) FROM resource_label l WHERE l.resource_id = r.id), '{}')::jsonb AS labels,
       h.key_type, h.public_key, h.fingerprint_sha256, h.created_at AS host_key_created_at
FROM resource r
LEFT JOIN resource_host_key h ON h.resource_id = r.id
WHERE r.org_id = @org_id
  AND (sqlc.narg(gateway_id)::uuid IS NULL OR r.gateway_id = sqlc.narg(gateway_id)::uuid)
  AND NOT EXISTS (
      SELECT 1 FROM jsonb_each_text(@label_filter::jsonb) f
      WHERE NOT EXISTS (
          SELECT 1 FROM resource_label l
          WHERE l.resource_id = r.id AND l.key = f.key AND l.value = f.value))
ORDER BY r.name;

-- name: DeleteLabels :exec
DELETE FROM resource_label WHERE resource_id = @resource_id AND org_id = @org_id;

-- name: InsertLabels :exec
INSERT INTO resource_label (resource_id, org_id, key, value)
SELECT @resource_id::uuid, @org_id::uuid, key, value FROM jsonb_each_text(@labels::jsonb);

-- name: InsertHostKey :execrows
INSERT INTO resource_host_key (resource_id, org_id, key_type, public_key, fingerprint_sha256)
VALUES (@resource_id, @org_id, @key_type, @public_key, @fingerprint_sha256)
ON CONFLICT (resource_id) DO NOTHING;

-- name: GetHostKeyFingerprint :one
SELECT fingerprint_sha256 FROM resource_host_key WHERE resource_id = @resource_id AND org_id = @org_id;

-- name: DeleteHostKey :execrows
DELETE FROM resource_host_key WHERE resource_id = @resource_id AND org_id = @org_id;
