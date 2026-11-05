-- +goose Up
-- +goose StatementBegin
-- Every project that has no schemes gets three schemes with different types,
-- authentication settings, access policies, states and firewall rules. This
-- keeps project cards, scheme lists, dashboards and firewall views populated
-- for all development projects.
INSERT INTO "scheme" (
  "id", "project_id", "title", "description", "active", "audit", "online",
  "scheme_type", "access", "access_policy", "locked_at", "archived_at",
  "created_at"
)
SELECT
  md5('seed-project-scheme-' || p.id || '-' || slot.n)::uuid,
  p.id,
  'Seed ' || CASE slot.n
    WHEN 1 THEN 'server'
    WHEN 2 THEN 'database'
    ELSE 'resource'
  END || ' #' || ((p.rn - 1) * 3 + slot.n),
  'Development fixture: project coverage',
  ((p.rn + slot.n) % 7 <> 0),
  ((p.rn + slot.n) % 5 = 0),
  ((p.rn + slot.n) % 6 = 0),
  t.scheme_type,
  CASE t.scheme_type
    WHEN 101 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'address', '10.20.' || (((p.rn - 1) % 200) + 1) || '.' || (10 + slot.n),
      'port', 2000 + ((p.rn * 3 + slot.n) % 500)
    )
    WHEN 102 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'address', '10.21.' || (((p.rn - 1) % 200) + 1) || '.' || (10 + slot.n),
      'port', 3000 + ((p.rn * 3 + slot.n) % 500)
    )
    WHEN 103 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'address', '10.22.' || (((p.rn - 1) % 200) + 1) || '.' || (10 + slot.n),
      'port', 2200 + ((p.rn * 3 + slot.n) % 500),
      'password', jsonb_build_object(
        'login', 'seeduser',
        'password', 'seed-password-' || p.rn
      )
    )
    WHEN 104 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'address', '10.23.' || (((p.rn - 1) % 200) + 1) || '.' || (10 + slot.n),
      'port', 23,
      'access', jsonb_build_object(
        'login', 'seeduser',
        'password', 'seed-password-' || p.rn
      )
    )
    WHEN 201 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'address', '10.30.' || (((p.rn - 1) % 200) + 1) || '.' || (10 + slot.n),
      'port', 3306,
      'database_name', 'seed_db_' || p.rn,
      'access', jsonb_build_object(
        'login', 'seeduser',
        'password', 'seed-password-' || p.rn
      ),
      'mtls', jsonb_build_object('server_ca', '', 'client_cert', '', 'client_key', '')
    )
    WHEN 202 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'address', '10.31.' || (((p.rn - 1) % 200) + 1) || '.' || (10 + slot.n),
      'port', 5432,
      'database_name', 'seed_db_' || p.rn,
      'access', jsonb_build_object(
        'login', 'seeduser',
        'password', 'seed-password-' || p.rn
      ),
      'mtls', jsonb_build_object('server_ca', '', 'client_cert', '', 'client_key', ''),
      'server_name_mtls', ''
    )
    WHEN 203 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'address', '10.32.' || (((p.rn - 1) % 200) + 1) || '.' || (10 + slot.n),
      'port', 6379,
      'access', jsonb_build_object(
        'login', 'seeduser',
        'password', 'seed-password-' || p.rn
      ),
      'tls_required', ((p.rn + slot.n) % 2 = 0)
    )
    WHEN 204 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'address', '10.33.' || (((p.rn - 1) % 200) + 1) || '.' || (10 + slot.n),
      'port', 27017,
      'database_name', 'seed_db_' || p.rn,
      'access', jsonb_build_object(
        'login', 'seeduser',
        'password', 'seed-password-' || p.rn
      ),
      'tls_required', ((p.rn + slot.n) % 2 = 0),
      'replica_connect', ((p.rn + slot.n) % 3 = 0)
    )
    WHEN 205 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'address', '10.34.' || (((p.rn - 1) % 200) + 1) || '.' || (10 + slot.n),
      'port', 9300,
      'access', jsonb_build_object(
        'login', 'seeduser',
        'password', 'seed-password-' || p.rn
      ),
      'tls_required', ((p.rn + slot.n) % 2 = 0)
    )
    WHEN 206 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'region', (ARRAY['us-east-1', 'us-west-2', 'eu-central-1', 'ap-southeast-2'])[1 + ((p.rn + slot.n) % 4)::int],
      'api', jsonb_build_object(
        'access_key_id', '',
        'secret_access_key', '',
        'role_arn', '',
        'role_external_id', ''
      )
    )
    WHEN 207 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'address', '10.35.' || (((p.rn - 1) % 200) + 1) || '.' || (10 + slot.n),
      'port', 7001,
      'access', jsonb_build_object(
        'login', 'seeduser',
        'password', 'seed-password-' || p.rn
      ),
      'tls_required', ((p.rn + slot.n) % 2 = 0)
    )
    WHEN 208 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'address', '10.36.' || (((p.rn - 1) % 200) + 1) || '.' || (10 + slot.n),
      'port', 1433,
      'access', jsonb_build_object(
        'login', 'seeduser',
        'password', 'seed-password-' || p.rn
      ),
      'schema', 'dbo',
      'default_database', ((p.rn + slot.n) % 2 = 0)
    )
    WHEN 209 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'region', (ARRAY['us-east-1', 'us-west-2', 'eu-central-1', 'ap-southeast-2'])[1 + ((p.rn + slot.n) % 4)::int],
      'access', jsonb_build_object(
        'login', 'seeduser',
        'password', 'seed-password-' || p.rn
      ),
      'schema', 'PUBLIC'
    )
    WHEN 301 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'address', '10.40.' || (((p.rn - 1) % 200) + 1) || '.' || (10 + slot.n),
      'port', 3389,
      'access', jsonb_build_object(
        'login', 'seeduser',
        'password', 'seed-password-' || p.rn
      )
    )
    WHEN 302 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'address', '10.41.' || (((p.rn - 1) % 200) + 1) || '.' || (10 + slot.n),
      'port', 5900,
      'access', jsonb_build_object(
        'login', 'seeduser',
        'password', 'seed-password-' || p.rn
      )
    )
    WHEN 401 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'address', '10.42.' || (((p.rn - 1) % 200) + 1) || '.' || (10 + slot.n),
      'port', 2375,
      'mtls', jsonb_build_object('server_ca', '', 'client_cert', '', 'client_key', '')
    )
    WHEN 402 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'address', '10.43.' || (((p.rn - 1) % 200) + 1) || '.' || (10 + slot.n),
      'port', 9000,
      'mtls', jsonb_build_object('server_ca', '', 'client_cert', '', 'client_key', ''),
      'healthcheck_namespace', 'default'
    )
    WHEN 501 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'access', jsonb_build_object(
        'access_key_id', '',
        'secret_access_key', '',
        'role_arn', '',
        'role_external_id', ''
      ),
      'healthcheck_region', 'us-east-1'
    )
    WHEN 502 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'service_account_keyfile', '',
      'scopes', ''
    )
    WHEN 503 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'app_id', '',
      'tenant', '',
      'password', '',
      'certificate', ''
    )
    WHEN 504 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'access', jsonb_build_object(
        'access_key_id', '',
        'secret_access_key', '',
        'role_arn', '',
        'role_external_id', ''
      )
    )
    WHEN 505 THEN jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6),
      'access', jsonb_build_object(
        'access_key_id', '',
        'secret_access_key', '',
        'role_arn', '',
        'role_external_id', ''
      )
    )
    ELSE jsonb_build_object(
      'alias', substr(md5('seed-project-scheme-' || p.id || '-' || slot.n), 1, 6)
    )
  END,
  CASE (p.rn + slot.n) % 4
    WHEN 0 THEN '{"country":0,"network":0}'::jsonb
    WHEN 1 THEN '{"country":1,"network":0}'::jsonb
    WHEN 2 THEN '{"country":0,"network":1}'::jsonb
    ELSE '{"country":1,"network":1}'::jsonb
  END,
  CASE WHEN (p.rn + slot.n) % 17 = 0
    THEN CURRENT_TIMESTAMP - INTERVAL '1 day'
  END,
  CASE WHEN (p.rn + slot.n) % 23 = 0
    THEN CURRENT_TIMESTAMP - INTERVAL '2 day'
  END,
  CURRENT_TIMESTAMP
    - ((p.rn * 3 + slot.n) % 60) * INTERVAL '1 day'
    - ((p.rn + slot.n) % 24) * INTERVAL '1 hour'
