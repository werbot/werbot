-- +goose Up
-- +goose StatementBegin
-- Tokens across every section (profile, project, scheme, agent) and every
-- status (done, sent, used, deleted, archived).
INSERT INTO "token" (
  "id", "section", "action", "status", "profile_id", "project_id",
  "scheme_id", "data", "expired_at", "updated_at", "created_at"
)
SELECT
  md5('seed-token-' || t.n)::uuid,
  t.sec,
  t.act,
  t.st,
  CASE
    WHEN t.sec = 1 THEN (ARRAY[
      '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686'::uuid, -- admin
      'c180ad5c-0c65-4cee-8725-12931cb5abb3'::uuid, -- user
      'b3dc36e2-7f84-414b-b147-7ac850369518'::uuid, -- user1
      'b8c3d9e4-6f4d-4e11-a3e6-72f9cb7660e0'::uuid, -- user2
      '68bf07a3-0132-4709-920b-5054f9eaa89a'::uuid  -- user3
    ])[1 + (t.n % 5)]
  END,
  CASE
    WHEN t.sec = 2 THEN (ARRAY[
      '2bef1080-cd6e-49e5-8042-1224cf6a3da9'::uuid, -- project1
      '26060c68-5a06-4a57-b87a-be0f1e787157'::uuid, -- project7
      'd958ee44-a960-420e-9bbf-c7a35084c4aa'::uuid, -- user project
      'fe52ca9b-5599-4bb6-818b-1896d56e9aa2'::uuid  -- project2
    ])[1 + (t.n % 4)]
    WHEN t.sec = 3 THEN
      CASE WHEN t.k <= 8
        THEN '2bef1080-cd6e-49e5-8042-1224cf6a3da9'::uuid
        ELSE 'd958ee44-a960-420e-9bbf-c7a35084c4aa'::uuid
      END
    WHEN t.sec = 4 THEN
      CASE WHEN t.n % 2 = 1
        THEN '2bef1080-cd6e-49e5-8042-1224cf6a3da9'::uuid
        ELSE 'd958ee44-a960-420e-9bbf-c7a35084c4aa'::uuid
      END
  END,
  CASE
    WHEN t.sec = 3 THEN md5('seed-scheme-' || t.k)::uuid
  END,
  CASE
    WHEN t.sec = 3 THEN jsonb_build_object(
      'email', 'seed-token-' || t.n || '@werbot.net',
      'scheme_type', 103
    )
    ELSE jsonb_build_object(
      'name', 'seed',
      'surname', 'token' || t.n,
      'email', 'seed-token-' || t.n || '@werbot.net'
    )
  END,
  CASE
    WHEN t.st = 2 THEN CURRENT_TIMESTAMP + ((t.n % 30) + 1) * INTERVAL '1 day'
    ELSE CURRENT_TIMESTAMP - ((t.n % 20) + 1) * INTERVAL '1 day'
  END,
  CASE
    WHEN t.st = 1 THEN CURRENT_TIMESTAMP - ((t.n % 7) + 1) * INTERVAL '1 hour'
  END,
  CURRENT_TIMESTAMP - (t.n % 30) * INTERVAL '1 day' - (t.n % 24) * INTERVAL '1 hour'
FROM (
  SELECT
    n,
    1 + ((n - 1) % 4) AS sec,
    1 + ((n - 1) % 8) AS act,
    1 + ((n - 1) % 5) AS st,
    1 + ((n - 1) % 12) AS k
  FROM generate_series(1, 72) AS n
) AS t;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM "token"
WHERE "id" IN (
  SELECT md5('seed-token-' || n)::uuid FROM generate_series(1, 72) AS n
);
-- +goose StatementEnd
