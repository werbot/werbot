-- +goose Up
-- +goose StatementBegin
CREATE TABLE "public"."firewall_list" (
  "id" uuid DEFAULT gen_random_uuid (),
  "name" varchar(255) COLLATE "pg_catalog"."default" NOT NULL,
  "active" bool NOT NULL,
  "path" varchar(255) COLLATE "pg_catalog"."default" NOT NULL,
  PRIMARY KEY ("id")
);

INSERT INTO "public"."firewall_list" VALUES ('afe7547c-f7af-41f0-bf80-dd72b807834f', 'werbot_ban.netset', 'f', 'local');
INSERT INTO "public"."firewall_list" VALUES ('6bae7414-bbcc-4824-8103-d7bba098ab78', 'firehol_webclient.netset', 'f', 'firehol');
INSERT INTO "public"."firewall_list" VALUES ('0820b52a-ec97-44e4-bc7d-8e05af1be6d1', 'firehol_proxies.netset', 'f', 'firehol');
INSERT INTO "public"."firewall_list" VALUES ('1916c066-04a3-439b-b4e6-2780605db6ff', 'firehol_anonymous.netset', 'f', 'firehol');
INSERT INTO "public"."firewall_list" VALUES ('282a74e4-ca06-4d72-877b-a56bf841cb57', 'firehol_level1.netset', 'f', 'firehol');
INSERT INTO "public"."firewall_list" VALUES ('aa134768-2324-46f9-b7b7-4fc3ebf44541', 'firehol_level2.netset', 'f', 'firehol');
INSERT INTO "public"."firewall_list" VALUES ('a76b09c0-0842-49fa-bf0c-19fcca25d767', 'firehol_level3.netset', 'f', 'firehol');
INSERT INTO "public"."firewall_list" VALUES ('742f992f-1e35-4fa5-a47d-cade1c8e74b9', 'firehol_level4.netset', 'f', 'firehol');
INSERT INTO "public"."firewall_list" VALUES ('3f3ce4bf-b3e6-456b-bf72-c03db47c1fa9', 'firehol_abusers_1d.netset', 'f', 'firehol');
INSERT INTO "public"."firewall_list" VALUES ('26a416d7-78e5-4655-9215-dd6a70cb08f9', 'firehol_abusers_30d.netset', 'f', 'firehol');
INSERT INTO "public"."firewall_list" VALUES ('e54ce36f-09ce-43cc-8162-a69c62e6e60d', 'firehol_webserver.netset', 'f', 'firehol');

CREATE TABLE "public"."firewall_ip" (
  "firewall_list_id" uuid NOT NULL,
  "start_ip" inet NOT NULL,
  "end_ip" inet NOT NULL,
  FOREIGN KEY ("firewall_list_id") REFERENCES "public"."firewall_list" ("id") ON DELETE CASCADE ON UPDATE NO ACTION
);

INSERT INTO "public"."firewall_ip" VALUES ('afe7547c-f7af-41f0-bf80-dd72b807834f', '178.239.2.11', '178.239.2.255');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS "public"."firewall_list" CASCADE;
DROP TABLE IF EXISTS "public"."firewall_ip" CASCADE;
-- +goose StatementEnd
