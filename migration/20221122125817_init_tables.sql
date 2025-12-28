-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE "profile" (
    "id" uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    "alias" varchar(32) NOT NULL,
    "name" varchar(32) NOT NULL,
    "surname" varchar(32) NOT NULL,
    "email" varchar(64) NOT NULL,
    "password" varchar(64) NOT NULL,
    "active" bool NOT NULL DEFAULT true,
    "confirmed" bool NOT NULL DEFAULT false,
    "role" int4 NOT NULL DEFAULT 1,
    "locked_at" timestamp DEFAULT NULL,
    "archived_at" timestamp DEFAULT NULL,
    "updated_at" timestamp DEFAULT NULL,
    "created_at" timestamp DEFAULT NOW()
);
CREATE INDEX idx_profile_id ON "profile" ("id");
CREATE INDEX idx_profile_alias ON "profile" ("alias");
CREATE INDEX idx_profile_role ON "profile" ("role");
CREATE INDEX idx_profile_active ON "profile" ("active");
CREATE INDEX idx_profile_confirmed ON "profile" ("confirmed");

CREATE TABLE "project" (
    "id" uuid DEFAULT gen_random_uuid () PRIMARY KEY,
    "owner_id" uuid NOT NULL REFERENCES "profile"("id") ON DELETE CASCADE,
    "title" varchar(128) NOT NULL,
    "alias" varchar(32) NOT NULL,
    "locked_at" timestamp DEFAULT NULL,
    "archived_at" timestamp DEFAULT NULL,
    "updated_at" timestamp DEFAULT NULL,
    "created_at" timestamp DEFAULT NOW()
);
CREATE INDEX idx_project_id ON "project" ("id");
CREATE INDEX idx_project_owner_id ON "project" ("owner_id");
CREATE INDEX idx_project_alias ON "project" ("alias");

CREATE TABLE "project_ldap" (
    "id" uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    "project_id" uuid NOT NULL REFERENCES "project"("id") ON DELETE CASCADE,
    "host" varchar(255) DEFAULT NULL,
    "password" varchar(255) DEFAULT NULL,
    "port" int4,
    "root_dn" varchar(255) DEFAULT NULL,
    "query_dn" varchar(255) DEFAULT NULL,
    "object_class" varchar(255) DEFAULT NULL,
    "description" varchar(255) DEFAULT NULL,
    "updated_at" timestamp DEFAULT NULL,
    "created_at" timestamp DEFAULT NOW()
);
CREATE INDEX idx_project_ldap_id ON "project_ldap" ("id");
CREATE INDEX idx_project_ldap_project_id ON "project_ldap" ("project_id");

CREATE TABLE "project_member" (
    "id" uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    "project_id" uuid NOT NULL REFERENCES "project"("id") ON DELETE CASCADE,
    "profile_id" uuid NOT NULL REFERENCES "profile"("id") ON DELETE CASCADE,
    "active" bool NOT NULL DEFAULT false,
    "online" bool NOT NULL DEFAULT false,
    "role" smallint NOT NULL,
    "locked_at" timestamp DEFAULT NULL,
    "archived_at" timestamp DEFAULT NULL,
    "updated_at" timestamp DEFAULT NULL,
    "created_at" timestamp DEFAULT NOW()
);
CREATE INDEX idx_project_member_id ON "project_member" ("id");
CREATE INDEX idx_project_member_project_id ON "project_member" ("project_id");
CREATE INDEX idx_project_member_profile_id ON "project_member" ("profile_id");
CREATE INDEX idx_project_member_active ON "project_member" ("active");
CREATE INDEX idx_project_member_online ON "project_member" ("online");

CREATE TABLE "scheme" (
    "id" uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    "project_id" uuid NOT NULL REFERENCES "project"("id") ON DELETE CASCADE,
    "title" varchar(128) NOT NULL,
    "description" varchar(255) DEFAULT NULL,
    "active" bool NOT NULL DEFAULT false,
    "audit" bool NOT NULL DEFAULT false,
    "online" bool NOT NULL DEFAULT false,
    "scheme_type" smallint NOT NULL,
    "access" jsonb NOT NULL DEFAULT '{}'::jsonb,
    "access_policy" jsonb NOT NULL DEFAULT '{"country":0,"network":0}'::jsonb,
    "locked_at" timestamp DEFAULT NULL,
    "archived_at" timestamp DEFAULT NULL,
    "updated_at" timestamp DEFAULT NULL,
    "created_at" timestamp DEFAULT NOW()
);
CREATE INDEX idx_scheme_id ON "scheme" ("id");
CREATE INDEX idx_scheme_project_id ON "scheme" ("project_id");
CREATE INDEX idx_scheme_active ON "scheme" ("active");

