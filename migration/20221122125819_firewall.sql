-- +goose Up
-- +goose StatementBegin
CREATE TABLE "firewall_list" (
  "id" uuid DEFAULT gen_random_uuid () PRIMARY KEY,
  "name" varchar(255) NOT NULL,
  "active" bool NOT NULL,
  "path" smallint NOT NULL,
  "updated_at" timestamp,
  "created_at" timestamp DEFAULT now()
);
CREATE INDEX idx_firewall_list_id ON "firewall_list" ("id");

INSERT INTO "firewall_list" VALUES ('afe7547c-f7af-41f0-bf80-dd72b807834f', 'werbot_ban.netset', true, 0);
INSERT INTO "firewall_list" VALUES ('282a74e4-ca06-4d72-877b-a56bf841cb57', 'firehol_level1.netset', true, 1);
INSERT INTO "firewall_list" VALUES ('a30bca4e-639e-45de-b1d2-d2039c1b6acc', 'urandomusto_rdp.ipset', false, 1);
INSERT INTO "firewall_list" VALUES ('5bf3e65c-3d32-45f2-8902-5740bc855693', 'urandomusto_smb.ipset', false, 1);
INSERT INTO "firewall_list" VALUES ('54789e1b-9f53-4ff9-962f-1dd7cfcd2044', 'urandomusto_spam.ipset', false, 1);
INSERT INTO "firewall_list" VALUES ('5f7f382a-2da0-4b0b-bf84-e0aef2d591f2', 'urandomusto_ssh.ipset', false, 1);
INSERT INTO "firewall_list" VALUES ('af0c4796-d75d-4e57-99e7-81522045ac13', 'urandomusto_telnet.ipset', false, 1);
INSERT INTO "firewall_list" VALUES ('3d4754b6-b041-43db-b063-fbca80a783ef', 'urandomusto_vnc.ipset', false, 1);
INSERT INTO "firewall_list" VALUES ('38e21a7c-65e6-4626-a714-a54fdcfb1812', 'blocklist_de_ssh.ipset', false, 1);
INSERT INTO "firewall_list" VALUES ('e94ee2b9-c784-4976-a40a-acb1316cce98', 'blocklist_de_ftp.ipset', false, 1);
INSERT INTO "firewall_list" VALUES ('fde4ce22-cb77-4e8e-9503-317cc99eb2df', 'blocklist_de_bruteforce.ipset', false, 1);
INSERT INTO "firewall_list" VALUES ('6edc1573-adfe-4b9c-86a0-bb9da528bdfe', 'bruteforceblocker.ipset', false, 1);
INSERT INTO "firewall_list" VALUES ('bac91a2f-6364-4ac0-95be-556015002a2a', 'darklist_de.netset', false, 1);

CREATE TABLE "firewall_network" (
  "firewall_list_id" uuid NOT NULL REFERENCES "firewall_list"("id") ON DELETE CASCADE,
  "network" cidr NOT NULL
);
CREATE INDEX idx_firewall_network_firewall_list_id ON "firewall_network" ("firewall_list_id");
CREATE INDEX idx_firewall_network_network ON "firewall_network" ("network");

CREATE TABLE "firewall_country" (
  "firewall_list_id" uuid NOT NULL REFERENCES "firewall_list"("id") ON DELETE CASCADE,
  "country_code" varchar(2) NOT NULL REFERENCES "country"("code") ON DELETE CASCADE
);
CREATE INDEX idx_firewall_country_firewall_list_id ON "firewall_country" ("firewall_list_id");
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS "firewall_country" CASCADE;
DROP TABLE IF EXISTS "firewall_list" CASCADE;
DROP TABLE IF EXISTS "firewall_network" CASCADE;
-- +goose StatementEnd
