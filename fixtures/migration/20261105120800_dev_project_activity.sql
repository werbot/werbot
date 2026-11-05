-- +goose Up
-- +goose StatementBegin
-- Project event feeds for every project, not only project1.
INSERT INTO "event" (
  "id", "related_id", "prime_section", "section", "type", "session", "data", "created_at"
)
SELECT
  md5('seed-project-event-' || p.id || '-' || n)::uuid,
  p.id,
  2,
  1,
  1 + (n % 8),
  jsonb_build_object(
    'id', md5('seed-project-event-session-' || p.id || '-' || n)::uuid,
    'user_agent', (ARRAY[
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15',
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
      'Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0',
      'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1'
    ])[1 + (n % 4)],
    'ip', (ARRAY[
      '192.168.30.' || (1 + (n % 250)),
      '10.60.' || (n % 250) || '.' || (1 + (n % 250)),
      '172.20.' || (n % 250) || '.' || (1 + (n % 250)),
      '2001:db8:' || to_hex(n % 65535) || '::2'
    ])[1 + (n % 4)]
  ),
  '{}'::jsonb,
  CURRENT_TIMESTAMP - (n % 30) * INTERVAL '1 day' - (n % 24) * INTERVAL '1 hour'
FROM "project" AS p
CROSS JOIN generate_series(1, 24) AS n;

-- Scheme event feeds for every server, including base and seeded schemes.
INSERT INTO "event" (
  "id", "related_id", "prime_section", "section", "type", "session", "data", "created_at"
)
SELECT
  md5('seed-scheme-event-' || s.id || '-' || n)::uuid,
  s.id,
  3,
  1,
  1 + (n % 8),
  jsonb_build_object(
    'id', md5('seed-scheme-event-session-' || s.id || '-' || n)::uuid,
    'user_agent', (ARRAY[
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15',
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
      'Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0'
    ])[1 + (n % 3)],
    'ip', '10.70.' || (n % 250) || '.' || (1 + (n % 250))
  ),
  '{}'::jsonb,
  CURRENT_TIMESTAMP - (n % 30) * INTERVAL '1 day' - (n % 24) * INTERVAL '1 hour'
FROM "scheme" AS s
CROSS JOIN generate_series(1, 10) AS n;

-- Shell sessions for every scheme member.
INSERT INTO "session" ("id", "member_id", "status", "message", "created_at")
SELECT
  md5('seed-scheme-session-' || sm.id || '-' || n)::uuid,
  sm.id,
  (n % 3 <> 0),
  'Seed shell session #' || n || ' for scheme ' || sm."scheme_id",
  CURRENT_TIMESTAMP - (n % 30) * INTERVAL '1 day' - (n % 24) * INTERVAL '1 hour'
FROM "scheme_member" AS sm
CROSS JOIN generate_series(1, 3) AS n;

-- Audit trails for every scheme member, with three screen records per audit.
INSERT INTO "audit" (
  "id", "member_id", "time_start", "time_end", "version", "width", "height",
  "duration", "command", "title", "env_term", "env_shell", "session",
  "client_ip", "created_at"
)
SELECT
  md5('seed-scheme-audit-' || sm.id || '-' || n)::uuid,
  sm.id,
  CURRENT_TIMESTAMP - (n % 30) * INTERVAL '1 day' - (n % 24) * INTERVAL '1 hour',
  CURRENT_TIMESTAMP - (n % 30) * INTERVAL '1 day' - (n % 24) * INTERVAL '1 hour'
    + (60 + (n % 600)) * INTERVAL '1 second',
  1 + (n % 3),
  1920,
  1080,
  (60 + (n % 600))::float8,
  '/bin/bash',
  'Seed audit session #' || n,
  'xterm-256color',
  '/bin/bash',
  'seed-audit-session-' || substr(md5(sm.id::text), 1, 8) || '-' || n,
  ('10.80.' || (n % 250) || '.' || (1 + (n % 250)))::inet,
  CURRENT_TIMESTAMP - (n % 30) * INTERVAL '1 day' - (n % 24) * INTERVAL '1 hour'
FROM "scheme_member" AS sm
CROSS JOIN generate_series(1, 2) AS n;

INSERT INTO "audit_record" ("id", "audit_id", "duration", "screen", "type")
SELECT
  md5('seed-scheme-audit-record-' || a.id || '-' || n)::uuid,
  a.id,
  (n * 10)::float8,
  'seed audit frame ' || n,
  (ARRAY['o', 'i', 'r'])[1 + (n % 3)]
FROM "audit" AS a
CROSS JOIN generate_series(1, 3) AS n
WHERE a.id IN (
  SELECT md5('seed-scheme-audit-' || sm.id || '-' || k)::uuid
  FROM "scheme_member" AS sm
  CROSS JOIN generate_series(1, 2) AS k
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM "audit"
WHERE "id" IN (
  SELECT md5('seed-scheme-audit-' || sm."id" || '-' || n)::uuid
  FROM "scheme_member" AS sm
  CROSS JOIN generate_series(1, 2) AS n
);

DELETE FROM "session"
WHERE "id" IN (
  SELECT md5('seed-scheme-session-' || sm."id" || '-' || n)::uuid
  FROM "scheme_member" AS sm
  CROSS JOIN generate_series(1, 3) AS n
);

DELETE FROM "event"
WHERE "id" IN (
  SELECT md5('seed-project-event-' || p."id" || '-' || n)::uuid
  FROM "project" AS p
  CROSS JOIN generate_series(1, 24) AS n
  UNION
  SELECT md5('seed-scheme-event-' || s."id" || '-' || n)::uuid
  FROM "scheme" AS s
  CROSS JOIN generate_series(1, 10) AS n
);
-- +goose StatementEnd
