-- +goose Up
-- +goose StatementBegin
-- Project invitations for every project: pending to a new address, pending to
-- an existing profile, accepted by a member, and deleted/archived history.
INSERT INTO "token" (
  "id", "section", "action", "status", "profile_id", "project_id",
  "scheme_id", "data", "expired_at", "updated_at", "created_at"
)
SELECT
  md5('seed-project-invite-' || p.id || '-' || n)::uuid,
  2,
  7,
  CASE n
    WHEN 1 THEN 2
    WHEN 2 THEN 1
    WHEN 3 THEN CASE WHEN p.rn % 2 = 0 THEN 4 ELSE 5 END
    ELSE 2
  END,
  CASE
    WHEN n = 2 THEN member.profile_id
    WHEN n = 4 THEN pending.profile_id
  END,
  p.id,
  NULL,
  CASE
    WHEN n = 2 AND member.profile_id IS NOT NULL THEN jsonb_build_object(
      'name', member.name,
      'surname', member.surname,
      'email', member.email
    )
    WHEN n = 4 AND pending.profile_id IS NOT NULL THEN jsonb_build_object(
      'name', pending.name,
      'surname', pending.surname,
      'email', pending.email
    )
    ELSE jsonb_build_object(
      'name', 'invite' || p.rn || n,
      'surname', 'project' || p.rn,
      'email', 'invite-' || p.alias || '-' || n || '@werbot.net'
    )
  END,
  CASE
    WHEN n IN (2, 3) THEN CURRENT_TIMESTAMP - ((p.rn % 10) + 1) * INTERVAL '1 day'
    ELSE CURRENT_TIMESTAMP + ((p.rn % 20) + 1) * INTERVAL '1 day'
  END,
  CASE
    WHEN n = 2 THEN CURRENT_TIMESTAMP - ((p.rn % 10) + 1) * INTERVAL '1 day'
  END,
  CURRENT_TIMESTAMP - ((p.rn + n) % 30) * INTERVAL '1 day'
FROM (
  SELECT
    "project"."id",
    "project"."owner_id",
    "project"."alias",
    row_number() OVER (ORDER BY "project"."id") AS rn
  FROM "project"
) AS p
CROSS JOIN generate_series(1, 4) AS n
LEFT JOIN LATERAL (
  SELECT
    "project_member"."profile_id",
    "profile"."name",
    "profile"."surname",
    "profile"."email"
  FROM "project_member"
  INNER JOIN "profile" ON "profile"."id" = "project_member"."profile_id"
  WHERE "project_member"."project_id" = p.id
  ORDER BY "project_member"."role" DESC, "project_member"."created_at" ASC, "project_member"."id"
  LIMIT 1
) AS member ON true
LEFT JOIN LATERAL (
  SELECT
    "profile"."id" AS profile_id,
    "profile"."name",
    "profile"."surname",
    "profile"."email"
  FROM "profile"
  WHERE "profile"."id" <> p.owner_id
    AND NOT EXISTS (
      SELECT 1 FROM "project_member"
      WHERE "project_member"."project_id" = p.id
        AND "project_member"."profile_id" = "profile"."id"
    )
  ORDER BY md5(p.id::text || "profile"."id"::text)
  LIMIT 1
) AS pending ON true;

-- Scheme invitations for every scheme, with sent and accepted history.
INSERT INTO "token" (
  "id", "section", "action", "status", "profile_id", "project_id",
  "scheme_id", "data", "expired_at", "updated_at", "created_at"
)
SELECT
  md5('seed-scheme-invite-' || s.id)::uuid,
  3,
  3,
  CASE
    WHEN s.rn % 7 = 0 THEN 1
    WHEN s.rn % 5 = 0 THEN 4
    ELSE 2
  END,
  CASE WHEN s.rn % 7 = 0 THEN member.profile_id END,
  s.project_id,
  s.id,
  jsonb_build_object(
    'email', 'scheme-invite-' || substr(md5(s.id::text), 1, 8) || '@werbot.net',
    'scheme_type', s.scheme_type
  ),
  CASE
    WHEN s.rn % 7 = 0 THEN CURRENT_TIMESTAMP - INTERVAL '1 day'
    ELSE CURRENT_TIMESTAMP + ((s.rn % 20) + 1) * INTERVAL '1 day'
  END,
  CASE
    WHEN s.rn % 7 = 0 THEN CURRENT_TIMESTAMP - INTERVAL '1 day'
  END,
  CURRENT_TIMESTAMP - ((s.rn % 30) + 1) * INTERVAL '1 day'
FROM (
  SELECT
    "scheme"."id",
    "scheme"."project_id",
    "scheme"."scheme_type",
    row_number() OVER (ORDER BY "scheme"."id") AS rn
  FROM "scheme"
) AS s
LEFT JOIN LATERAL (
  SELECT "project_member"."profile_id"
  FROM "scheme_member"
  INNER JOIN "project_member" ON "project_member"."id" = "scheme_member"."project_member_id"
  WHERE "scheme_member"."scheme_id" = s.id
  ORDER BY "scheme_member"."active" DESC, "scheme_member"."created_at" ASC
  LIMIT 1
) AS member ON true;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM "token"
WHERE "id" IN (
  SELECT md5('seed-project-invite-' || p."id" || '-' || n)::uuid
  FROM "project" AS p
  CROSS JOIN generate_series(1, 4) AS n
  UNION
  SELECT md5('seed-scheme-invite-' || s."id")::uuid
  FROM "scheme" AS s
);
-- +goose StatementEnd
