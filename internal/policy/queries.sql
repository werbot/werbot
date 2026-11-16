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
