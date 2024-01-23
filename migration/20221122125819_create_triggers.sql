-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE TRIGGER "tg_host_key" AFTER INSERT ON "public"."server" FOR EACH ROW EXECUTE FUNCTION wrb_add_host_key();

CREATE OR REPLACE TRIGGER "tg_updated_at" BEFORE UPDATE ON "public"."user" FOR EACH ROW EXECUTE FUNCTION wrb_updated_at();
CREATE OR REPLACE TRIGGER "tg_updated_at" BEFORE UPDATE ON "public"."project" FOR EACH ROW EXECUTE FUNCTION wrb_updated_at();
CREATE OR REPLACE TRIGGER "tg_updated_at" BEFORE UPDATE ON "public"."project_invite" FOR EACH ROW EXECUTE FUNCTION wrb_updated_at();
CREATE OR REPLACE TRIGGER "tg_updated_at" BEFORE UPDATE ON "public"."project_ldap" FOR EACH ROW EXECUTE FUNCTION wrb_updated_at();
CREATE OR REPLACE TRIGGER "tg_updated_at" BEFORE UPDATE ON "public"."project_member" FOR EACH ROW EXECUTE FUNCTION wrb_updated_at();
CREATE OR REPLACE TRIGGER "tg_updated_at" BEFORE UPDATE ON "public"."server_access" FOR EACH ROW EXECUTE FUNCTION wrb_updated_at();
CREATE OR REPLACE TRIGGER "tg_updated_at" BEFORE UPDATE ON "public"."server" FOR EACH ROW EXECUTE FUNCTION wrb_updated_at();
CREATE OR REPLACE TRIGGER "tg_updated_at" BEFORE UPDATE ON "public"."server_member" FOR EACH ROW EXECUTE FUNCTION wrb_updated_at();
CREATE OR REPLACE TRIGGER "tg_updated_at" BEFORE UPDATE ON "public"."server_access_policy" FOR EACH ROW EXECUTE FUNCTION wrb_updated_at();
CREATE OR REPLACE TRIGGER "tg_updated_at" BEFORE UPDATE ON "public"."server_access_token" FOR EACH ROW EXECUTE FUNCTION wrb_updated_at();
CREATE OR REPLACE TRIGGER "tg_updated_at" BEFORE UPDATE ON "public"."user_public_key" FOR EACH ROW EXECUTE FUNCTION wrb_updated_at();
CREATE OR REPLACE TRIGGER "tg_updated_at" BEFORE UPDATE ON "public"."user_token" FOR EACH ROW EXECUTE FUNCTION wrb_updated_at();
CREATE OR REPLACE TRIGGER "tg_updated_at" BEFORE UPDATE ON "public"."server_host_key" FOR EACH ROW EXECUTE FUNCTION wrb_updated_at();
CREATE OR REPLACE TRIGGER "tg_updated_at" BEFORE UPDATE ON "public"."project_api" FOR EACH ROW EXECUTE FUNCTION wrb_updated_at();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER "tg_updated_at" ON "project_api";
DROP TRIGGER "tg_updated_at" ON "server_host_key";
DROP TRIGGER "tg_updated_at" ON "user_token";
DROP TRIGGER "tg_updated_at" ON "user_public_key";
DROP TRIGGER "tg_updated_at" ON "server_access_token";
DROP TRIGGER "tg_updated_at" ON "server_access_policy";
DROP TRIGGER "tg_updated_at" ON "server_member";
DROP TRIGGER "tg_updated_at" ON "server";
DROP TRIGGER "tg_updated_at" ON "server_access";
DROP TRIGGER "tg_updated_at" ON "project_member";
DROP TRIGGER "tg_updated_at" ON "project_ldap";
DROP TRIGGER "tg_updated_at" ON "project_invite";
DROP TRIGGER "tg_updated_at" ON "project";
DROP TRIGGER "tg_updated_at" ON "user";

DROP TRIGGER "tg_host_key" ON "server";
-- +goose StatementEnd
