-- +goose Up
-- +goose StatementBegin
-- Development-only fixtures. IDs are deterministic so Down can remove exactly
-- the rows created here, and the rows stay attached to the base dev fixture
-- projects (project1, project7 and the user project).
INSERT INTO "project_ldap" (
  "id", "project_id", "host", "password", "port", "root_dn", "query_dn",
  "object_class", "description", "created_at"
)
SELECT
  md5('seed-project-ldap-' || n)::uuid,
  CASE (n % 3)
    WHEN 1 THEN '2bef1080-cd6e-49e5-8042-1224cf6a3da9'::uuid -- project1
    WHEN 2 THEN '26060c68-5a06-4a57-b87a-be0f1e787157'::uuid -- project7
    ELSE 'd958ee44-a960-420e-9bbf-c7a35084c4aa'::uuid        -- user project
  END,
  'ldap' || n || '.seed.example.internal',
  'seed-ldap-password-' || n,
  389,
  'dc=seed' || n || ',dc=example,dc=internal',
  'cn=svc' || n || ',dc=seed' || n || ',dc=example,dc=internal',
  'inetOrgPerson',
  'Seed LDAP directory #' || n,
  CURRENT_TIMESTAMP - (n % 30) * INTERVAL '1 day' - (n % 24) * INTERVAL '1 hour'
FROM generate_series(1, 24) AS n;

INSERT INTO "project_api" ("id", "project_id", "api_key", "api_secret", "active", "created_at")
SELECT
  md5('seed-project-api-' || n)::uuid,
  CASE (n % 4)
    WHEN 1 THEN '2bef1080-cd6e-49e5-8042-1224cf6a3da9'::uuid -- project1
    WHEN 2 THEN '26060c68-5a06-4a57-b87a-be0f1e787157'::uuid -- project7
    WHEN 3 THEN 'd958ee44-a960-420e-9bbf-c7a35084c4aa'::uuid -- user project
    ELSE 'fe52ca9b-5599-4bb6-818b-1896d56e9aa2'::uuid        -- project2
  END,
  md5('seed-api-key-' || n) || substr(md5('seed-api-secret-' || n), 1, 5),
  md5('seed-api-secret-' || n) || substr(md5('seed-api-key-' || n), 1, 5),
  (n % 5 <> 0),
  CURRENT_TIMESTAMP - (n % 30) * INTERVAL '1 day' - (n % 24) * INTERVAL '1 hour'
FROM generate_series(1, 24) AS n;

INSERT INTO "profile_public_key" (
  "id", "profile_id", "title", "key", "fingerprint",
  "locked_at", "archived_at", "created_at"
)
SELECT
  md5('seed-public-key-' || n)::uuid,
  CASE (n % 4)
    WHEN 0 THEN '008feb1d-12f2-4bc3-97ff-c8d7fb9f7686'::uuid -- admin
    WHEN 1 THEN 'c180ad5c-0c65-4cee-8725-12931cb5abb3'::uuid -- user
    WHEN 2 THEN 'b3dc36e2-7f84-414b-b147-7ac850369518'::uuid -- user1
    ELSE 'b8c3d9e4-6f4d-4e11-a3e6-72f9cb7660e0'::uuid        -- user2
  END,
  'seed public key ' || n,
  'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI'
    || substr(md5('seed-public-key-' || n), 1, 32)
    || ' seed' || n || '@werbot.net',
  rtrim(regexp_replace(md5('seed-public-key-fp-' || n), '(.{2})', '\1:', 'g'), ':'),
  CASE WHEN n % 7 = 0 THEN CURRENT_TIMESTAMP - INTERVAL '2 hour' END,
  CASE WHEN n % 11 = 0 THEN CURRENT_TIMESTAMP - INTERVAL '1 day' END,
  CURRENT_TIMESTAMP - (n % 30) * INTERVAL '1 day' - (n % 24) * INTERVAL '1 hour'
FROM generate_series(1, 24) AS n;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM "profile_public_key"
WHERE "id" IN (
  SELECT md5('seed-public-key-' || n)::uuid FROM generate_series(1, 24) AS n
);

DELETE FROM "project_api"
WHERE "id" IN (
  SELECT md5('seed-project-api-' || n)::uuid FROM generate_series(1, 24) AS n
);

DELETE FROM "project_ldap"
WHERE "id" IN (
  SELECT md5('seed-project-ldap-' || n)::uuid FROM generate_series(1, 24) AS n
);
-- +goose StatementEnd
