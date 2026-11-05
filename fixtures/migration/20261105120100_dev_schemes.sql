-- +goose Up
-- +goose StatementBegin
-- New schemes give the scheme-linked development data (activity, host keys,
-- firewall rules, members, sessions, audits) a stable parent without touching
-- the base fixture rows from 20221122125822_init_data.sql.
INSERT INTO "scheme" (
  "id", "project_id", "title", "description", "active", "audit", "online",
  "scheme_type", "access", "access_policy", "created_at"
)
SELECT
  md5('seed-scheme-' || n)::uuid,
  CASE
    WHEN n <= 8 THEN '2bef1080-cd6e-49e5-8042-1224cf6a3da9'::uuid -- project1
    ELSE 'd958ee44-a960-420e-9bbf-c7a35084c4aa'::uuid            -- user project
  END,
  CASE WHEN n % 3 = 0 THEN 'Seed TCP service #' || n ELSE 'Seed SSH server #' || n END,
  'Seed scheme generated for local development',
  (n % 5 <> 0),
  (n % 4 = 0),
  (n % 6 = 0),
  CASE WHEN n % 3 = 0 THEN 101 ELSE 103 END,
  CASE
    WHEN n % 3 = 0 THEN jsonb_build_object(
      'alias', substr(md5('seed-scheme-' || n), 1, 6),
      'address', '127.0.1.' || (n + 1),
      'port', 2300 + n
    )
    ELSE jsonb_build_object(
      'alias', substr(md5('seed-scheme-' || n), 1, 6),
      'address', '127.0.2.' || (n + 1),
      'port', 2200 + n,
      'password', jsonb_build_object(
        'login', 'seed',
        'password', 'seed-password-' || n
      )
    )
  END,
  CASE WHEN n % 2 = 0
    THEN '{"country":1,"network":1}'::jsonb
    ELSE '{"country":0,"network":0}'::jsonb
  END,
  CURRENT_TIMESTAMP - (n % 30) * INTERVAL '1 day' - (n % 24) * INTERVAL '1 hour'
FROM generate_series(1, 12) AS n;

-- Every SSH scheme keeps a host key row (the scheme trigger creates the row,
-- this fills it for both base fixtures and the schemes above).
UPDATE "scheme_host_key" AS h
SET
  "host_key" = decode(md5('seed-host-key-' || h."scheme_id"), 'hex'),
  "updated_at" = CURRENT_TIMESTAMP - INTERVAL '1 hour'
FROM "scheme" AS s
WHERE s."id" = h."scheme_id"
  AND s."scheme_type" = 103;

-- Varied 24-hour weekly grids for every scheme owned by project1.
WITH grids AS (
  SELECT
    s."id",
    (
      SELECT jsonb_object_agg(d.day_name, d.hours)
      FROM (
        SELECT
          v.day_name,
          (
            SELECT jsonb_agg(
              CASE
                WHEN ((h + v.day_index * 3 + (ascii(substr(md5(s."id"::text), 1, 1)) % 5)) % 9) = 0
                  THEN 0
                ELSE 1
              END
              ORDER BY h
            )
            FROM generate_series(0, 23) AS h
          ) AS hours
        FROM (VALUES
          ('mon', 0), ('tue', 1), ('wed', 2), ('thu', 3),
          ('fri', 4), ('sat', 5), ('sun', 6)
        ) AS v(day_name, day_index)
      ) AS d
    ) AS data
  FROM "scheme" AS s
  WHERE s."project_id" = '2bef1080-cd6e-49e5-8042-1224cf6a3da9'::uuid
)
UPDATE "scheme_activity" AS sa
SET "data" = g.data
FROM grids AS g
WHERE g."id" = sa."scheme_id";

INSERT INTO "scheme_firewall_country" ("id", "scheme_id", "country_code", "created_at")
SELECT
  md5('seed-scheme-fw-country-' || n)::uuid,
  md5('seed-scheme-' || (1 + ((n - 1) % 12)))::uuid,
  (ARRAY[
    'DE', 'FR', 'GB', 'NL', 'SE', 'FI', 'EE', 'UA', 'KZ', 'TR', 'JP', 'BR',
    'CA', 'AU', 'IN', 'ES', 'IT', 'CH', 'AT', 'CZ', 'DK', 'NO', 'PT', 'IE',
    'BE', 'LU', 'SK', 'HU', 'RO', 'BG', 'GR', 'HR', 'SI', 'PL', 'LT', 'LV'
  ])[1 + ((n - 1) % 36)],
  CURRENT_TIMESTAMP - (n % 30) * INTERVAL '1 day'
FROM generate_series(1, 36) AS n;

INSERT INTO "scheme_firewall_network" ("id", "scheme_id", "network", "created_at")
SELECT
  md5('seed-scheme-fw-network-' || n)::uuid,
  md5('seed-scheme-' || (1 + ((n - 1) % 12)))::uuid,
  ('10.' || (100 + ((n - 1) % 20)) || '.' || ((n - 1) % 256) || '.0/24')::cidr,
  CURRENT_TIMESTAMP - (n % 30) * INTERVAL '1 day'
FROM generate_series(1, 36) AS n;

