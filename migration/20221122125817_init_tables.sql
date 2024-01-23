-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE "public"."user" (
    "id" uuid DEFAULT gen_random_uuid (),
    "login" varchar(255) NOT NULL,
    "name" varchar(255) NOT NULL,
    "surname" varchar(255) NOT NULL,
    "email" varchar(255) NOT NULL,
    "password" varchar(255) NOT NULL,
    "enabled" bool NOT NULL DEFAULT true,
    "confirmed" bool NOT NULL DEFAULT false,
    "role" int4 NOT NULL DEFAULT 1,
    "locked_at" timestamp DEFAULT NULL,
    "archived_at" timestamp DEFAULT NULL,
    "updated_at" timestamp DEFAULT now(),
    "created_at" timestamp DEFAULT now(),
    PRIMARY KEY ("id")
);
CREATE UNIQUE INDEX "user_on_login" ON "public"."user" USING BTREE ("login");
CREATE UNIQUE INDEX "user_on_email" ON "public"."user" USING BTREE ("email");
CREATE UNIQUE INDEX "user_uniqueness" ON "public"."user" USING BTREE ("id","login","role","enabled","confirmed");

CREATE TABLE "public"."project" (
    "id" uuid DEFAULT gen_random_uuid (),
    "owner_id" uuid NOT NULL,
    "title" varchar(255) NOT NULL,
    "login" varchar(255) NOT NULL,
    "locked_at" timestamp DEFAULT NULL,
    "archived_at" timestamp DEFAULT NULL,
    "updated_at" timestamp DEFAULT now(),
    "created_at" timestamp DEFAULT now(),
    PRIMARY KEY ("id"),
    FOREIGN KEY ("owner_id") REFERENCES "public"."user"("id") ON DELETE CASCADE
);
CREATE UNIQUE INDEX "project_uniqueness" ON "public"."project" USING BTREE ("id","owner_id","login");

