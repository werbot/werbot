-- name: ListApprovalPolicies :many
SELECT id, org_id, name, resource_selector, requesters, approvers, mode,
    extract(epoch FROM max_duration)::bigint AS max_duration_secs,
    require_reason, four_eyes, break_glass, created_by, created_at
FROM approval_policy WHERE org_id = @org_id ORDER BY name;

-- name: CreateApprovalPolicy :one
INSERT INTO approval_policy (org_id, name, resource_selector, requesters, approvers, mode, max_duration,
    require_reason, four_eyes, break_glass, created_by)
VALUES (@org_id, @name, @resource_selector, @requesters, @approvers, @mode, make_interval(secs => @max_duration_secs::float8),
    @require_reason, @four_eyes, @break_glass, @created_by)
RETURNING id, org_id, name, resource_selector, requesters, approvers, mode,
    extract(epoch FROM max_duration)::bigint AS max_duration_secs,
    require_reason, four_eyes, break_glass, created_by, created_at;

-- name: UpdateApprovalPolicy :one
UPDATE approval_policy SET name = @name, resource_selector = @resource_selector, requesters = @requesters,
    approvers = @approvers, mode = @mode, max_duration = make_interval(secs => @max_duration_secs::float8),
    require_reason = @require_reason, four_eyes = @four_eyes, break_glass = @break_glass
WHERE id = @id AND org_id = @org_id
RETURNING id, org_id, name, resource_selector, requesters, approvers, mode,
    extract(epoch FROM max_duration)::bigint AS max_duration_secs,
    require_reason, four_eyes, break_glass, created_by, created_at;

-- name: DeleteApprovalPolicy :one
DELETE FROM approval_policy WHERE id = @id AND org_id = @org_id
RETURNING name, mode, break_glass;

-- name: CountPolicyRefs :one
-- How many of the given ids exist in the organization, one count per kind.
SELECT (SELECT count(*) FROM resource r WHERE r.org_id = @org_id AND r.id = ANY(@resource_ids::uuid[]))::int AS resources,
       (SELECT count(*) FROM membership m WHERE m.org_id = @org_id AND m.user_id = ANY(@user_ids::uuid[]))::int AS users,
       (SELECT count(*) FROM team t WHERE t.org_id = @org_id AND t.id = ANY(@team_ids::uuid[]))::int AS teams;
