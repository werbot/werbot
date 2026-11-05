-- +goose Up
-- +goose StatementBegin
-- Audit trails for the seeded scheme members, with four screen records each.
INSERT INTO "audit" (
  "id", "member_id", "time_start", "time_end", "version", "width", "height",
  "duration", "command", "title", "env_term", "env_shell", "session",
  "client_ip", "created_at"
)
SELECT
  md5('seed-audit-' || n)::uuid,
  md5('seed-scheme-member-' || (1 + ((n - 1) % 24)))::uuid,
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
  'seed-audit-session-' || n,
  ('10.21.' || (n % 250) || '.' || (1 + (n % 250)))::inet,
  CURRENT_TIMESTAMP - (n % 30) * INTERVAL '1 day' - (n % 24) * INTERVAL '1 hour'
FROM generate_series(1, 120) AS n;

INSERT INTO "audit_record" ("id", "audit_id", "duration", "screen", "type")
SELECT
  md5('seed-audit-record-' || n)::uuid,
  md5('seed-audit-' || (1 + ((n - 1) % 120)))::uuid,
  (n % 60)::float8,
  'seed audit frame ' || n,
  (ARRAY['o', 'i', 'r'])[1 + (n % 3)]
FROM generate_series(1, 480) AS n;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM "audit_record"
WHERE "id" IN (
  SELECT md5('seed-audit-record-' || n)::uuid FROM generate_series(1, 480) AS n
);

DELETE FROM "audit"
WHERE "id" IN (
  SELECT md5('seed-audit-' || n)::uuid FROM generate_series(1, 120) AS n
);
-- +goose StatementEnd