CREATE TABLE "scheme_member" (
    "id" uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    "scheme_id" uuid NOT NULL REFERENCES "scheme"("id") ON DELETE CASCADE,
    "project_member_id" uuid NOT NULL REFERENCES "project_member"("id") ON DELETE CASCADE,
    "active" bool NOT NULL DEFAULT false,
    "online" bool NOT NULL DEFAULT false,
    "locked_at" timestamp DEFAULT NULL,
    "archived_at" timestamp DEFAULT NULL,
    "updated_at" timestamp DEFAULT NULL,
    "created_at" timestamp DEFAULT NOW()
);
CREATE INDEX idx_scheme_member_id ON "scheme_member" ("id");
CREATE INDEX idx_scheme_member_scheme_id ON "scheme_member" ("scheme_id");
CREATE INDEX idx_scheme_member_project_member_id ON "scheme_member" ("project_member_id");
CREATE INDEX idx_scheme_member_active ON "scheme_member" ("active");

CREATE TABLE "scheme_activity" (
    "id" uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    "scheme_id" uuid NOT NULL REFERENCES "scheme"("id") ON DELETE CASCADE,
    "data" jsonb NOT NULL DEFAULT '{}'::jsonb,
    "updated_at" timestamp DEFAULT NULL
);
CREATE INDEX idx_scheme_activity_id ON "scheme_activity" ("id");
CREATE INDEX idx_scheme_activity_scheme_id ON "scheme_activity" ("scheme_id");

CREATE TABLE "country" (
    "code" varchar(4) NOT NULL PRIMARY KEY,
    "name" varchar(255) NOT NULL
);
CREATE INDEX idx_scheme_country_code ON "country" ("code");

CREATE TABLE "scheme_firewall_country" (
    "id" uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    "scheme_id" uuid NOT NULL REFERENCES "scheme"("id") ON DELETE CASCADE,
    "country_code" varchar(2) DEFAULT NULL REFERENCES "country"("code") ON DELETE CASCADE,
    "created_at" timestamp DEFAULT NOW()
);
CREATE INDEX idx_scheme_firewall_country_id ON "scheme_firewall_country" ("id");
CREATE INDEX idx_scheme_firewall_country_scheme_id ON "scheme_firewall_country" ("scheme_id");

CREATE TABLE "scheme_firewall_network" (
    "id" uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    "scheme_id" uuid NOT NULL REFERENCES "scheme"("id") ON DELETE CASCADE,
    "network" cidr NOT NULL,
    "created_at" timestamp DEFAULT NOW()
);
CREATE INDEX idx_scheme_firewall_network_id ON "scheme_firewall_network" ("id");
CREATE INDEX idx_scheme_firewall_network_scheme_id ON "scheme_firewall_network" ("scheme_id");
CREATE INDEX idx_scheme_firewall_network_network ON "scheme_firewall_network" ("network");

CREATE TABLE "event" (
    "id" uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    "related_id" uuid NOT NULL,
    "prime_section" smallint NOT NULL,
    "section" smallint NOT NULL,
    "type" smallint NOT NULL,
    "session" jsonb NOT NULL DEFAULT '{}'::jsonb,
    "data" jsonb NOT NULL DEFAULT '{}'::jsonb,
    "created_at" timestamp DEFAULT NOW()
);
CREATE INDEX idx_event_id ON "event" ("id");
CREATE INDEX idx_event_related_id ON "event" ("related_id");
CREATE INDEX idx_event_prime_section ON "event" ("prime_section");
CREATE INDEX idx_event_section ON "event" ("section");
CREATE INDEX idx_event_event ON "event" ("type");

CREATE TABLE "session" (
    "id" uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    "member_id" uuid NOT NULL REFERENCES "scheme_member"("id") ON DELETE CASCADE,
    "status" bool NOT NULL DEFAULT false,
    "message" varchar(1024) NOT NULL,
    "created_at" timestamp DEFAULT NOW()
);
CREATE INDEX idx_session_id ON "session" ("id");
CREATE INDEX idx_session_member_id ON "session" ("member_id");
CREATE INDEX idx_session_status ON "session" ("status");

CREATE TABLE "profile_public_key" (
    "id" uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    "profile_id" uuid NOT NULL REFERENCES "profile"("id") ON DELETE CASCADE,
    "title" varchar(255) NOT NULL,
    "key" text NOT NULL,
    "fingerprint" varchar(255) NOT NULL,
    "locked_at" timestamp DEFAULT NULL,
    "archived_at" timestamp DEFAULT NULL,
    "updated_at" timestamp DEFAULT NULL,
    "created_at" timestamp DEFAULT NOW()
);
CREATE INDEX idx_profile_public_key_id ON "profile_public_key" ("id");
CREATE INDEX idx_profile_public_key_profile_id ON "profile_public_key" ("profile_id");
CREATE INDEX idx_profile_public_key_fingerprint ON "profile_public_key" ("fingerprint");