CREATE TABLE "public"."project_invite" (
    "id" uuid DEFAULT gen_random_uuid (),
    "project_id" uuid NOT NULL,
    "user_id" uuid,
    "invite" uuid NOT NULL,
    "name" varchar(255) DEFAULT NULL,
    "surname" varchar(255) DEFAULT NULL,
    "email" varchar(255) NOT NULL,
    "status" varchar(255) NOT NULL,
    "ldap_user" bool,
    "ldap_name" varchar(255) DEFAULT NULL,
    "updated_at" timestamp DEFAULT now(),
    "created_at" timestamp DEFAULT now(),
    PRIMARY KEY ("id"),
    FOREIGN KEY ("user_id") REFERENCES "public"."user"("id") ON DELETE SET NULL,
    FOREIGN KEY ("project_id") REFERENCES "public"."project"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."project_ldap" (
    "id" uuid DEFAULT gen_random_uuid (),
    "project_id" uuid NOT NULL,
    "host" varchar(255) DEFAULT NULL,
    "password" varchar(255) DEFAULT NULL,
    "port" int4,
    "root_dn" varchar(255) DEFAULT NULL,
    "query_dn" varchar(255) DEFAULT NULL,
    "object_class" varchar(255) DEFAULT NULL,
    "description" varchar(255) DEFAULT NULL,
    "updated_at" timestamp DEFAULT now(),
    "created_at" timestamp DEFAULT now(),
    PRIMARY KEY ("id"),
    FOREIGN KEY ("project_id") REFERENCES "public"."project"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."project_member" (
    "id" uuid DEFAULT gen_random_uuid (),
    "project_id" uuid NOT NULL,
    "user_id" uuid NOT NULL,
    "active" bool NOT NULL DEFAULT false,
    "online" bool NOT NULL DEFAULT false,
    "role" varchar(255) NOT NULL,
    "locked_at" timestamp DEFAULT NULL,
    "archived_at" timestamp DEFAULT NULL,
    "updated_at" timestamp DEFAULT now(),
    "created_at" timestamp DEFAULT now(),
    PRIMARY KEY ("id"),
    FOREIGN KEY ("project_id") REFERENCES "public"."project"("id") ON DELETE CASCADE,
    FOREIGN KEY ("user_id") REFERENCES "public"."user"("id") ON DELETE CASCADE
);
CREATE UNIQUE INDEX "project_member_on_login" ON "public"."project_member" USING BTREE ("id", "active", "online");
CREATE UNIQUE INDEX "project_member_uniqueness" ON "public"."project_member" USING BTREE ("id","project_id","user_id");

CREATE TABLE "public"."server_access" (
    "id" uuid DEFAULT gen_random_uuid (),
    "server_id" uuid NOT NULL,
    "auth" varchar(1) NOT NULL,
    "login" varchar(255) DEFAULT NULL,
    "password" varchar(255) DEFAULT NULL,
    "key" jsonb NOT NULL DEFAULT '{}'::jsonb,
    "updated_at" timestamp DEFAULT now(),
    "created_at" timestamp DEFAULT now(),
    PRIMARY KEY ("id")
);

CREATE TABLE "public"."server" (
    "id" uuid DEFAULT gen_random_uuid (),
    "project_id" uuid NOT NULL,
    "access_id" uuid,
    "address" varchar(255) NOT NULL,
    "port" int4 NOT NULL,
    "token" varchar(255) NOT NULL,
    "title" varchar(255) NOT NULL,
    "description" text NOT NULL,
    "active" bool NOT NULL DEFAULT false,
    "audit" bool NOT NULL DEFAULT false,
    "online" bool NOT NULL DEFAULT false,
    "scheme" varchar(1) NOT NULL,
    "previous_state" jsonb NOT NULL DEFAULT '{}'::jsonb,
    "locked_at" timestamp DEFAULT NULL,
    "archived_at" timestamp DEFAULT NULL,
    "updated_at" timestamp DEFAULT now(),
    "created_at" timestamp DEFAULT now(),
    PRIMARY KEY ("id"),
    FOREIGN KEY ("project_id") REFERENCES "public"."project"("id") ON DELETE CASCADE,
    FOREIGN KEY ("access_id") REFERENCES "public"."server_access"("id") ON DELETE CASCADE
);
CREATE UNIQUE INDEX "server_uniqueness" ON "public"."server" USING BTREE ("id","project_id","access_id");

CREATE TABLE "public"."server_member" (
    "id" uuid DEFAULT gen_random_uuid (),
    "server_id" uuid NOT NULL,
    "member_id" uuid NOT NULL,
    "active" bool NOT NULL DEFAULT false,
    "online" bool NOT NULL DEFAULT false,
    "previous_state" jsonb NOT NULL DEFAULT '{}'::jsonb,
    "locked_at" timestamp DEFAULT NULL,
    "archived_at" timestamp DEFAULT NULL,
    "updated_at" timestamp DEFAULT now(),
    "created_at" timestamp DEFAULT now(),
    PRIMARY KEY ("id"),
    FOREIGN KEY ("member_id") REFERENCES "public"."project_member"("id") ON DELETE CASCADE,
    FOREIGN KEY ("server_id") REFERENCES "public"."server"("id") ON DELETE CASCADE
);
CREATE UNIQUE INDEX "server_member_uniqueness" ON "public"."server_member" USING BTREE ("id","member_id","server_id");

CREATE TABLE "public"."server_access_policy" (
    "server_id" uuid NOT NULL,
    "ip" bool NOT NULL,
    "country" bool NOT NULL,
    "updated_at" timestamp DEFAULT now(),
    PRIMARY KEY ("server_id"),
    FOREIGN KEY ("server_id") REFERENCES "public"."server"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."server_access_token" (
    "id" uuid DEFAULT gen_random_uuid (),
    "account_id" uuid NOT NULL,
    "expired" timestamp NOT NULL,
    "updated_at" timestamp DEFAULT now(),
    PRIMARY KEY ("id"),
    FOREIGN KEY ("account_id") REFERENCES "public"."server_member"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."server_activity" (
    "id" uuid DEFAULT gen_random_uuid (),
    "server_id" uuid NOT NULL,
    "dow" int4 NOT NULL,
    "time_from" time NOT NULL,
    "time_to" time NOT NULL,
    PRIMARY KEY ("id"),
    FOREIGN KEY ("server_id") REFERENCES "public"."server"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."country" (
    "code" varchar(4) NOT NULL,
    "name" varchar(255) NOT NULL,
    PRIMARY KEY ("code")
);

CREATE TABLE "public"."server_security_country" (
    "id" uuid DEFAULT gen_random_uuid (),
    "server_id" uuid NOT NULL,
    "country_code" varchar(4) DEFAULT NULL,
    PRIMARY KEY ("id"),
    FOREIGN KEY ("country_code") REFERENCES "public"."country"("code") ON DELETE CASCADE,
    FOREIGN KEY ("server_id") REFERENCES "public"."server"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."event_profile" (
    "id" uuid DEFAULT gen_random_uuid (),
    "profile_id" uuid NOT NULL,
    "user_id" uuid NOT NULL,
    "user_agent" varchar(255) NOT NULL DEFAULT '',
    "ip" inet,
    "event" int2 NOT NULL,
    "data" jsonb NOT NULL DEFAULT '{}'::jsonb,
    "created_at" timestamp DEFAULT now(),
    PRIMARY KEY ("id"),
    FOREIGN KEY ("profile_id") REFERENCES "public"."user"("id") ON DELETE CASCADE,
    FOREIGN KEY ("user_id") REFERENCES "public"."user"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."event_project" (
    "id" uuid DEFAULT gen_random_uuid (),
    "project_id" uuid NOT NULL,
    "user_id" uuid NOT NULL,
    "user_agent" varchar(255) NOT NULL DEFAULT '',
    "ip" inet,
    "event" int2 NOT NULL,
    "data" jsonb NOT NULL DEFAULT '{}'::jsonb,
    "created_at" timestamp DEFAULT now(),
    PRIMARY KEY ("id"),
    FOREIGN KEY ("project_id") REFERENCES "public"."project"("id") ON DELETE CASCADE,
    FOREIGN KEY ("user_id") REFERENCES "public"."user"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."event_server" (
    "id" uuid DEFAULT gen_random_uuid (),
    "server_id" uuid NOT NULL,
    "user_id" uuid NOT NULL,
    "user_agent" varchar(255) NOT NULL DEFAULT '',
    "ip" inet,
    "event" int2 NOT NULL,
    "data" jsonb NOT NULL DEFAULT '{}'::jsonb,
    "created_at" timestamp DEFAULT now(),
    PRIMARY KEY ("id"),
    FOREIGN KEY ("server_id") REFERENCES "public"."server"("id") ON DELETE CASCADE,
    FOREIGN KEY ("user_id") REFERENCES "public"."user"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."server_security_ip" (
    "id" uuid DEFAULT gen_random_uuid (),
    "server_id" uuid NOT NULL,
    "start_ip" inet,
    "end_ip" inet,
    PRIMARY KEY ("id"),
    FOREIGN KEY ("server_id") REFERENCES "public"."server"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."session" (
    "id" uuid DEFAULT gen_random_uuid (),
    "member_id" uuid NOT NULL,
    "status" varchar(255) NOT NULL CHECK ((status)::text = ANY ((ARRAY['unknown', 'opened', 'closed'])::text[])),
    "message" varchar(1024) NOT NULL,
    "created_at" timestamp DEFAULT now(),
    PRIMARY KEY ("id"),
    FOREIGN KEY ("member_id") REFERENCES "public"."server_member"("id") ON DELETE CASCADE
);
COMMENT ON COLUMN "public"."session"."status" IS '(DC2Type:SessionStatusType)';

CREATE TABLE "public"."user_public_key" (
    "id" uuid DEFAULT gen_random_uuid (),
    "user_id" uuid NOT NULL,
    "title" varchar(255) NOT NULL,
    "key_" text NOT NULL,
    "fingerprint" varchar(255) NOT NULL,
    "locked_at" timestamp DEFAULT NULL,
    "archived_at" timestamp DEFAULT NULL,
    "updated_at" timestamp DEFAULT now(),
    "created_at" timestamp DEFAULT now(),
    PRIMARY KEY ("id"),
    FOREIGN KEY ("user_id") REFERENCES "public"."user"("id") ON DELETE CASCADE
);
CREATE UNIQUE INDEX "user_public_key_uniqueness" ON "public"."user_public_key" USING BTREE ("id","user_id","key_");

CREATE TABLE "public"."user_token" (
    "token" varchar(255) NOT NULL,
    "user_id" uuid NOT NULL,
    "date_used" timestamp DEFAULT NULL,
    "action" varchar(255) NOT NULL CHECK ((action)::text = ANY ((ARRAY['reset', 'delete'])::text[])),
    "used" bool NOT NULL DEFAULT false,
    "updated_at" timestamp DEFAULT now(),
    "created_at" timestamp DEFAULT now(),
    PRIMARY KEY ("token","user_id"),
    FOREIGN KEY ("user_id") REFERENCES "public"."user"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."audit" (
    "id" uuid DEFAULT gen_random_uuid (),
    "account_id" uuid NOT NULL,
    "time_start" timestamp NOT NULL,
    "time_end" timestamp DEFAULT NULL,
    "version" int2 NOT NULL,
    "width" int2 NOT NULL,
    "height" int2 NOT NULL,
    "duration" float8 NOT NULL,
    "command" varchar(255) NOT NULL,
    "title" varchar(255) NOT NULL,
    "env_term" varchar(255) NOT NULL,
    "env_shell" varchar(255) NOT NULL,
    "session" varchar(255) NOT NULL,
    "client_ip" varchar(255) NOT NULL,
    "created_at" timestamp DEFAULT now(),
    PRIMARY KEY ("id"),
    FOREIGN KEY ("account_id") REFERENCES "public"."server_member"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."audit_record" (
    "id" uuid DEFAULT gen_random_uuid (),
    "audit_id" uuid NOT NULL,
    "duration" float8 NOT NULL,
    "screen" text NOT NULL,
    "type" varchar(1) NOT NULL,
    PRIMARY KEY ("id"),
    FOREIGN KEY ("audit_id") REFERENCES "public"."audit"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."limit_user_count" (
    "type" varchar(255) NOT NULL,
    "entity_id" uuid NOT NULL,
    "user_id" uuid NOT NULL,
    "count" int4 NOT NULL DEFAULT 0,
    PRIMARY KEY ("type","entity_id","user_id"),
    FOREIGN KEY ("user_id") REFERENCES "public"."user"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."server_host_key" (
    "server_id" uuid NOT NULL,
    "host_key" bytea,
    "updated_at" timestamp DEFAULT now(),
    FOREIGN KEY ("server_id") REFERENCES "public"."server" ("id") ON DELETE CASCADE ON UPDATE NO ACTION
);

CREATE TABLE "public"."project_api" (
    "id" uuid DEFAULT gen_random_uuid (),
    "project_id" uuid NOT NULL,
    "api_key" varchar(37) COLLATE "pg_catalog"."default" NOT NULL,
    "api_secret" varchar(37) COLLATE "pg_catalog"."default" NOT NULL,
    "online" bool NOT NULL,
    "locked_at" timestamp DEFAULT NULL,
    "archived_at" timestamp DEFAULT NULL,
    "updated_at" timestamp DEFAULT now(),
    "created_at" timestamp DEFAULT now(),
    FOREIGN KEY ("project_id") REFERENCES "public"."project" ("id") ON DELETE CASCADE ON UPDATE NO ACTION,
    PRIMARY KEY ("id")
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP EXTENSION IF EXISTS "pgcrypto";
DROP TABLE IF EXISTS "public"."server_host_key";
DROP TABLE IF EXISTS "public"."project_api";
DROP TABLE IF EXISTS "public"."limit_user_count";
DROP TABLE IF EXISTS "public"."audit_record";
DROP TABLE IF EXISTS "public"."audit";
DROP TABLE IF EXISTS "public"."user_token";
DROP TABLE IF EXISTS "public"."user_public_key";
DROP TABLE IF EXISTS "public"."session";
DROP TABLE IF EXISTS "public"."server_security_ip";
DROP TABLE IF EXISTS "public"."event_profile";
DROP TABLE IF EXISTS "public"."event_server";
DROP TABLE IF EXISTS "public"."event_project";
DROP TABLE IF EXISTS "public"."server_security_country";
DROP TABLE IF EXISTS "public"."country";
DROP TABLE IF EXISTS "public"."server_activity";
DROP TABLE IF EXISTS "public"."server_access_policy";
DROP TABLE IF EXISTS "public"."server_access_token";
DROP TABLE IF EXISTS "public"."server_member";
DROP TABLE IF EXISTS "public"."server";
DROP TABLE IF EXISTS "public"."server_access";
DROP TABLE IF EXISTS "public"."project_member";
DROP TABLE IF EXISTS "public"."project_ldap";
DROP TABLE IF EXISTS "public"."project_invite";
DROP TABLE IF EXISTS "public"."project";
DROP TABLE IF EXISTS "public"."user";
-- +goose StatementEnd
