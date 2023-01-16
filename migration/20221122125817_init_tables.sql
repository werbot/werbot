-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE "public"."user" (
    "id" uuid DEFAULT gen_random_uuid (),
    "fio" varchar(255) NOT NULL,
    "name" varchar(255) NOT NULL,
    "email" varchar(255) NOT NULL,
    "password" varchar(255) NOT NULL,
    "enabled" bool NOT NULL DEFAULT true,
    "confirmed" bool NOT NULL DEFAULT false,
    "last_active" timestamp(0) DEFAULT NULL::timestamp without time zone,
    "register_date" timestamp(0) DEFAULT NULL::timestamp without time zone,
    "role" int4 NOT NULL DEFAULT 1,
    PRIMARY KEY ("id")
);

CREATE TABLE "public"."project" (
    "id" uuid DEFAULT gen_random_uuid (),
    "owner_id" uuid,
    "title" varchar(255) NOT NULL,
    "login" varchar(255) NOT NULL,
    "created" timestamp(0) NOT NULL,
    PRIMARY KEY ("id"),
    FOREIGN KEY ("owner_id") REFERENCES "public"."user"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."project_invite" (
    "id" uuid DEFAULT gen_random_uuid (),
    "project_id" uuid,
    "user_id" uuid,
    "invite" uuid,
    "name" varchar(255) DEFAULT NULL::character varying,
    "surname" varchar(255) DEFAULT NULL::character varying,
    "email" varchar(255) NOT NULL,
    "created" timestamp(0) NOT NULL,
    "status" varchar(255) NOT NULL,
    "ldap_user" bool,
    "ldap_name" varchar(255) DEFAULT NULL::character varying,
    PRIMARY KEY ("id"),
    FOREIGN KEY ("user_id") REFERENCES "public"."user"("id") ON DELETE SET NULL,
    FOREIGN KEY ("project_id") REFERENCES "public"."project"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."project_ldap" (
    "id" uuid DEFAULT gen_random_uuid (),
    "project_id" uuid,
    "host" varchar(255) DEFAULT NULL::character varying,
    "password" varchar(255) DEFAULT NULL::character varying,
    "port" int4,
    "root_dn" varchar(255) DEFAULT NULL::character varying,
    "query_dn" varchar(255) DEFAULT NULL::character varying,
    "object_class" varchar(255) DEFAULT NULL::character varying,
    "description" varchar(255) DEFAULT NULL::character varying,
    PRIMARY KEY ("id"),
    FOREIGN KEY ("project_id") REFERENCES "public"."project"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."project_member" (
    "id" uuid DEFAULT gen_random_uuid (),
    "project_id" uuid,
    "user_id" uuid,
    "active" bool NOT NULL DEFAULT false,
    "online" bool NOT NULL DEFAULT false,
    "role" varchar(255) NOT NULL,
    "created" timestamp(0) NOT NULL,
    PRIMARY KEY ("id"),
    FOREIGN KEY ("project_id") REFERENCES "public"."project"("id") ON DELETE CASCADE,
    FOREIGN KEY ("user_id") REFERENCES "public"."user"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."server" (
    "id" uuid DEFAULT gen_random_uuid (),
    "project_id" uuid,
    "address" varchar(255) NOT NULL,
    "port" int4 NOT NULL,
    "token" varchar(255) NOT NULL,
    "login" varchar(255) NOT NULL,
    "password" varchar(255) DEFAULT NULL::character varying,
    "private_description" text NOT NULL,
    "public_description" text NOT NULL,
    "title" varchar(255) NOT NULL,
    "active" bool NOT NULL DEFAULT false,
    "audit" bool NOT NULL DEFAULT false,
    "online" bool NOT NULL DEFAULT false,
    "public_key" text NOT NULL,
    "private_key" text NOT NULL,
    "private_key_password" varchar(255),
    "created" timestamp(0) NOT NULL,
    "auth" varchar(255) NOT NULL CHECK ((auth)::text = ANY ((ARRAY['key'::character varying, 'password'::character varying, 'agent'::character varying])::text[])),
    "scheme" varchar(255) NOT NULL CHECK ((scheme)::text = ANY ((ARRAY['telnet'::character varying, 'ssh'::character varying])::text[])),
    "previous_state" json NOT NULL DEFAULT '{}'::json,
    PRIMARY KEY ("id"),
    FOREIGN KEY ("project_id") REFERENCES "public"."project"("id") ON DELETE CASCADE
);
COMMENT ON COLUMN "public"."server"."auth" IS '(DC2Type:AuthType)';
COMMENT ON COLUMN "public"."server"."scheme" IS '(DC2Type:ProtocolSchemeType)';


CREATE TABLE "public"."server_member" (
    "id" uuid DEFAULT gen_random_uuid (),
    "server_id" uuid,
    "member_id" uuid,
    "active" bool NOT NULL DEFAULT false,
    "online" bool NOT NULL DEFAULT false,
    "last_activity" timestamp(0) DEFAULT NULL::timestamp without time zone,
    "previous_state" json NOT NULL DEFAULT '{}'::json,
    PRIMARY KEY ("id"),
    FOREIGN KEY ("member_id") REFERENCES "public"."project_member"("id") ON DELETE CASCADE,
    FOREIGN KEY ("server_id") REFERENCES "public"."server"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."server_access_policy" (
    "server_id" uuid NOT NULL,
    "ip" bool NOT NULL,
    "country" bool NOT NULL,
    PRIMARY KEY ("server_id"),
    FOREIGN KEY ("server_id") REFERENCES "public"."server"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."server_access_token" (
    "id" uuid DEFAULT gen_random_uuid (),
    "account_id" uuid,
    "expired" timestamp(0) NOT NULL,
    PRIMARY KEY ("id"),
    FOREIGN KEY ("account_id") REFERENCES "public"."server_member"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."server_activity" (
    "id" uuid DEFAULT gen_random_uuid (),
    "server_id" uuid,
    "dow" int4 NOT NULL,
    "time_from" time(0) NOT NULL,
    "time_to" time(0) NOT NULL,
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
    "server_id" uuid,
    "country_code" varchar(4) DEFAULT NULL::character varying,
    PRIMARY KEY ("id"),
    FOREIGN KEY ("country_code") REFERENCES "public"."country"("code") ON DELETE CASCADE,
    FOREIGN KEY ("server_id") REFERENCES "public"."server"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."logs_project" (
    "id" uuid DEFAULT gen_random_uuid (),
    "project_id" uuid,
    "date" timestamp(0) NOT NULL,
    "entity_id" varchar(255) NOT NULL,
    "entity_name" varchar(255) NOT NULL,
    "editor_name" varchar(255) DEFAULT NULL::character varying,
    "editor_role" bpchar(32) DEFAULT NULL::bpchar,
    "user_agent" varchar(255) NOT NULL DEFAULT ''::character varying,
    "ip" int8,
    "event" varchar(32) NOT NULL,
    "data" json NOT NULL,
    PRIMARY KEY ("id"),
    FOREIGN KEY ("project_id") REFERENCES "public"."project"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."logs_profile" (
    "id" uuid DEFAULT gen_random_uuid (),
    "profile_id" uuid,
    "date" timestamp(0) NOT NULL,
    "entity_id" varchar(255) NOT NULL,
    "entity_name" varchar(255) NOT NULL,
    "editor_name" varchar(255) DEFAULT NULL::character varying,
    "editor_role" bpchar(32) DEFAULT NULL::bpchar,
    "user_agent" varchar(255) NOT NULL DEFAULT ''::character varying,
    "ip" int8,
    "event" varchar(32) NOT NULL,
    "data" json NOT NULL,
    PRIMARY KEY ("id"),
    FOREIGN KEY ("profile_id") REFERENCES "public"."user"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."server_security_ip" (
    "id" uuid DEFAULT gen_random_uuid (),
    "server_id" uuid,
    "start_ip" inet,
    "end_ip" inet,
    PRIMARY KEY ("id"),
    FOREIGN KEY ("server_id") REFERENCES "public"."server"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."session" (
    "id" uuid DEFAULT gen_random_uuid (),
    "member_id" uuid,
    "status" varchar(255) NOT NULL CHECK ((status)::text = ANY ((ARRAY['unknown'::character varying, 'opened'::character varying, 'closed'::character varying])::text[])),
    "created" timestamp(0) NOT NULL,
    "message" varchar(1024) NOT NULL,
    PRIMARY KEY ("id"),
    FOREIGN KEY ("member_id") REFERENCES "public"."server_member"("id") ON DELETE CASCADE
);
COMMENT ON COLUMN "public"."session"."status" IS '(DC2Type:SessionStatusType)';

CREATE TABLE "public"."user_confirmation_token" (
    "id" uuid DEFAULT gen_random_uuid (),
    "user_id" uuid,
    "token" varchar(255) NOT NULL,
    "type" varchar(255) NOT NULL,
    "date_create" timestamp(0) NOT NULL,
    "date_confirm" timestamp(0) DEFAULT NULL::timestamp without time zone,
    PRIMARY KEY ("id"),
    FOREIGN KEY ("user_id") REFERENCES "public"."user"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."user_public_key" (
    "id" uuid DEFAULT gen_random_uuid (),
    "user_id" uuid,
    "title" varchar(255) NOT NULL,
    "key_" text NOT NULL,
    "fingerprint" varchar(255) NOT NULL,
    "last_used" timestamp(0) DEFAULT NULL::timestamp without time zone,
    "created" timestamp(0) DEFAULT NULL::timestamp without time zone,
    PRIMARY KEY ("id"),
    FOREIGN KEY ("user_id") REFERENCES "public"."user"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."user_token" (
    "token" varchar(255) NOT NULL,
    "user_id" uuid NOT NULL,
    "date_create" timestamp(0) NOT NULL,
    "date_used" timestamp(0),
    "action" varchar(255) NOT NULL CHECK ((action)::text = ANY ((ARRAY['reset'::character varying, 'delete'::character varying])::text[])),
    "used" bool NOT NULL DEFAULT false,
    PRIMARY KEY ("token","user_id"),
    FOREIGN KEY ("user_id") REFERENCES "public"."user"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."audit" (
    "id" uuid DEFAULT gen_random_uuid (),
    "account_id" uuid,
    "time_start" timestamp(0) NOT NULL,
    "time_end" timestamp(0) DEFAULT NULL::timestamp without time zone,
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
    PRIMARY KEY ("id"),
    FOREIGN KEY ("account_id") REFERENCES "public"."server_member"("id") ON DELETE CASCADE
);

CREATE TABLE "public"."audit_record" (
    "id" uuid DEFAULT gen_random_uuid (),
    "audit_id" uuid,
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
  "server_id" uuid,
  "host_key" bytea,
  FOREIGN KEY ("server_id") REFERENCES "public"."server" ("id") ON DELETE CASCADE ON UPDATE NO ACTION
);

CREATE TABLE "public"."project_api" (
    "id" uuid DEFAULT gen_random_uuid (),
    "project_id" uuid NOT NULL,
    "api_key" varchar(37) COLLATE "pg_catalog"."default" NOT NULL,
    "api_secret" varchar(37) COLLATE "pg_catalog"."default" NOT NULL,
    "online" bool NOT NULL,
    "created" timestamp(0) NOT NULL,
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
DROP TABLE IF EXISTS "public"."user_confirmation_token";
DROP TABLE IF EXISTS "public"."session";
DROP TABLE IF EXISTS "public"."server_security_ip";
DROP TABLE IF EXISTS "public"."logs_profile";
DROP TABLE IF EXISTS "public"."logs_project";
DROP TABLE IF EXISTS "public"."server_security_country";
DROP TABLE IF EXISTS "public"."country";
DROP TABLE IF EXISTS "public"."server_activity";
DROP TABLE IF EXISTS "public"."server_access_policy";
DROP TABLE IF EXISTS "public"."server_access_token";
DROP TABLE IF EXISTS "public"."server_member";
DROP TABLE IF EXISTS "public"."server";
DROP TABLE IF EXISTS "public"."project_member";
DROP TABLE IF EXISTS "public"."project_ldap";
DROP TABLE IF EXISTS "public"."project_invite";
DROP TABLE IF EXISTS "public"."project";
DROP TABLE IF EXISTS "public"."user";
-- +goose StatementEnd