CREATE TABLE "audit" (
    "id" uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    "member_id" uuid NOT NULL REFERENCES "scheme_member"("id") ON DELETE CASCADE,
    "time_start" timestamp NOT NULL,
    "time_end" timestamp DEFAULT NULL,
    "version" smallint NOT NULL,
    "width" smallint NOT NULL,
    "height" smallint NOT NULL,
    "duration" float8 NOT NULL,
    "command" varchar(255) NOT NULL,
    "title" varchar(255) NOT NULL,
    "env_term" varchar(255) NOT NULL,
    "env_shell" varchar(255) NOT NULL,
    "session" varchar(255) NOT NULL,
    "client_ip" inet NOT NULL,
    "created_at" timestamp DEFAULT NOW()
);
CREATE INDEX idx_audit_id ON "audit" ("id");
CREATE INDEX idx_audit_member_id ON "audit" ("member_id");
CREATE INDEX idx_audit_time_start ON "audit" ("time_start");
CREATE INDEX idx_audit_client_ip ON "audit" ("client_ip");

CREATE TABLE "audit_record" (
    "id" uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    "audit_id" uuid NOT NULL REFERENCES "audit"("id") ON DELETE CASCADE,
    "duration" float8 NOT NULL,
    "screen" text NOT NULL,
    "type" smallint NOT NULL
);
CREATE INDEX idx_audit_record_id ON "audit_record" ("id");
CREATE INDEX idx_audit_record_audit_id ON "audit_record" ("audit_id");
CREATE INDEX idx_audit_record_type ON "audit_record" ("type");

CREATE TABLE "scheme_host_key" (
    "scheme_id" uuid NOT NULL REFERENCES "scheme"("id") ON DELETE CASCADE,
    "host_key" bytea,
    "updated_at" timestamp DEFAULT NULL
);
CREATE INDEX idx_scheme_host_key_scheme_id ON "scheme_host_key" ("scheme_id");

CREATE TABLE "project_api" (
    "id" uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    "project_id" uuid NOT NULL REFERENCES "project"("id") ON DELETE CASCADE,
    "api_key" varchar(37) NOT NULL,
    "api_secret" varchar(37) NOT NULL,
    "active" bool NOT NULL,
    "locked_at" timestamp DEFAULT NULL,
    "archived_at" timestamp DEFAULT NULL,
    "updated_at" timestamp DEFAULT NULL,
    "created_at" timestamp DEFAULT NOW()
);
CREATE INDEX idx_project_api_id ON "project_api" ("id");
CREATE INDEX idx_project_api_project_id ON "project_api" ("project_id");
CREATE INDEX idx_project_api_api_key ON "project_api" ("api_key");
CREATE INDEX idx_project_api_api_secret ON "project_api" ("api_secret");

CREATE TABLE "token" (
    "id" uuid DEFAULT gen_random_uuid() PRIMARY KEY,
    "section" smallint NOT NULL,
    "action" smallint NOT NULL,
    "status" smallint NOT NULL,
    "profile_id" uuid DEFAULT NULL REFERENCES "profile"("id") ON DELETE CASCADE,
    "project_id" uuid DEFAULT NULL REFERENCES "project"("id") ON DELETE CASCADE,
    "scheme_id" uuid DEFAULT NULL REFERENCES "scheme"("id") ON DELETE CASCADE,
    "data" jsonb NOT NULL DEFAULT '{}'::jsonb,
    "expired_at" timestamp DEFAULT NULL,
    "updated_at" timestamp DEFAULT NULL,
    "created_at" timestamp DEFAULT NOW()
);
CREATE INDEX idx_token_id ON "token" ("id");
CREATE INDEX idx_token_section ON "token" ("section");
CREATE INDEX idx_token_action ON "token" ("action");
CREATE INDEX idx_token_status ON "token" ("status");
CREATE INDEX idx_token_profile_id ON "token" ("profile_id");
CREATE INDEX idx_token_project_id ON "token" ("project_id");
CREATE INDEX idx_token_scheme_id ON "token" ("scheme_id");
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS "token";
DROP TABLE IF EXISTS "project_api";
DROP TABLE IF EXISTS "scheme_host_key";
DROP TABLE IF EXISTS "audit_record";
DROP TABLE IF EXISTS "audit";
DROP TABLE IF EXISTS "profile_public_key";
DROP TABLE IF EXISTS "session";
DROP TABLE IF EXISTS "scheme_firewall_network";
DROP TABLE IF EXISTS "event";
DROP TABLE IF EXISTS "scheme_firewall_country";
DROP TABLE IF EXISTS "country";
DROP TABLE IF EXISTS "scheme_activity";
DROP TABLE IF EXISTS "scheme_member";
DROP TABLE IF EXISTS "scheme";
DROP TABLE IF EXISTS "project_member";
DROP TABLE IF EXISTS "project_ldap";
DROP TABLE IF EXISTS "project";
DROP TABLE IF EXISTS "profile";

DROP EXTENSION IF EXISTS "pgcrypto";
-- +goose StatementEnd