FROM (
  SELECT
    "project"."id",
    "project"."owner_id",
    row_number() OVER (ORDER BY "project"."id") AS rn
  FROM "project"
  WHERE NOT EXISTS (
    SELECT 1 FROM "scheme" WHERE "scheme"."project_id" = "project"."id"
  )
) AS p
CROSS JOIN generate_series(1, 3) AS slot(n)
CROSS JOIN LATERAL (
  SELECT CASE slot.n
    WHEN 1 THEN (ARRAY[101, 102, 103, 104])[1 + (p.rn % 4)::int]
    WHEN 2 THEN (ARRAY[201, 202, 203, 204, 205, 206, 207, 208, 209])[1 + (p.rn % 9)::int]
    ELSE (ARRAY[301, 302, 401, 402, 501, 502, 503, 504, 505, 601])[1 + (p.rn % 10)::int]
  END AS scheme_type
) AS t;

-- Give every seeded scheme a different weekly activity grid.
WITH seeded AS (
  SELECT md5('seed-project-scheme-' || p.id || '-' || n)::uuid AS id
  FROM "project" AS p
  CROSS JOIN generate_series(1, 3) AS n
),
grids AS (
  SELECT
    seeded.id,
    (
      SELECT jsonb_object_agg(d.day_name, d.hours)
      FROM (
        SELECT
          v.day_name,
          (
            SELECT jsonb_agg(
              CASE
                WHEN ((h + v.day_index * 2 + (ascii(substr(md5(seeded.id::text), 1, 1)) % 4)) % 9) = 0
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
  FROM seeded
)
UPDATE "scheme_activity" AS sa
SET "data" = g.data
FROM grids AS g
WHERE g.id = sa."scheme_id";

-- SSH schemes need a host key; the insert trigger creates the row.
UPDATE "scheme_host_key" AS h
SET
  "host_key" = decode(md5('seed-project-host-key-' || h."scheme_id"), 'hex'),
  "updated_at" = CURRENT_TIMESTAMP - INTERVAL '1 hour'
FROM "scheme" AS s
WHERE s."id" = h."scheme_id"
  AND s."scheme_type" = 103
  AND s."id" IN (
    SELECT md5('seed-project-scheme-' || p.id || '-' || n)::uuid
    FROM "project" AS p
    CROSS JOIN generate_series(1, 3) AS n
  );

-- Firewall lists differ per scheme.
INSERT INTO "scheme_firewall_country" ("id", "scheme_id", "country_code", "created_at")
SELECT
  md5('seed-project-fw-country-' || s."id" || '-' || n)::uuid,
  s."id",
  (ARRAY[
    'DE', 'FR', 'US', 'NL', 'SE', 'FI', 'EE', 'UA', 'KZ', 'TR',
    'JP', 'BR', 'CA', 'AU', 'IN', 'ES', 'IT', 'CH', 'AT', 'CZ'
  ])[1 + ((n - 1 + (ascii(substr(md5(s."id"::text), 1, 1)) % 20)) % 20)::int],
  CURRENT_TIMESTAMP - (n % 20) * INTERVAL '1 day'
FROM "scheme" AS s
CROSS JOIN generate_series(1, 3) AS n
WHERE s."id" IN (
  SELECT md5('seed-project-scheme-' || p.id || '-' || gs)::uuid
  FROM "project" AS p
  CROSS JOIN generate_series(1, 3) AS gs
);

INSERT INTO "scheme_firewall_network" ("id", "scheme_id", "network", "created_at")
SELECT
  md5('seed-project-fw-network-' || s."id" || '-' || n)::uuid,
  s."id",
  ('10.' || (200 + ((ascii(substr(md5(s."id"::text), 2, 1)) + n) % 20))
    || '.' || ((ascii(substr(md5(s."id"::text), 3, 1)) + n) % 256)
    || '.0/24')::cidr,
  CURRENT_TIMESTAMP - (n % 20) * INTERVAL '1 day'
FROM "scheme" AS s
CROSS JOIN generate_series(1, 2) AS n
WHERE s."id" IN (
  SELECT md5('seed-project-scheme-' || p.id || '-' || gs)::uuid
  FROM "project" AS p
  CROSS JOIN generate_series(1, 3) AS gs
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM "scheme_firewall_network"
WHERE "id" IN (
  SELECT md5('seed-project-fw-network-' || s."id" || '-' || n)::uuid
  FROM "scheme" AS s
  CROSS JOIN generate_series(1, 2) AS n
);

DELETE FROM "scheme_firewall_country"
WHERE "id" IN (
  SELECT md5('seed-project-fw-country-' || s."id" || '-' || n)::uuid
  FROM "scheme" AS s
  CROSS JOIN generate_series(1, 3) AS n
);

DELETE FROM "scheme"
WHERE "id" IN (
  SELECT md5('seed-project-scheme-' || p.id || '-' || n)::uuid
  FROM "project" AS p
  CROSS JOIN generate_series(1, 3) AS n
);
-- +goose StatementEnd
