-- +goose Up
-- +goose StatementBegin
-- Events for project1, the seeded schemes and the base fixture profiles.
-- Timestamps are spread over the last 30 days so charts have history.
INSERT INTO "event" (
  "id", "related_id", "prime_section", "section", "type", "session", "data", "created_at"
)
SELECT
  md5('seed-event-project-' || n)::uuid,
  '2bef1080-cd6e-49e5-8042-1224cf6a3da9'::uuid,
  2,
  1,
  1 + (n % 8),
  jsonb_build_object(
    'id', md5('seed-event-session-project-' || n),
    'user_agent', (ARRAY[
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15',
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
      'Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0',
      'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1'
    ])[1 + (n % 4)],
    'ip', (ARRAY[
      '192.168.10.' || (1 + (n % 250)),
      '10.16.' || (n % 250) || '.' || (1 + (n % 250)),
      '172.16.' || (n % 250) || '.' || (1 + (n % 250)),
      '2001:db8:' || to_hex(n % 65535) || '::1'
    ])[1 + (n % 4)]
  ),
  '{}'::jsonb,
  CURRENT_TIMESTAMP - (n % 30) * INTERVAL '1 day' - (n % 24) * INTERVAL '1 hour'
FROM generate_series(1, 500) AS n;

INSERT INTO "event" (
  "id", "related_id", "prime_section", "section", "type", "session", "data", "created_at"
)
SELECT
  md5('seed-event-scheme-' || n)::uuid,
  md5('seed-scheme-' || (1 + ((n - 1) % 12)))::uuid,
  3,
  1,
  1 + (n % 8),
  jsonb_build_object(
    'id', md5('seed-event-session-scheme-' || n),
    'user_agent', (ARRAY[
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15',
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
      'Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0'
    ])[1 + (n % 3)],
    'ip', '10.17.' || (n % 250) || '.' || (1 + (n % 250))
  ),
  '{}'::jsonb,
  CURRENT_TIMESTAMP - (n % 30) * INTERVAL '1 day' - (n % 24) * INTERVAL '1 hour'
FROM generate_series(1, 200) AS n;

INSERT INTO "event" (
  "id", "related_id", "prime_section", "section", "type", "session", "data", "created_at"
)
SELECT
  md5('seed-event-profile-' || n)::uuid,
  (CASE (n % 6)
    WHEN 0 THEN '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686'::uuid
    WHEN 1 THEN 'c180ad5c-0c65-4cee-8725-12931cb5abb3'::uuid
    WHEN 2 THEN 'b3dc36e2-7f84-414b-b147-7ac850369518'::uuid
    WHEN 3 THEN 'b8c3d9e4-6f4d-4e11-a3e6-72f9cb7660e0'::uuid
    WHEN 4 THEN '68bf07a3-0132-4709-920b-5054f9eaa89a'::uuid
    ELSE '395b237f-037e-49d4-b409-fd2f514242f6'::uuid
  END),
  1,
  1,
  CASE WHEN n % 2 = 0 THEN 9 ELSE 10 END,
  jsonb_build_object(
    'id', md5('seed-event-session-profile-' || n),
    'user_agent', (ARRAY[
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15',
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36'
    ])[1 + (n % 2)],
    'ip', '192.168.20.' || (1 + (n % 250))
  ),
  '{}'::jsonb,
  CURRENT_TIMESTAMP - (n % 30) * INTERVAL '1 day' - (n % 24) * INTERVAL '1 hour'
FROM generate_series(1, 100) AS n;

-- Shell sessions for the seeded scheme members.
INSERT INTO "session" ("id", "member_id", "status", "message", "created_at")
SELECT
  md5('seed-session-' || n)::uuid,
  md5('seed-scheme-member-' || (1 + ((n - 1) % 24)))::uuid,
  (n % 3 <> 0),
  'Seed shell session #' || n || ' for development',
  CURRENT_TIMESTAMP - (n % 30) * INTERVAL '1 day' - (n % 24) * INTERVAL '1 hour'
FROM generate_series(1, 120) AS n;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM "session"
WHERE "id" IN (
  SELECT md5('seed-session-' || n)::uuid FROM generate_series(1, 120) AS n
);

DELETE FROM "event"
WHERE "id" IN (
  SELECT md5('seed-event-project-' || n)::uuid FROM generate_series(1, 500) AS n
);

DELETE FROM "event"
WHERE "id" IN (
  SELECT md5('seed-event-scheme-' || n)::uuid FROM generate_series(1, 200) AS n
);

DELETE FROM "event"
WHERE "id" IN (
  SELECT md5('seed-event-profile-' || n)::uuid FROM generate_series(1, 100) AS n
);
-- +goose StatementEnd