-- Two members per seeded scheme so sessions and audits can hang off them.
INSERT INTO "scheme_member" (
  "id", "scheme_id", "project_member_id", "active", "online", "created_at"
)
SELECT
  md5('seed-scheme-member-' || n)::uuid,
  md5('seed-scheme-' || (1 + ((n - 1) % 12)))::uuid,
  CASE
    WHEN (1 + ((n - 1) % 12)) <= 8 THEN
      CASE
        WHEN n <= 12 THEN '9d3f7efc-14a5-436d-a763-314441d6e0a5'::uuid -- project1, user
        ELSE '92de7a44-08fc-4d42-aab5-37f86fd598a2'::uuid            -- project1, user1
      END
    ELSE
      CASE
        WHEN n <= 12 THEN '4fc69519-b683-46f0-860c-3e7f12a17563'::uuid -- user project, admin
        ELSE '49a10a09-0bb3-48af-99cb-181533692585'::uuid            -- user project, user1
      END
  END,
  (n % 3 <> 0),
  (n % 4 = 0),
  CURRENT_TIMESTAMP - (n % 30) * INTERVAL '1 day'
FROM generate_series(1, 24) AS n;

-- Extra global firewall list rows (the base fixture only seeds one network).
INSERT INTO "firewall_country" ("firewall_list_id", "country_code")
SELECT
  (ARRAY[
    'afe7547c-f7af-41f0-bf80-dd72b807834f'::uuid,
    '282a74e4-ca06-4d72-877b-a56bf841cb57'::uuid,
    'a30bca4e-639e-45de-b1d2-d2039c1b6acc'::uuid
  ])[1 + (n % 3)],
  (ARRAY[
    'DE', 'FR', 'GB', 'NL', 'SE', 'FI', 'EE', 'UA', 'KZ', 'TR',
    'JP', 'BR', 'CA', 'AU', 'IN', 'ES', 'IT', 'CH', 'AT', 'CZ'
  ])[1 + (n % 20)]
FROM generate_series(1, 40) AS n;

INSERT INTO "firewall_network" ("firewall_list_id", "network")
SELECT
  (ARRAY[
    'afe7547c-f7af-41f0-bf80-dd72b807834f'::uuid,
    '282a74e4-ca06-4d72-877b-a56bf841cb57'::uuid,
    'a30bca4e-639e-45de-b1d2-d2039c1b6acc'::uuid
  ])[1 + (n % 3)],
  ('203.0.113.' || ((n - 1) % 256) || '/32')::cidr
FROM generate_series(1, 40) AS n;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM "firewall_network"
WHERE "firewall_list_id" IN (
  'afe7547c-f7af-41f0-bf80-dd72b807834f'::uuid,
  '282a74e4-ca06-4d72-877b-a56bf841cb57'::uuid,
  'a30bca4e-639e-45de-b1d2-d2039c1b6acc'::uuid
)
AND "network" <<= '203.0.113.0/24'::cidr;

DELETE FROM "firewall_country"
WHERE "firewall_list_id" IN (
  'afe7547c-f7af-41f0-bf80-dd72b807834f'::uuid,
  '282a74e4-ca06-4d72-877b-a56bf841cb57'::uuid,
  'a30bca4e-639e-45de-b1d2-d2039c1b6acc'::uuid
)
AND "country_code" IN (
  'DE', 'FR', 'GB', 'NL', 'SE', 'FI', 'EE', 'UA', 'KZ', 'TR',
  'JP', 'BR', 'CA', 'AU', 'IN', 'ES', 'IT', 'CH', 'AT', 'CZ'
);

-- Restore the base fixture activity grid for project1 schemes that were not
-- created here. Seeded schemes are removed below and cascade their rows.
UPDATE "scheme_activity" AS sa
SET "data" = '{"mon":[1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1],"tue":[1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1],"wed":[1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1],"thu":[1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1],"fri":[1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1],"sat":[1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1],"sun":[1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1,1]}'::jsonb
FROM "scheme" AS s
WHERE s."id" = sa."scheme_id"
  AND s."project_id" = '2bef1080-cd6e-49e5-8042-1224cf6a3da9'::uuid
  AND s."id" NOT IN (
    SELECT md5('seed-scheme-' || n)::uuid FROM generate_series(1, 12) AS n
  );

-- The base fixture customised this one grid; put it back exactly.
UPDATE "scheme_activity"
SET "data" = '{"mon":[0,1,1,1,1,1,1,1,1,1,1,1,0,1,1,1,1,1,1,1,1,1,1,1],"tue":[1,0,1,1,1,1,1,1,1,1,1,0,1,0,1,1,1,1,1,1,1,1,1,0],"wed":[1,1,0,1,1,1,1,1,1,1,0,1,1,1,0,1,1,1,1,1,1,1,0,1],"thu":[1,1,1,0,1,1,1,1,1,0,1,1,1,1,1,0,1,1,1,1,1,0,1,1],"fri":[1,1,1,1,0,1,1,1,0,1,1,1,1,1,1,1,0,1,1,1,0,1,1,1],"sat":[1,1,1,1,1,0,1,0,1,1,1,1,1,1,1,1,1,0,1,0,1,1,1,1],"sun":[1,1,1,1,1,1,0,1,1,1,1,1,1,1,1,1,1,1,0,1,1,1,1,1]}'::jsonb
WHERE "scheme_id" = '156d8d65-cfe5-48a4-a636-198a5f509abf'::uuid;

-- Restore base host keys (the base fixture only sets one explicit value).
UPDATE "scheme_host_key"
SET "host_key" = NULL, "updated_at" = NULL
WHERE "scheme_id" IN (
  SELECT "id" FROM "scheme" WHERE "scheme_type" = 103
)
AND "scheme_id" NOT IN (
  SELECT md5('seed-scheme-' || n)::uuid FROM generate_series(1, 12) AS n
);
UPDATE "scheme_host_key"
SET "host_key" = '\x746573740A'::bytea
WHERE "scheme_id" = '156d8d65-cfe5-48a4-a636-198a5f509abf'::uuid;

DELETE FROM "scheme"
WHERE "id" IN (
  SELECT md5('seed-scheme-' || n)::uuid FROM generate_series(1, 12) AS n
);
-- +goose StatementEnd
