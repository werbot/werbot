-- name: ListNetworkRules :many
-- A set resource_id narrows the list to that resource.
SELECT resource_id, cidr, effect FROM resource_network_rule
WHERE org_id = @org_id AND (sqlc.narg(resource_id)::uuid IS NULL OR resource_id = sqlc.narg(resource_id)::uuid)
ORDER BY resource_id, cidr;

-- name: ListCountryRules :many
SELECT resource_id, country, effect FROM resource_country_rule
WHERE org_id = @org_id AND (sqlc.narg(resource_id)::uuid IS NULL OR resource_id = sqlc.narg(resource_id)::uuid)
ORDER BY resource_id, country;

-- name: ListSchedule :many
SELECT resource_id, weekday, hour FROM resource_schedule
WHERE org_id = @org_id AND (sqlc.narg(resource_id)::uuid IS NULL OR resource_id = sqlc.narg(resource_id)::uuid)
ORDER BY resource_id, weekday, hour;

-- name: DeleteNetworkRules :exec
DELETE FROM resource_network_rule WHERE org_id = @org_id AND resource_id = @resource_id;

-- name: DeleteCountryRules :exec
DELETE FROM resource_country_rule WHERE org_id = @org_id AND resource_id = @resource_id;

-- name: DeleteSchedule :exec
DELETE FROM resource_schedule WHERE org_id = @org_id AND resource_id = @resource_id;

-- name: InsertNetworkRule :exec
INSERT INTO resource_network_rule (org_id, resource_id, cidr, effect) VALUES (@org_id, @resource_id, @cidr, @effect);

-- name: InsertCountryRule :exec
INSERT INTO resource_country_rule (org_id, resource_id, country, effect) VALUES (@org_id, @resource_id, @country, @effect);

-- name: InsertSlot :exec
INSERT INTO resource_schedule (org_id, resource_id, weekday, hour) VALUES (@org_id, @resource_id, @weekday, @hour);

-- name: IPBlocked :one
SELECT EXISTS (SELECT 1 FROM ip_blocklist WHERE cidr >>= @ip::inet);

-- name: ListBlocklist :many
SELECT cidr FROM ip_blocklist ORDER BY cidr;

-- name: GetGeoIPData :one
SELECT data FROM geoip_db WHERE id = 1;

-- name: ReplaceBlocklist :copyfrom
INSERT INTO ip_blocklist (cidr, source) VALUES (@cidr, @source);

-- name: DeleteBlocklistSource :exec
DELETE FROM ip_blocklist WHERE source = @source;

-- name: GetGeoIPMonth :one
SELECT month FROM geoip_db WHERE id = 1;

-- name: UpsertGeoIP :exec
INSERT INTO geoip_db (id, month, data, updated_at) VALUES (1, @month, @data, now())
ON CONFLICT (id) DO UPDATE SET month = EXCLUDED.month, data = EXCLUDED.data, updated_at = now();

-- name: NotifyGeoIPChanged :exec
SELECT pg_notify('werbot_geoip_changed', '');

-- name: ListCommandRules :many
SELECT * FROM command_rule WHERE org_id = @org_id ORDER BY name;

-- name: CreateCommandRule :one
INSERT INTO command_rule (org_id, name, subjects, resource_selector, classes, effect, message, created_by)
VALUES (@org_id, @name, @subjects, @resource_selector, @classes, @effect, @message, @created_by)
RETURNING *;

-- name: UpdateCommandRule :one
UPDATE command_rule SET name = @name, subjects = @subjects, resource_selector = @resource_selector, classes = @classes,
    effect = @effect, message = @message
WHERE org_id = @org_id AND id = @id
RETURNING *;

-- name: DeleteCommandRule :one
DELETE FROM command_rule WHERE org_id = @org_id AND id = @id RETURNING *;

-- name: CountRuleRefs :one
-- How many of the given ids exist in the organization, one count per kind.
SELECT (SELECT count(*) FROM resource r WHERE r.org_id = @org_id AND r.id = ANY(@resource_ids::uuid[]))::int AS resources,
       (SELECT count(*) FROM membership m WHERE m.org_id = @org_id AND m.user_id = ANY(@user_ids::uuid[]))::int AS users,
       (SELECT count(*) FROM team t WHERE t.org_id = @org_id AND t.id = ANY(@team_ids::uuid[]))::int AS teams;

-- name: ListClassRules :many
SELECT * FROM command_class_rule WHERE org_id = @org_id ORDER BY pattern;

-- name: CreateClassRule :one
INSERT INTO command_class_rule (org_id, pattern, classes, created_by) VALUES (@org_id, @pattern, @classes, @created_by)
RETURNING *;

-- name: DeleteClassRule :one
DELETE FROM command_class_rule WHERE org_id = @org_id AND id = @id RETURNING *;

-- name: ListCustomPolicies :many
SELECT * FROM custom_policy WHERE org_id = @org_id ORDER BY name;

-- name: CreateCustomPolicy :one
INSERT INTO custom_policy (org_id, name, cedar, enabled, created_by) VALUES (@org_id, @name, @cedar, @enabled, @created_by)
RETURNING *;

-- name: UpdateCustomPolicy :one
UPDATE custom_policy SET name = @name, cedar = @cedar, enabled = @enabled, updated_at = now()
WHERE org_id = @org_id AND id = @id
RETURNING *;

-- name: DeleteCustomPolicy :one
DELETE FROM custom_policy WHERE org_id = @org_id AND id = @id RETURNING *;

-- name: IsMember :one
SELECT EXISTS (SELECT 1 FROM membership WHERE org_id = @org_id AND user_id = @user_id);

-- name: ListPathRules :many
SELECT * FROM path_rule WHERE org_id = @org_id ORDER BY name;

-- name: CreatePathRule :one
INSERT INTO path_rule (org_id, name, subjects, resource_selector, paths, access, effect, message, created_by)
VALUES (@org_id, @name, @subjects, @resource_selector, @paths, @access, @effect, @message, @created_by)
RETURNING *;

-- name: UpdatePathRule :one
UPDATE path_rule SET name = @name, subjects = @subjects, resource_selector = @resource_selector, paths = @paths,
    access = @access, effect = @effect, message = @message
WHERE org_id = @org_id AND id = @id
RETURNING *;

-- name: DeletePathRule :one
DELETE FROM path_rule WHERE org_id = @org_id AND id = @id RETURNING *;

-- name: ListTerminalRules :many
SELECT * FROM terminal_rule WHERE org_id = @org_id ORDER BY name;

-- name: CreateTerminalRule :one
INSERT INTO terminal_rule (org_id, name, subjects, resource_selector, mode, created_by)
VALUES (@org_id, @name, @subjects, @resource_selector, @mode, @created_by)
RETURNING *;

-- name: DeleteTerminalRule :one
DELETE FROM terminal_rule WHERE org_id = @org_id AND id = @id RETURNING *;

-- name: ListMFARules :many
SELECT * FROM mfa_rule WHERE org_id = @org_id ORDER BY name;

-- name: CreateMFARule :one
INSERT INTO mfa_rule (org_id, name, subjects, resource_selector, max_age, strength, created_by)
VALUES (@org_id, @name, @subjects, @resource_selector, @max_age, @strength, @created_by)
RETURNING *;

-- name: UpdateMFARule :one
UPDATE mfa_rule SET name = @name, subjects = @subjects, resource_selector = @resource_selector, max_age = @max_age, strength = @strength
WHERE org_id = @org_id AND id = @id
RETURNING *;

-- name: DeleteMFARule :one
DELETE FROM mfa_rule WHERE org_id = @org_id AND id = @id RETURNING *;
