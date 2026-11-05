-- +goose Up
-- +goose StatementBegin
-- Projects without members get three: one manager and two regular users.
INSERT INTO "project_member" (
  "id", "project_id", "profile_id", "active", "online", "role", "created_at"
)
SELECT
  md5('seed-project-member-' || p.id || '-' || pm.slot)::uuid,
  p.id,
  pm.id,
  (pm.slot <> 2),
  (pm.slot = 1),
  CASE WHEN pm.slot = 1 THEN 2 ELSE 1 END,
  CURRENT_TIMESTAMP - ((p.rn + pm.slot) % 20) * INTERVAL '1 day'
FROM (
  SELECT
    "project"."id",
    "project"."owner_id",
    row_number() OVER (ORDER BY "project"."id") AS rn
  FROM "project"
  WHERE NOT EXISTS (
    SELECT 1 FROM "project_member"
    WHERE "project_member"."project_id" = "project"."id"
  )
) AS p
CROSS JOIN LATERAL (
  SELECT
    "profile"."id",
    row_number() OVER (
      ORDER BY md5(p.id::text || "profile"."id"::text)
    ) AS slot
  FROM "profile"
  WHERE "profile"."id" <> p."owner_id"
  ORDER BY md5(p.id::text || "profile"."id"::text)
  LIMIT 3
) AS pm;

-- Safety net: every profile without a project membership joins two projects.
INSERT INTO "project_member" (
  "id", "project_id", "profile_id", "active", "online", "role", "created_at"
)
SELECT
  md5('seed-profile-access-' || need.id || '-' || chosen.slot)::uuid,
  chosen.project_id,
  need.id,
  (chosen.slot = 1),
  (chosen.slot = 2),
  CASE WHEN chosen.slot = 1 THEN 2 ELSE 1 END,
  CURRENT_TIMESTAMP - ((ascii(substr(md5(need.id::text), 1, 1)) + chosen.slot) % 20) * INTERVAL '1 day'
FROM (
  SELECT "profile"."id"
  FROM "profile"
  WHERE NOT EXISTS (
    SELECT 1 FROM "project_member"
    WHERE "project_member"."profile_id" = "profile"."id"
  )
) AS need
CROSS JOIN LATERAL (
  SELECT
    "project"."id" AS project_id,
    row_number() OVER (
      ORDER BY md5(need.id::text || "project"."id"::text)
    ) AS slot
  FROM "project"
  WHERE "project"."owner_id" <> need.id
  ORDER BY md5(need.id::text || "project"."id"::text)
  LIMIT 2
) AS chosen;

-- The four projects with base fixture members also get an explicit manager.
INSERT INTO "project_member" (
  "id", "project_id", "profile_id", "active", "online", "role", "created_at"
)
SELECT
  md5('seed-main-project-manager-' || p.id)::uuid,
  p.id,
  chosen.id,
  true,
  false,
  2,
  CURRENT_TIMESTAMP - INTERVAL '3 day'
FROM (VALUES
  ('2bef1080-cd6e-49e5-8042-1224cf6a3da9'::uuid), -- project1
  ('fe52ca9b-5599-4bb6-818b-1896d56e9aa2'::uuid), -- project2
  ('d958ee44-a960-420e-9bbf-c7a35084c4aa'::uuid), -- user project
  ('c22bc277-36c0-4a90-86e5-44b2f607046e'::uuid)  -- project4
) AS p(id)
CROSS JOIN LATERAL (
  SELECT "profile"."id"
  FROM "profile"
  WHERE "profile"."id" <> (
    SELECT "project"."owner_id" FROM "project" WHERE "project"."id" = p.id
  )
    AND NOT EXISTS (
      SELECT 1 FROM "project_member"
      WHERE "project_member"."project_id" = p.id
        AND "project_member"."profile_id" = "profile"."id"
    )
  ORDER BY md5(p.id::text || "profile"."id"::text)
  LIMIT 1
) AS chosen;

-- Every scheme gets at least one member so no server access list is empty.
INSERT INTO "scheme_member" (
  "id", "scheme_id", "project_member_id", "active", "online", "created_at"
)
SELECT
  md5('seed-scheme-access-' || s.id)::uuid,
  s.id,
  pm.id,
  (ascii(substr(md5(s.id::text), 1, 1)) % 3 <> 0),
  (ascii(substr(md5(s.id::text), 2, 1)) % 4 = 0),
  CURRENT_TIMESTAMP - (ascii(substr(md5(s.id::text), 3, 1)) % 20) * INTERVAL '1 day'
FROM "scheme" AS s
CROSS JOIN LATERAL (
  SELECT "project_member"."id"
  FROM "project_member"
  WHERE "project_member"."project_id" = s."project_id"
  ORDER BY "project_member"."active" DESC, "project_member"."created_at" ASC, "project_member"."id"
  LIMIT 1
) AS pm
WHERE NOT EXISTS (
  SELECT 1 FROM "scheme_member" WHERE "scheme_member"."scheme_id" = s."id"
);

-- Every profile without server access gets one scheme from a project it joined.
INSERT INTO "scheme_member" (
  "id", "scheme_id", "project_member_id", "active", "online", "created_at"
)
SELECT
  md5('seed-profile-scheme-access-' || need.id)::uuid,
  chosen.scheme_id,
  chosen.project_member_id,
  true,
  false,
  CURRENT_TIMESTAMP - INTERVAL '1 day'
FROM (
  SELECT "profile"."id"
  FROM "profile"
  WHERE NOT EXISTS (
    SELECT 1
    FROM "scheme_member"
    INNER JOIN "project_member" ON "scheme_member"."project_member_id" = "project_member"."id"
    WHERE "project_member"."profile_id" = "profile"."id"
  )
) AS need
CROSS JOIN LATERAL (
  SELECT
    "scheme"."id" AS scheme_id,
    "project_member"."id" AS project_member_id
  FROM "project_member"
  INNER JOIN "scheme" ON "scheme"."project_id" = "project_member"."project_id"
  WHERE "project_member"."profile_id" = need.id
  ORDER BY md5(need.id::text || "scheme"."id"::text)
  LIMIT 1
) AS chosen;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM "scheme_member"
WHERE "id" IN (
  SELECT md5('seed-scheme-access-' || s."id")::uuid FROM "scheme" AS s
  UNION
  SELECT md5('seed-profile-scheme-access-' || p."id")::uuid FROM "profile" AS p
);

DELETE FROM "project_member"
WHERE "id" IN (
  SELECT md5('seed-project-member-' || p."id" || '-' || n)::uuid
  FROM "project" AS p
  CROSS JOIN generate_series(1, 3) AS n
  UNION
  SELECT md5('seed-profile-access-' || p."id" || '-' || n)::uuid
  FROM "profile" AS p
  CROSS JOIN generate_series(1, 2) AS n
  UNION
  SELECT md5('seed-main-project-manager-' || p."id")::uuid
  FROM "project" AS p
);
-- +goose StatementEnd
